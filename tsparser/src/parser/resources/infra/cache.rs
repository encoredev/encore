use std::rc::Rc;

use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_common::Span;
use swc_common::Spanned;
use swc_ecma_ast as ast;

use litparser::{report_and_continue, LitParser, Sp, ToParseErr};

use crate::parser::resourceparser::bind::{BindData, BindKind, BindName, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    extract_bind_name, extract_type_param, is_default_export, iter_references,
    resolve_object_for_bind_name, NamedClassResourceOptionalConfig, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::types::Object;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;
use crate::span_err::ErrReporter;

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
    /// Reference to the cache cluster object.
    pub cluster: Sp<Rc<Object>>,
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

#[allow(non_snake_case)]
#[derive(LitParser, Default, Debug)]
struct DecodedClusterConfig {
    #[allow(dead_code)]
    evictionPolicy: Option<Sp<String>>,
}

#[allow(non_snake_case)]
#[derive(LitParser, Debug)]
struct DecodedKeyspaceConfig {
    keyPattern: Sp<String>,
    #[allow(dead_code)]
    defaultExpiry: Option<ast::Expr>,
}

/// Specification for a keyspace constructor.
struct KeyspaceConstructorSpec {
    class_name: &'static str,
    keyspace_type: KeyspaceType,
    /// Number of type parameters (1 for implicit value types, 2 for explicit)
    num_type_params: usize,
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
                    .and_then(|c| c.evictionPolicy.as_ref())
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

/// All supported keyspace constructors.
const KEYSPACE_CONSTRUCTORS: &[KeyspaceConstructorSpec] = &[
    KeyspaceConstructorSpec {
        class_name: "StringKeyspace",
        keyspace_type: KeyspaceType::String,
        num_type_params: 1, // Only key type, value is implicitly string
    },
    KeyspaceConstructorSpec {
        class_name: "IntKeyspace",
        keyspace_type: KeyspaceType::Int,
        num_type_params: 1, // Only key type, value is implicitly int
    },
    KeyspaceConstructorSpec {
        class_name: "FloatKeyspace",
        keyspace_type: KeyspaceType::Float,
        num_type_params: 1, // Only key type, value is implicitly float
    },
    KeyspaceConstructorSpec {
        class_name: "ListKeyspace",
        keyspace_type: KeyspaceType::List,
        num_type_params: 2, // Key type and element type
    },
    KeyspaceConstructorSpec {
        class_name: "SetKeyspace",
        keyspace_type: KeyspaceType::Set,
        num_type_params: 2, // Key type and element type
    },
    KeyspaceConstructorSpec {
        class_name: "StructKeyspace",
        keyspace_type: KeyspaceType::Struct,
        num_type_params: 2, // Key type and value type
    },
];

pub const CACHE_KEYSPACE_PARSER: ResourceParser = ResourceParser {
    name: "cache_keyspace",
    interesting_pkgs: &[PkgPath("encore.dev/storage/cache")],

    run: |pass| {
        // Build tracked names for all keyspace constructors
        let keyspace_names: Vec<(&str, &str)> = KEYSPACE_CONSTRUCTORS
            .iter()
            .map(|spec| ("encore.dev/storage/cache", spec.class_name))
            .collect();

        let names = TrackedNames::new(&keyspace_names);
        let module = pass.module.clone();

        // Create a map from class name to spec for quick lookup
        let spec_map: std::collections::HashMap<&str, &KeyspaceConstructorSpec> =
            KEYSPACE_CONSTRUCTORS
                .iter()
                .map(|spec| (spec.class_name, spec))
                .collect();

        for r in iter_references::<KeyspaceReference>(&module, &names) {
            let r = report_and_continue!(r);

            // Get the spec for this keyspace type
            let Some(spec) = spec_map.get(r.class_name.as_str()) else {
                continue;
            };

            // Validate we have the right number of type parameters
            let type_args = r.expr.type_args.as_deref();
            let key_type_ast = extract_type_param(type_args, 0);
            let value_type_ast = if spec.num_type_params > 1 {
                extract_type_param(type_args, 1)
            } else {
                None
            };

            if key_type_ast.is_none() {
                r.expr.span().err("missing key type parameter");
                continue;
            }

            // Resolve the key type
            let key_type = pass
                .type_checker
                .resolve_type(pass.module.clone(), key_type_ast.unwrap());

            // Resolve the value type (if explicit)
            let value_type =
                value_type_ast.map(|vt| pass.type_checker.resolve_type(pass.module.clone(), vt));

            // Resolve the cluster reference from the stored expression
            let cluster_span = r.cluster_expr.span();
            let Some(cluster_obj) = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &r.cluster_expr)
            else {
                r.expr
                    .span()
                    .err("could not resolve cache cluster reference");
                continue;
            };

            let object =
                resolve_object_for_bind_name(pass.type_checker, pass.module.clone(), &r.bind_name);

            let resource = Resource::CacheKeyspace(Lrc::new(CacheKeyspace {
                span: r.expr.span(),
                cluster: Sp::new(cluster_span, cluster_obj),
                doc: r.doc_comment.clone(),
                keyspace_type: spec.keyspace_type,
                key_pattern: r.config.keyPattern.to_string(),
                default_expiry_ms: None, // TODO: parse defaultExpiry if needed
                key_type,
                value_type,
            }));

            pass.add_resource(resource.clone());
            pass.add_bind(BindData {
                range: r.expr.span().into(),
                resource: ResourceOrPath::Resource(resource),
                object,
                kind: BindKind::Create,
                ident: r.bind_name,
            });
        }
    },
};

/// Parsed keyspace reference from constructor call.
#[derive(Debug)]
struct KeyspaceReference {
    expr: ast::NewExpr,
    class_name: String,
    /// The AST expression for the cluster argument (resolved later in the main loop).
    cluster_expr: Box<ast::Expr>,
    config: DecodedKeyspaceConfig,
    doc_comment: Option<String>,
    bind_name: BindName,
}

impl ReferenceParser for KeyspaceReference {
    fn parse_resource_reference(
        module: &crate::parser::module_loader::Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> litparser::ParseResult<Option<Self>> {
        use swc_ecma_visit::AstParentNodeRef;

        for node in path.iter().rev() {
            if let AstParentNodeRef::NewExpr(expr, swc_ecma_visit::fields::NewExprField::Callee) =
                node
            {
                let Some(args) = &expr.args else {
                    return Err(expr.span.parse_err("missing constructor arguments"));
                };

                if args.len() < 2 {
                    return Err(expr
                        .span
                        .parse_err("keyspace constructor requires cluster and config arguments"));
                }

                // Extract the class name from the callee
                let class_name = match &*expr.callee {
                    ast::Expr::Ident(ident) => ident.sym.to_string(),
                    _ => {
                        return Err(expr.span.parse_err("expected keyspace class name"));
                    }
                };

                // First argument is the cluster reference
                let cluster_arg = &args[0];
                if cluster_arg.spread.is_some() {
                    return Err(cluster_arg
                        .span()
                        .parse_err("cannot use spread for cluster"));
                }

                // Store the cluster expression to be resolved later
                let cluster_expr = cluster_arg.expr.clone();

                // Second argument is the config
                let config_arg = &args[1];
                if config_arg.spread.is_some() {
                    return Err(config_arg.span().parse_err("cannot use spread for config"));
                }

                let config = DecodedKeyspaceConfig::parse_lit(&config_arg.expr)?;

                let bind_name = match extract_bind_name(path)? {
                    Some(name) => BindName::Named(name),
                    None => {
                        if is_default_export(path, (*expr).into()) {
                            BindName::DefaultExport
                        } else {
                            BindName::Anonymous
                        }
                    }
                };

                let doc_comment = module.preceding_comments(expr.span.lo.into());

                return Ok(Some(Self {
                    expr: (*expr).clone(),
                    class_name,
                    cluster_expr,
                    config,
                    doc_comment,
                    bind_name,
                }));
            }
        }

        Ok(None)
    }
}

impl Spanned for KeyspaceReference {
    fn span(&self) -> Span {
        self.expr.span()
    }
}

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
                | "listKeyspace" | "setKeyspace" => Some(Usage::CacheCluster(CacheClusterUsage {
                    cluster,
                    operation: "keyspace".to_string(),
                    range: data.expr.range.clone(),
                })),
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
