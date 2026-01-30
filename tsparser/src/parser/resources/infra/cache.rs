use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_common::Span;

use litparser::{report_and_continue, LitParser, Sp};

use crate::parser::resourceparser::bind::{BindData, BindKind, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    iter_references, resolve_object_for_bind_name, NamedClassResourceOptionalConfig, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;

/// Redis eviction policy.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum EvictionPolicy {
    #[default]
    NoEviction,
    AllKeysLRU,
    AllKeysLFU,
    AllKeysRandom,
    VolatileLRU,
    VolatileLFU,
    VolatileTTL,
    VolatileRandom,
}

impl EvictionPolicy {
    pub fn as_str(&self) -> &'static str {
        match self {
            EvictionPolicy::NoEviction => "noeviction",
            EvictionPolicy::AllKeysLRU => "allkeys-lru",
            EvictionPolicy::AllKeysLFU => "allkeys-lfu",
            EvictionPolicy::AllKeysRandom => "allkeys-random",
            EvictionPolicy::VolatileLRU => "volatile-lru",
            EvictionPolicy::VolatileLFU => "volatile-lfu",
            EvictionPolicy::VolatileTTL => "volatile-ttl",
            EvictionPolicy::VolatileRandom => "volatile-random",
        }
    }
}

impl std::str::FromStr for EvictionPolicy {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "noeviction" => Ok(EvictionPolicy::NoEviction),
            "allkeys-lru" => Ok(EvictionPolicy::AllKeysLRU),
            "allkeys-lfu" => Ok(EvictionPolicy::AllKeysLFU),
            "allkeys-random" => Ok(EvictionPolicy::AllKeysRandom),
            "volatile-lru" => Ok(EvictionPolicy::VolatileLRU),
            "volatile-lfu" => Ok(EvictionPolicy::VolatileLFU),
            "volatile-ttl" => Ok(EvictionPolicy::VolatileTTL),
            "volatile-random" => Ok(EvictionPolicy::VolatileRandom),
            _ => Err(format!("unknown eviction policy: {}", s)),
        }
    }
}

/// A cache cluster resource.
#[derive(Debug, Clone)]
pub struct CacheCluster {
    pub span: Span,
    pub name: String,
    pub doc: Option<String>,
    pub eviction_policy: EvictionPolicy,
}

/// The type of keyspace (determines the value type).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum KeyspaceType {
    /// Stores arbitrary struct/object values (serialized as JSON).
    Struct,
    /// Stores string values.
    String,
    /// Stores integer values (i64).
    Int,
    /// Stores float values (f64).
    Float,
    /// Stores list of values.
    List,
    /// Stores set of values.
    Set,
}

impl KeyspaceType {
    pub fn as_str(&self) -> &'static str {
        match self {
            KeyspaceType::Struct => "struct",
            KeyspaceType::String => "string",
            KeyspaceType::Int => "int",
            KeyspaceType::Float => "float",
            KeyspaceType::List => "list",
            KeyspaceType::Set => "set",
        }
    }
}

/// A cache keyspace resource.
#[derive(Debug, Clone)]
pub struct CacheKeyspace {
    pub span: Span,
    pub cluster_name: String,
    pub doc: Option<String>,
    pub keyspace_type: KeyspaceType,
    /// The pattern for generating cache keys.
    pub key_pattern: String,
    /// Default expiry in milliseconds, if any.
    pub default_expiry_ms: Option<u64>,
    /// The key type (for generating key mappers).
    pub key_type: Sp<crate::parser::types::Type>,
    /// The value type (for struct keyspaces).
    pub value_type: Option<Sp<crate::parser::types::Type>>,
}

#[derive(LitParser, Default, Debug)]
struct DecodedClusterConfig {
    #[allow(dead_code)]
    eviction_policy: Option<Sp<String>>,
}

pub const CACHE_CLUSTER_PARSER: ResourceParser = ResourceParser {
    name: "cache_cluster",
    interesting_pkgs: &[PkgPath("encore.dev/storage/cache")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/storage/cache", "CacheCluster")]);

        let module = pass.module.clone();
        {
            type Res = NamedClassResourceOptionalConfig<DecodedClusterConfig>;
            for r in iter_references::<Res>(&module, &names) {
                let r = report_and_continue!(r);

                let eviction_policy = r
                    .config
                    .as_ref()
                    .and_then(|c| c.eviction_policy.as_ref())
                    .and_then(|p| p.parse().ok())
                    .unwrap_or_default();

                let object = resolve_object_for_bind_name(
                    pass.type_checker,
                    pass.module.clone(),
                    &r.bind_name,
                );

                let resource = Resource::CacheCluster(Lrc::new(CacheCluster {
                    span: r.range.to_span(),
                    name: r.resource_name.to_string(),
                    doc: r.doc_comment.clone(),
                    eviction_policy,
                }));

                pass.add_resource(resource.clone());
                pass.add_bind(BindData {
                    range: r.range,
                    resource: ResourceOrPath::Resource(resource),
                    object,
                    kind: BindKind::Create,
                    ident: r.bind_name,
                });
            }
        }
    },
};

// Note: CacheKeyspace parsing will be handled separately or via code generation.
// The keyspace creation pattern (cluster.keyspace(...)) requires more complex
// AST traversal that is better suited for a second-pass parser or code generation.
pub const CACHE_KEYSPACE_PARSER: ResourceParser = ResourceParser {
    name: "cache_keyspace",
    interesting_pkgs: &[PkgPath("encore.dev/storage/cache")],

    run: |_pass| {
        // Keyspace parsing is complex because it involves method calls on cluster instances.
        // For now, we'll handle keyspace metadata generation through code generation
        // similar to how the Go runtime handles it.
        // The runtime will parse keyspace configurations at initialization time.
    },
};

/// Resolves usage of cache resources.
pub fn resolve_cache_cluster_usage(
    data: &ResolveUsageData,
    cluster: Lrc<CacheCluster>,
) -> Option<Usage> {
    match &data.expr.kind {
        UsageExprKind::MethodCall(method) => {
            // Track keyspace creation methods (Go-style API)
            let method_name = method.method.sym.as_ref();
            match method_name {
                "keyspace" | "stringKeyspace" | "intKeyspace" | "floatKeyspace"
                | "listKeyspace" | "setKeyspace" => {
                    Some(Usage::CacheCluster(CacheClusterUsage {
                        cluster,
                        operation: "keyspace".to_string(),
                        range: data.expr.range.clone(),
                    }))
                }
                _ => None,
            }
        }
        UsageExprKind::ConstructorArg(_) => {
            // Track when cluster is passed to keyspace constructors (TypeScript-style API)
            // e.g., new StringKeyspace(cluster, { ... })
            Some(Usage::CacheCluster(CacheClusterUsage {
                cluster,
                operation: "keyspace".to_string(),
                range: data.expr.range.clone(),
            }))
        }
        _ => None,
    }
}

#[derive(Debug)]
pub struct CacheClusterUsage {
    pub cluster: Lrc<CacheCluster>,
    pub operation: String,
    pub range: Range,
}
