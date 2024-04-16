use crate::encore::parser::meta::v1 as meta;
use serde::Serialize;
use std::collections::{HashMap, HashSet};

pub trait Pather {
    type Key;
    type Value;

    fn key(&self) -> Self::Key;
    fn value(&self) -> Self::Value;
    fn path(&self) -> &meta::Path;
}

#[derive(Debug, Serialize, Clone)]
pub struct PathSet<K, V> {
    pub main: HashMap<K, Vec<(V, Vec<String>)>>,
    pub fallback: HashMap<K, Vec<(V, Vec<String>)>>,
}

/// Computes paths to register, grouped by the given key for easier correlation.
pub fn compute<P, K, V>(endpoints: impl Iterator<Item = P>) -> PathSet<K, V>
where
    P: Pather<Key = K, Value = V>,
    K: Eq + Clone + std::hash::Hash,
{
    use crate::encore::parser::meta::v1::path_segment::SegmentType;

    let mut main: HashMap<K, Vec<(V, Vec<String>)>> = HashMap::new();
    let mut fallback: HashMap<K, Vec<(V, Vec<String>)>> = HashMap::new();
    for ep in endpoints {
        let path = ep.path();
        let mut entries = Vec::with_capacity(2);

        // Compute the axum path.
        let mut result = String::new();
        for seg in &path.segments {
            let typ = SegmentType::try_from(seg.r#type).unwrap_or(SegmentType::Literal);
            match typ {
                SegmentType::Literal => {
                    result.push('/');
                    result.push_str(&seg.value)
                }
                SegmentType::Param => {
                    result.push_str("/:");
                    result.push_str(&seg.value)
                }
                SegmentType::Wildcard => {
                    // The wildcard is the last segment.
                    // Axum doesn't match e.g. "/" for "/*wildcard", so we need to register both.
                    result.push_str("/");
                    entries.push(result.clone());

                    result.push('*');
                    result.push_str(&seg.value);
                }
                SegmentType::Fallback => {
                    // Axum doesn't match e.g. "/" for "/*wildcard", so we need to register both.
                    result.push_str("/");
                    entries.push(result.clone());

                    result.push('*');
                    result.push_str(&seg.value);
                }
            }
        }
        entries.push(result);

        let is_fallback = path
            .segments
            .last()
            .map_or(false, |seg| seg.r#type == SegmentType::Fallback as i32);

        let key = ep.key();
        let routes = (ep.value(), entries);
        if is_fallback {
            fallback.entry(key).or_insert_with(Vec::new).push(routes);
        } else {
            main.entry(key).or_insert_with(Vec::new).push(routes);
        }
    }

    // Add paths for TSR redirects.
    {
        for paths in [&mut main, &mut fallback] {
            let path_set: HashSet<String> = HashSet::from_iter(
                paths
                    .values()
                    .flatten()
                    .flat_map(|(_, routes)| routes.iter())
                    .cloned(),
            );
            for entries in paths.values_mut() {
                for (_, endpoint_routes) in entries {
                    let mut tsr_routes = Vec::new();
                    for route in endpoint_routes.iter() {
                        // Is this entry incompatible with TSR?
                        if route == "/" || route.contains("/*") || route.ends_with('/') {
                            continue;
                        }

                        let tsr = format!("{}/", route);
                        if !path_set.contains(&tsr) {
                            tsr_routes.push(tsr);
                        }
                    }
                    endpoint_routes.extend(tsr_routes);
                }
            }
        }
    }

    PathSet { main, fallback }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::encore::parser::meta::v1::path_segment::SegmentType;
    use serde::ser::SerializeStruct;

    #[test]
    fn test_basic() {
        let endpoints = vec![
            ep("one", "a", &[lit("foo")]),
            ep("two", "a", &[lit("foo"), lit("bar")]),
        ];

        let paths = compute(endpoints.into_iter());
        insta::assert_yaml_snapshot!(paths);
    }

    #[test]
    fn test_tsr_conflict() {
        let endpoints = vec![
            ep("one", "a", &[lit("foo"), lit("")]),
            ep("two", "b", &[lit("foo")]),
        ];

        let paths = compute(endpoints.into_iter());
        insta::assert_yaml_snapshot!(paths);
    }

    #[test]
    fn test_wildcard() {
        let endpoints = vec![ep("one", "a", &[wildcard("foo")])];

        let paths = compute(endpoints.into_iter());
        insta::assert_yaml_snapshot!(paths);
    }

    #[test]
    fn test_fallback() {
        let endpoints = vec![ep("one", "a", &[fallback("foo")])];

        let paths = compute(endpoints.into_iter());
        insta::assert_yaml_snapshot!(paths);
    }

    fn path(segs: &[meta::PathSegment]) -> meta::Path {
        meta::Path {
            segments: segs.to_vec(),
            r#type: meta::path::Type::Url as i32,
        }
    }

    fn seg(typ: SegmentType, value: &str) -> meta::PathSegment {
        meta::PathSegment {
            r#type: typ as i32,
            value: value.to_string(),
            value_type: meta::path_segment::ParamType::String as i32,
        }
    }

    fn lit(str: &str) -> meta::PathSegment {
        seg(SegmentType::Literal, str)
    }

    fn param(str: &str) -> meta::PathSegment {
        seg(SegmentType::Param, str)
    }

    fn wildcard(str: &str) -> meta::PathSegment {
        seg(SegmentType::Wildcard, str)
    }

    fn fallback(str: &str) -> meta::PathSegment {
        seg(SegmentType::Fallback, str)
    }

    #[derive(Clone, Debug)]
    struct TestEndpoint {
        name: &'static str,
        key: &'static str,
        path: meta::Path,
    }

    impl Serialize for TestEndpoint {
        fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
        where
            S: serde::Serializer,
        {
            let mut state = serializer.serialize_struct("TestEndpoint", 2)?;
            let path = path_to_str(&self.path);
            state.serialize_field("key", &self.key)?;
            state.serialize_field("path", &path)?;
            state.end()
        }
    }

    fn ep(name: &'static str, key: &'static str, segs: &[meta::PathSegment]) -> TestEndpoint {
        TestEndpoint {
            name,
            key,
            path: path(segs),
        }
    }

    impl Pather for TestEndpoint {
        type Key = String;
        type Value = String;

        fn key(&self) -> Self::Key {
            self.key.to_string()
        }
        fn value(&self) -> Self::Value {
            self.name.to_string()
        }

        fn path(&self) -> &meta::Path {
            &self.path
        }
    }
}

pub fn path_to_str(path: &meta::Path) -> String {
    let mut result = String::new();
    for seg in &path.segments {
        result.push('/');

        use meta::path_segment::SegmentType;
        match SegmentType::try_from(seg.r#type).unwrap() {
            SegmentType::Literal => result.push_str(&seg.value),
            SegmentType::Param => {
                result.push(':');
                result.push_str(&seg.value)
            }
            SegmentType::Wildcard | SegmentType::Fallback => {
                result.push('*');
                result.push_str(&seg.value)
            }
        }
    }

    result
}
