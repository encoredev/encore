use std::collections::HashSet;
use std::fmt;
use std::path::PathBuf;

use indexmap::IndexMap;
use serde::de::{self, Deserialize, Deserializer, MapAccess, Visitor};

/// The parsed representation of the "exports" field in a package.json file.
/// See https://nodejs.org/api/packages.html#package-entry-points for syntax.
#[derive(Debug)]
pub struct Exports {
    subpaths: IndexMap<String, Subpath>,
}

impl Exports {
    /// Resolves a relative path to candidate target file paths, in priority order.
    /// The caller should use the first candidate that exists.
    ///
    /// As in Node and TypeScript, an exact key wins, and otherwise the most specific
    /// matching pattern does. That entry alone decides the result: if it gives no
    /// target, the path isn't exported and less specific patterns aren't tried.
    pub fn resolve_import_path(&self, rel_path: &str, conditions: &HashSet<&str>) -> Vec<PathBuf> {
        let mut best: Option<(&str, &Subpath, &str)> = None;
        for (key, subpath) in self.subpaths.iter() {
            match candidate_matches(key, rel_path) {
                None => continue,
                Some(Match::Exact) => {
                    return match subpath.matches(conditions) {
                        Some(SubpathMatch::Targets(paths)) => {
                            paths.into_iter().map(PathBuf::from).collect()
                        }
                        Some(SubpathMatch::Exclude) | None => vec![],
                    };
                }
                Some(Match::Wildcard { replacement }) => {
                    let more_specific = match best {
                        None => true,
                        Some((best_key, ..)) => {
                            pattern_specificity(key) > pattern_specificity(best_key)
                        }
                    };
                    if more_specific {
                        best = Some((key, subpath, replacement));
                    }
                }
            }
        }

        let Some((_, subpath, replacement)) = best else {
            return vec![];
        };
        match subpath.matches(conditions) {
            Some(SubpathMatch::Targets(paths)) => paths
                .into_iter()
                .map(|path| path.replace('*', replacement).into())
                .collect(),
            Some(SubpathMatch::Exclude) | None => vec![],
        }
    }
}

/// Orders pattern keys like Node's PATTERN_KEY_COMPARE: a longer prefix before the
/// "*" is more specific, then a longer key.
fn pattern_specificity(key: &str) -> (usize, usize) {
    (key.find('*').unwrap_or(key.len()), key.len())
}

enum Match<'a> {
    Exact,
    Wildcard { replacement: &'a str },
}

fn candidate_matches<'a>(candidate: &str, rel_path: &'a str) -> Option<Match<'a>> {
    let candidate = match (candidate, rel_path) {
        (".", "") => return Some(Match::Exact),
        (".", _) => return None,
        (_, _) => {
            // Strip "./" prefix.
            let candidate = candidate.strip_prefix("./").unwrap_or(candidate);
            if candidate == rel_path {
                return Some(Match::Exact);
            }
            candidate
        }
    };

    if let Some(idx) = candidate.find('*') {
        let (prefix, suffix) = candidate.split_at(idx);
        // Strip the "*" from the suffix.
        let suffix = &suffix[1..];
        // The length check keeps the prefix and suffix from overlapping.
        if rel_path.len() >= prefix.len() + suffix.len()
            && rel_path.starts_with(prefix)
            && rel_path.ends_with(suffix)
        {
            // Get the middle part of the path, to be injected into the result.
            let replacement = &rel_path[prefix.len()..(rel_path.len() - suffix.len())];
            return Some(Match::Wildcard { replacement });
        }
    }

    None
}

#[derive(Debug, PartialEq, Eq)]
enum Subpath {
    Target(String),
    Conditions(IndexMap<String, Subpath>),
    Exclude,
}

enum SubpathMatch<'a> {
    /// Candidate targets, in priority order. The first one that exists wins.
    Targets(Vec<&'a str>),
    Exclude,
}

impl Subpath {
    /// Returns the targets this subpath maps to under the active conditions.
    ///
    /// Like TypeScript, a matching condition doesn't end the search: the targets of
    /// later matching conditions are kept as fallbacks, in object order. So when a
    /// "types" target doesn't exist, the "import" target is tried next.
    /// A null under a condition gives no target, at any nesting level; only a
    /// subpath that maps directly to null is excluded.
    fn matches(&self, active_conditions: &HashSet<&str>) -> Option<SubpathMatch<'_>> {
        match self {
            Subpath::Target(path) => Some(SubpathMatch::Targets(vec![path])),
            Subpath::Exclude => Some(SubpathMatch::Exclude),
            Subpath::Conditions(conds) => {
                let mut targets = Vec::new();
                for (cond, subpath) in conds.iter() {
                    if !active_conditions.contains(cond.as_str()) {
                        continue;
                    }
                    // A null target, or nested conditions with no match, gives
                    // nothing: try the next condition.
                    if let Some(SubpathMatch::Targets(paths)) = subpath.matches(active_conditions) {
                        targets.extend(paths);
                    }
                }
                if targets.is_empty() {
                    None
                } else {
                    Some(SubpathMatch::Targets(targets))
                }
            }
        }
    }
}

// The "exports" value can be defined in lots of ways.
//
// - A single subpath, e.g. "exports": "./index.js".
//   Syntactic sugar for {".": "./index.js"}
//
// - A condition spec: {"node": "./index.node.js"}
//   Conditions can be nested: {"node": {"import": "./index.node.js", "default": "./index.js"}}
//
// - An ordered map of subpaths key-value pairs: {".": "./index.js", "./foo/*": "./foo/*.js"}
// - Each subpath value can be one of:
//   - A single target path, e.g. "./index.js" (possibly with wildcards, "./foo/*.js")
//   - A condition spec.
//   - Null, indicating the subpath is not exported (overriding other exports that may match).

impl<'de> Deserialize<'de> for Exports {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        struct StringOrMap;
        impl<'de> Visitor<'de> for StringOrMap {
            type Value = Exports;

            fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
                formatter.write_str("string or map")
            }

            fn visit_str<E>(self, value: &str) -> Result<Self::Value, E>
            where
                E: de::Error,
            {
                let mut subpaths = IndexMap::<String, Subpath>::new();
                subpaths.insert(".".into(), Subpath::Target(value.to_string()));
                Ok(Exports { subpaths })
            }

            fn visit_map<M>(self, mut access: M) -> Result<Self::Value, M::Error>
            where
                M: MapAccess<'de>,
            {
                let mut subpaths = IndexMap::new();

                // Peek at the first entry to decide whether it's a map of subpaths or conditions.
                let Some((key, value)) = access.next_entry::<String, Subpath>()? else {
                    // Empty map.
                    return Ok(Exports { subpaths });
                };

                if !key.starts_with('.') {
                    let mut conditions: IndexMap<String, Subpath> = IndexMap::new();
                    conditions.insert(key, value);
                    let rest: IndexMap<String, Subpath> =
                        Deserialize::deserialize(de::value::MapAccessDeserializer::new(access))?;
                    conditions.extend(rest);
                    subpaths.insert(".".to_string(), Subpath::Conditions(conditions));
                    return Ok(Exports { subpaths });
                }

                subpaths.insert(key, value);
                while let Some((key, value)) = access.next_entry::<String, Subpath>()? {
                    subpaths.insert(key, value);
                }

                Ok(Exports { subpaths })
            }
        }

        deserializer.deserialize_any(StringOrMap)
    }
}

struct SubpathVisitor;

impl<'de> Visitor<'de> for SubpathVisitor {
    type Value = Subpath;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("string or map")
    }

    fn visit_unit<E>(self) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        Ok(Subpath::Exclude)
    }

    fn visit_str<E>(self, value: &str) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        Ok(Subpath::Target(value.to_string()))
    }

    fn visit_map<M>(self, map: M) -> Result<Self::Value, M::Error>
    where
        M: MapAccess<'de>,
    {
        let conditions: IndexMap<String, Subpath> =
            Deserialize::deserialize(de::value::MapAccessDeserializer::new(map))?;
        Ok(Subpath::Conditions(conditions))
    }
}

impl<'de> Deserialize<'de> for Subpath {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_any(SubpathVisitor)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_subpaths() {
        let exports: Exports = serde_json::from_str(
            r#"{
            ".": "./index.js",
            "./foo/*": "./foo/*.js",
            "./bar": {"node": "./bar.node.js", "default": "./bar.js"},
            "./baz": {"node": "./baz.node.js", "default": null},
            "./qux": null
        }"#,
        )
        .unwrap();
        assert_eq!(exports.subpaths.len(), 5);
        assert_eq!(
            exports.subpaths.get(".").unwrap(),
            &Subpath::Target("./index.js".into())
        );
        assert_eq!(
            exports.subpaths.get("./foo/*").unwrap(),
            &Subpath::Target("./foo/*.js".into())
        );
        assert_eq!(
            exports.subpaths.get("./bar").unwrap(),
            &Subpath::Conditions({
                let mut map = IndexMap::new();
                map.insert("node".to_owned(), Subpath::Target("./bar.node.js".into()));
                map.insert("default".to_owned(), Subpath::Target("./bar.js".into()));
                map
            })
        );
        assert_eq!(
            exports.subpaths.get("./baz").unwrap(),
            &Subpath::Conditions({
                let mut map = IndexMap::new();
                map.insert("node".to_owned(), Subpath::Target("./baz.node.js".into()));
                map.insert("default".to_owned(), Subpath::Exclude);
                map
            })
        );
        assert_eq!(exports.subpaths.get("./qux").unwrap(), &Subpath::Exclude);
    }

    #[test]
    fn parse_toplevel_conditions() {
        let exports: Exports = serde_json::from_str(
            r#"{
            "node": {"import": "./bar.node.js", "default": "./bar.js"},
            "default": "./index.js"
        }"#,
        )
        .unwrap();
        assert_eq!(exports.subpaths.len(), 1);
        assert_eq!(
            exports.subpaths.get(".").unwrap(),
            &Subpath::Conditions({
                let mut map = IndexMap::new();
                map.insert(
                    "node".to_owned(),
                    Subpath::Conditions({
                        let mut map = IndexMap::new();
                        map.insert("import".to_owned(), Subpath::Target("./bar.node.js".into()));
                        map.insert("default".to_owned(), Subpath::Target("./bar.js".into()));
                        map
                    }),
                );
                map.insert("default".to_owned(), Subpath::Target("./index.js".into()));
                map
            })
        );
    }

    #[test]
    fn toplevel_condition_priority_order() {
        let exports: Exports = serde_json::from_str(
            r#"{
            "types": "./dist/index.d.ts",
            "import": "./dist/index.mjs",
            "default": "./dist/index.js"
        }"#,
        )
        .unwrap();

        let conditions: HashSet<&str> = ["types", "import", "default"].into_iter().collect();
        let resolved = exports.resolve_import_path("", &conditions);
        // "types" must win because it appears first in the JSON.
        assert_eq!(resolved.first(), Some(&PathBuf::from("./dist/index.d.ts")));
    }

    fn resolve(exports: &str, rel_path: &str, conditions: &[&str]) -> Vec<PathBuf> {
        let exports: Exports = serde_json::from_str(exports).unwrap();
        let conditions: HashSet<&str> = conditions.iter().copied().collect();
        exports.resolve_import_path(rel_path, &conditions)
    }

    fn paths(paths: &[&str]) -> Vec<PathBuf> {
        paths.iter().map(PathBuf::from).collect()
    }

    #[test]
    fn condition_fallback_order() {
        let exports = r#"{
            "types": "./dist/index.d.ts",
            "import": "./dist/index.mjs",
            "default": "./dist/index.js"
        }"#;

        // Object order decides priority, not the order of the active conditions.
        assert_eq!(
            resolve(exports, "", &["default", "import", "types"]),
            paths(&["./dist/index.d.ts", "./dist/index.mjs", "./dist/index.js"])
        );
        assert_eq!(
            resolve(exports, "", &["import", "default"]),
            paths(&["./dist/index.mjs", "./dist/index.js"])
        );
    }

    #[test]
    fn wildcard_types_falls_back_to_import() {
        // The "./*" entry of @modelcontextprotocol/sdk >= 1.29.0. For "server/mcp.js" the
        // "types" target is "dist/esm/server/mcp.js.d.ts", which doesn't exist, so the
        // "import" target must remain a candidate.
        let exports = r#"{
            ".": {
                "types": "./dist/esm/index.d.ts",
                "import": "./dist/esm/index.js"
            },
            "./*": {
                "types": "./dist/esm/*.d.ts",
                "import": "./dist/esm/*",
                "require": "./dist/cjs/*"
            }
        }"#;

        assert_eq!(
            resolve(exports, "server/mcp.js", &["types", "import", "default"]),
            paths(&["./dist/esm/server/mcp.js.d.ts", "./dist/esm/server/mcp.js"])
        );
    }

    #[test]
    fn wildcard_with_suffix() {
        let exports = r#"{"./lib/*.js": "./dist/*.mjs"}"#;
        assert_eq!(
            resolve(exports, "lib/a/b.js", &["default"]),
            paths(&["./dist/a/b.mjs"])
        );
        assert_eq!(resolve(exports, "lib/a/b.ts", &["default"]), paths(&[]));
    }

    #[test]
    fn nested_conditions_without_match_fall_through() {
        let exports = r#"{
            "node": {"import": "./index.node.mjs"},
            "default": "./index.js"
        }"#;

        assert_eq!(
            resolve(exports, "", &["node", "require", "default"]),
            paths(&["./index.js"])
        );
    }

    #[test]
    fn null_subpath_is_excluded() {
        let exports = r#"{
            "./*": "./dist/*.js",
            "./internal/*": null
        }"#;

        assert_eq!(resolve(exports, "internal/foo", &["default"]), paths(&[]));
        assert_eq!(
            resolve(exports, "foo", &["default"]),
            paths(&["./dist/foo.js"])
        );
        assert_eq!(
            resolve(r#"{"./foo": null}"#, "foo", &["default"]),
            paths(&[])
        );
    }

    #[test]
    fn null_condition_is_skipped() {
        // Like TypeScript, a null under a condition gives no candidate and the search
        // goes on, at every nesting level.
        assert_eq!(
            resolve(
                r#"{"./internal/*": {"node": null, "default": "./internal/*.js"}}"#,
                "internal/foo",
                &["node", "default"]
            ),
            paths(&["./internal/foo.js"])
        );
        assert_eq!(
            resolve(
                r#"{"types": "./index.d.ts", "node": null, "default": "./index.js"}"#,
                "",
                &["types", "node", "default"]
            ),
            paths(&["./index.d.ts", "./index.js"])
        );
        assert_eq!(
            resolve(
                r#"{
                    "outer": {"first": "./missing.js", "second": null},
                    "default": "./fallback.js"
                }"#,
                "",
                &["outer", "first", "second", "default"]
            ),
            paths(&["./missing.js", "./fallback.js"])
        );
        assert_eq!(
            resolve(
                r#"{"outer": {"second": null}, "default": "./fallback.js"}"#,
                "",
                &["outer", "second", "default"]
            ),
            paths(&["./fallback.js"])
        );

        // Only nulls under the active conditions: nothing to resolve.
        assert_eq!(
            resolve(r#"{"node": null}"#, "", &["node", "default"]),
            paths(&[])
        );
    }

    #[test]
    fn most_specific_pattern_wins() {
        // As in Node and TypeScript, the most specific matching pattern decides the
        // result, whatever its position in the object.
        let conditions = &["node", "default"];
        assert_eq!(
            resolve(
                r#"{"./*": "./dist/*.js", "./internal/*": "./internal/*.js"}"#,
                "internal/foo",
                conditions
            ),
            paths(&["./internal/foo.js"])
        );

        // A longer prefix before the "*" wins; with equal prefixes, the longer key.
        assert_eq!(
            resolve(
                r#"{"./a/*": "./one/*", "./a/*.js": "./two/*.js"}"#,
                "a/b.js",
                conditions
            ),
            paths(&["./two/b.js"])
        );

        // If the most specific pattern resolves to nothing, the path isn't exported:
        // less specific patterns aren't tried.
        for exports in [
            r#"{"./*": "./dist/*.js", "./internal/*": {"node": null}}"#,
            r#"{"./internal/*": {"node": null}, "./*": "./dist/*.js"}"#,
            r#"{"./*": "./dist/*.js", "./internal/*": {"browser": "./browser/*.js"}}"#,
        ] {
            assert_eq!(
                resolve(exports, "internal/foo", conditions),
                paths(&[]),
                "{exports}"
            );
        }
    }

    #[test]
    fn pattern_prefix_and_suffix_overlap() {
        assert_eq!(
            resolve(r#"{"./ab*bc": "./x/*.js"}"#, "abc", &["default"]),
            paths(&[])
        );
        assert_eq!(
            resolve(r#"{"./ab*bc": "./x/*.js"}"#, "abxbc", &["default"]),
            paths(&["./x/x.js"])
        );
    }
}
