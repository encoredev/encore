use std::collections::{HashMap, HashSet};
use std::path::{Path, PathBuf};

use swc_common::FileName;
use swc_ecma_loader::resolve::Resolve;

use crate::exports::Exports;
use crate::resolve_utils::{dts_counterpart, split_package_name};
use crate::tsconfig::TsConfigPathResolver;

/// Conditions used when resolving package.json "exports" in the browser/WASM context.
static RESOLVE_CONDITIONS: &[&str] = &["types", "import", "default"];

/// Extensions to try when resolving a module path.
static RESOLVE_EXTENSIONS: &[&str] = &[
    ".d.ts", ".ts", ".tsx", ".d.mts", ".mts", ".d.cts", ".cts", ".js", ".jsx", ".mjs", ".cjs",
];

/// Index filenames to try when resolving a directory-like import.
static INDEX_NAMES: &[&str] = &[
    "index.d.ts",
    "index.ts",
    "index.d.mts",
    "index.mts",
    "index.js",
    "index.mjs",
];

/// A module resolver that works entirely in-memory, for use in WASM/browser contexts.
/// It resolves `encore.dev/*` imports to custom filenames, relative imports
/// against a known set of file paths, and bare specifier imports via node_modules.
pub struct InMemoryResolver {
    known_files: HashSet<PathBuf>,
    app_root: PathBuf,
    package_jsons: HashMap<PathBuf, serde_json::Value>,
    tsconfig: Option<TsConfigPathResolver>,
}

impl InMemoryResolver {
    pub fn new(app_root: PathBuf, file_paths: Vec<PathBuf>) -> Self {
        let known_files = file_paths.into_iter().collect();
        Self {
            known_files,
            app_root,
            package_jsons: HashMap::new(),
            tsconfig: None,
        }
    }

    /// Set the tsconfig path resolver for path alias support.
    pub fn set_tsconfig(&mut self, tsconfig: TsConfigPathResolver) {
        self.tsconfig = Some(tsconfig);
    }

    /// Register a parsed package.json for a given path (e.g. `/app/node_modules/zod/package.json`).
    pub fn register_package_json(&mut self, path: PathBuf, value: serde_json::Value) {
        self.package_jsons.insert(path, value);
    }

    /// Try to resolve a bare specifier (e.g. "zod", "zod/lib/types") to a file path.
    fn resolve_bare_specifier(&self, target: &str) -> Result<FileName, anyhow::Error> {
        let (pkg_name, subpath) = split_package_name(target);

        // Try the package itself, then fall back to @types/{name} for non-scoped packages.
        let candidates: Vec<String> = if pkg_name.starts_with('@') {
            vec![pkg_name.to_string()]
        } else {
            vec![pkg_name.to_string(), format!("@types/{}", pkg_name)]
        };

        for candidate_pkg in &candidates {
            let pkg_json_path = self
                .app_root
                .join("node_modules")
                .join(candidate_pkg)
                .join("package.json");

            let Some(pkg_json) = self.package_jsons.get(&pkg_json_path) else {
                continue;
            };

            let pkg_dir = pkg_json_path.parent().unwrap().to_path_buf();

            // 1. Try "exports" field
            if let Some(exports_val) = pkg_json.get("exports") {
                if let Ok(exports) = serde_json::from_value::<Exports>(exports_val.clone()) {
                    let conditions: HashSet<&str> = RESOLVE_CONDITIONS.iter().copied().collect();
                    if let Some(resolved) = exports.resolve_import_path(subpath, &conditions) {
                        let full_path = pkg_dir.join(&resolved);
                        if let Some(found) = self.try_with_dts(&full_path) {
                            return Ok(FileName::Real(found));
                        }
                    }
                }
            }

            // Only use fallback fields when resolving the package root (no subpath).
            if !subpath.is_empty() {
                // For subpath imports without exports, try direct file resolution.
                let direct = pkg_dir.join(subpath);
                for candidate in file_candidates(&direct) {
                    if self.known_files.contains(&candidate) {
                        return Ok(FileName::Real(candidate));
                    }
                }
                continue;
            }

            // 2. Fallback: "types" field
            if let Some(types) = pkg_json.get("types").or_else(|| pkg_json.get("typings")) {
                if let Some(types_str) = types.as_str() {
                    let full_path = pkg_dir.join(types_str);
                    if self.known_files.contains(&full_path) {
                        return Ok(FileName::Real(full_path));
                    }
                }
            }

            // 3. Fallback: "main" field (try .d.ts counterpart)
            if let Some(main) = pkg_json.get("main") {
                if let Some(main_str) = main.as_str() {
                    let full_path = pkg_dir.join(main_str);
                    if let Some(found) = self.try_with_dts(&full_path) {
                        return Ok(FileName::Real(found));
                    }
                }
            }

            // 4. Last resort: index.d.ts, index.ts
            for name in &["index.d.ts", "index.ts"] {
                let candidate = pkg_dir.join(name);
                if self.known_files.contains(&candidate) {
                    return Ok(FileName::Real(candidate));
                }
            }
        }

        Err(anyhow::anyhow!(
            "unable to resolve bare specifier: {target}"
        ))
    }

    /// Try the .d.ts counterpart first, then the original path.
    fn try_with_dts(&self, path: &Path) -> Option<PathBuf> {
        if let Some(dts) = dts_counterpart(path) {
            if self.known_files.contains(&dts) {
                return Some(dts);
            }
        }
        if self.known_files.contains(path) {
            return Some(path.to_path_buf());
        }
        None
    }
}

/// Generate candidate file paths to try (with extensions and index variants).
fn file_candidates(base: &Path) -> Vec<PathBuf> {
    let mut candidates = Vec::with_capacity(1 + RESOLVE_EXTENSIONS.len() + INDEX_NAMES.len());
    // Exact path
    candidates.push(base.to_path_buf());
    // With extensions
    for ext in RESOLVE_EXTENSIONS {
        candidates.push(PathBuf::from(format!("{}{}", base.display(), ext)));
    }
    // Index variants
    for name in INDEX_NAMES {
        candidates.push(base.join(name));
    }
    candidates
}

impl Resolve for InMemoryResolver {
    fn resolve(&self, base: &FileName, target: &str) -> Result<FileName, anyhow::Error> {
        // Handle encore.dev/* imports -> return custom filename
        if target.starts_with("encore.dev/") || target == "encore.dev" {
            return Ok(FileName::Custom(target.to_string()));
        }

        // Try tsconfig path aliases (e.g. "@/*" -> "./src/*")
        if let Some(tsconfig) = &self.tsconfig {
            if let Some(resolved) =
                tsconfig.resolve_with_checker(target, |p| self.known_files.contains(p))
            {
                // The resolved path is relative to tsconfig's base dir.
                // Resolve it the same way we resolve relative imports.
                if let FileName::Real(tsconfig_base) = tsconfig.base() {
                    let resolved_path = normalize_path(&tsconfig_base.join(resolved.as_ref()));
                    for candidate in file_candidates(&resolved_path) {
                        if self.known_files.contains(&candidate) {
                            return Ok(FileName::Real(candidate));
                        }
                    }
                }
            }
        }

        // Handle relative imports
        if target.starts_with("./") || target.starts_with("../") {
            if let FileName::Real(base_path) = base {
                let parent = base_path.parent().unwrap_or(base_path);
                let resolved = normalize_path(&parent.join(target));

                for candidate in file_candidates(&resolved) {
                    if self.known_files.contains(&candidate) {
                        return Ok(FileName::Real(candidate));
                    }
                }
            }
        }

        // Handle bare specifier imports (e.g. "zod", "@types/react")
        if let Ok(resolved) = self.resolve_bare_specifier(target) {
            return Ok(resolved);
        }

        Err(anyhow::anyhow!("unable to resolve {target}"))
    }
}

/// Normalize a path by resolving `.` and `..` components without filesystem access.
fn normalize_path(path: &Path) -> PathBuf {
    use std::path::Component;
    let mut components = Vec::new();
    for component in path.components() {
        match component {
            Component::CurDir => {}
            Component::ParentDir => {
                components.pop();
            }
            c => components.push(c),
        }
    }
    components.iter().collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_resolver(files: &[&str], package_jsons: &[(&str, &str)]) -> InMemoryResolver {
        let app_root = PathBuf::from("/app");
        let file_paths: Vec<PathBuf> = files.iter().map(|f| app_root.join(f)).collect();
        let mut resolver = InMemoryResolver::new(app_root.clone(), file_paths);
        for (path, content) in package_jsons {
            let value: serde_json::Value = serde_json::from_str(content).unwrap();
            resolver.register_package_json(app_root.join(path), value);
        }
        resolver
    }

    #[test]
    fn resolve_encore_dev() {
        let resolver = make_resolver(&[], &[]);
        let result = resolver
            .resolve(&FileName::Real("/app/src/foo.ts".into()), "encore.dev/api")
            .unwrap();
        assert_eq!(result, FileName::Custom("encore.dev/api".into()));
    }

    #[test]
    fn resolve_encore_dev_root() {
        let resolver = make_resolver(&[], &[]);
        let result = resolver
            .resolve(&FileName::Real("/app/src/foo.ts".into()), "encore.dev")
            .unwrap();
        assert_eq!(result, FileName::Custom("encore.dev".into()));
    }

    #[test]
    fn resolve_relative_ts() {
        let resolver = make_resolver(&["src/bar.ts"], &[]);
        let result = resolver
            .resolve(&FileName::Real("/app/src/foo.ts".into()), "./bar")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/bar.ts".into()));
    }

    #[test]
    fn resolve_relative_dts() {
        let resolver = make_resolver(&["src/types.d.ts"], &[]);
        let result = resolver
            .resolve(&FileName::Real("/app/src/foo.ts".into()), "./types")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/types.d.ts".into()));
    }

    #[test]
    fn resolve_relative_index() {
        let resolver = make_resolver(&["src/utils/index.ts"], &[]);
        let result = resolver
            .resolve(&FileName::Real("/app/src/foo.ts".into()), "./utils")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/utils/index.ts".into()));
    }

    #[test]
    fn resolve_relative_index_dts() {
        let resolver = make_resolver(&["src/utils/index.d.ts"], &[]);
        let result = resolver
            .resolve(&FileName::Real("/app/src/foo.ts".into()), "./utils")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/utils/index.d.ts".into()));
    }

    #[test]
    fn resolve_relative_parent() {
        let resolver = make_resolver(&["lib/helper.ts"], &[]);
        let result = resolver
            .resolve(
                &FileName::Real("/app/src/deep/foo.ts".into()),
                "../../lib/helper",
            )
            .unwrap();
        assert_eq!(result, FileName::Real("/app/lib/helper.ts".into()));
    }

    #[test]
    fn resolve_bare_with_types_field() {
        let resolver = make_resolver(
            &["node_modules/foo/dist/index.d.ts"],
            &[(
                "node_modules/foo/package.json",
                r#"{"name": "foo", "types": "dist/index.d.ts"}"#,
            )],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "foo")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/foo/dist/index.d.ts".into())
        );
    }

    #[test]
    fn resolve_bare_with_exports() {
        let resolver = make_resolver(
            &["node_modules/zod/lib/index.d.mts"],
            &[(
                "node_modules/zod/package.json",
                r#"{"name": "zod", "exports": {".": {"types": "./lib/index.d.mts"}}}"#,
            )],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "zod")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/zod/lib/index.d.mts".into())
        );
    }

    #[test]
    fn resolve_bare_exports_js_to_dts() {
        // exports points to .js, but .d.ts counterpart exists and should be preferred
        let resolver = make_resolver(
            &[
                "node_modules/pkg/dist/index.js",
                "node_modules/pkg/dist/index.d.ts",
            ],
            &[(
                "node_modules/pkg/package.json",
                r#"{"name": "pkg", "exports": {".": "./dist/index.js"}}"#,
            )],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "pkg")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/pkg/dist/index.d.ts".into())
        );
    }

    #[test]
    fn resolve_bare_main_dts_counterpart() {
        let resolver = make_resolver(
            &["node_modules/lib/main.d.ts"],
            &[(
                "node_modules/lib/package.json",
                r#"{"name": "lib", "main": "main.js"}"#,
            )],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "lib")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/lib/main.d.ts".into())
        );
    }

    #[test]
    fn resolve_bare_index_fallback() {
        let resolver = make_resolver(
            &["node_modules/simple/index.d.ts"],
            &[("node_modules/simple/package.json", r#"{"name": "simple"}"#)],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "simple")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/simple/index.d.ts".into())
        );
    }

    #[test]
    fn resolve_bare_types_fallback() {
        // Package not found directly, but @types/foo exists
        let resolver = make_resolver(
            &["node_modules/@types/foo/index.d.ts"],
            &[(
                "node_modules/@types/foo/package.json",
                r#"{"name": "@types/foo", "types": "index.d.ts"}"#,
            )],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "foo")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/@types/foo/index.d.ts".into())
        );
    }

    #[test]
    fn resolve_scoped_package() {
        let resolver = make_resolver(
            &["node_modules/@scope/pkg/dist/index.d.ts"],
            &[(
                "node_modules/@scope/pkg/package.json",
                r#"{"name": "@scope/pkg", "types": "dist/index.d.ts"}"#,
            )],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "@scope/pkg")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/@scope/pkg/dist/index.d.ts".into())
        );
    }

    #[test]
    fn resolve_subpath_import() {
        let resolver = make_resolver(
            &["node_modules/pkg/lib/utils.d.ts"],
            &[("node_modules/pkg/package.json", r#"{"name": "pkg"}"#)],
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "pkg/lib/utils")
            .unwrap();
        assert_eq!(
            result,
            FileName::Real("/app/node_modules/pkg/lib/utils.d.ts".into())
        );
    }

    #[test]
    fn resolve_unresolvable() {
        let resolver = make_resolver(&[], &[]);
        let result = resolver.resolve(&FileName::Real("/app/src/main.ts".into()), "nonexistent");
        assert!(result.is_err());
    }

    fn make_resolver_with_tsconfig(files: &[&str], tsconfig_json: &str) -> InMemoryResolver {
        let app_root = PathBuf::from("/app");
        let file_paths: Vec<PathBuf> = files.iter().map(|f| app_root.join(f)).collect();
        let mut resolver = InMemoryResolver::new(app_root.clone(), file_paths);
        let tsconfig = TsConfigPathResolver::from_str(&app_root, tsconfig_json).unwrap();
        resolver.set_tsconfig(tsconfig);
        resolver
    }

    #[test]
    fn resolve_tsconfig_wildcard_alias() {
        let resolver = make_resolver_with_tsconfig(
            &["src/utils/helper.ts"],
            r#"{"compilerOptions": {"paths": {"@/*": ["./src/*"]}}}"#,
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "@/utils/helper")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/utils/helper.ts".into()));
    }

    #[test]
    fn resolve_tsconfig_exact_alias() {
        let resolver = make_resolver_with_tsconfig(
            &["src/config.ts"],
            r#"{"compilerOptions": {"paths": {"config": ["./src/config"]}}}"#,
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "config")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/config.ts".into()));
    }

    #[test]
    fn resolve_tsconfig_with_base_url() {
        let resolver = make_resolver_with_tsconfig(
            &["src/lib/utils.ts"],
            r#"{"compilerOptions": {"baseUrl": "./src", "paths": {"@lib/*": ["./lib/*"]}}}"#,
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "@lib/utils")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/lib/utils.ts".into()));
    }

    #[test]
    fn resolve_tsconfig_fallback_values() {
        // First path doesn't exist, second does
        let resolver = make_resolver_with_tsconfig(
            &["lib/helper.ts"],
            r#"{"compilerOptions": {"paths": {"@/*": ["./src/*", "./lib/*"]}}}"#,
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/main.ts".into()), "@/helper")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/lib/helper.ts".into()));
    }

    #[test]
    fn resolve_tsconfig_no_match_falls_through() {
        // tsconfig alias doesn't match, should fall through to relative resolution
        let resolver = make_resolver_with_tsconfig(
            &["src/bar.ts"],
            r#"{"compilerOptions": {"paths": {"@/*": ["./src/*"]}}}"#,
        );
        let result = resolver
            .resolve(&FileName::Real("/app/src/foo.ts".into()), "./bar")
            .unwrap();
        assert_eq!(result, FileName::Real("/app/src/bar.ts".into()));
    }
}
