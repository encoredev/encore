use std::collections::HashMap;
use std::collections::HashSet;
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
    resolve_object_for_bind_name, NamedClassResourceOptionalConfig, NamedStaticMethod,
    ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::resources::ResourcePath;
use crate::parser::types::Basic;
use crate::parser::types::FieldName;
use crate::parser::types::Object;
use crate::parser::types::Type;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;
use crate::span_err::ErrReporter;

/// Redis eviction policy.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum EvictionPolicy {
    NoEviction,
    #[default]
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
    /// Stores list of string values.
    StringList,
    /// Stores list of numeric values.
    NumberList,
    /// Stores set of unique string values.
    StringSet,
    /// Stores set of unique numeric values.
    NumberSet,
}

impl KeyspaceType {
    pub fn as_str(&self) -> &'static str {
        match self {
            KeyspaceType::Struct => "struct",
            KeyspaceType::String => "string",
            KeyspaceType::Int => "int",
            KeyspaceType::Float => "float",
            KeyspaceType::StringList => "string_list",
            KeyspaceType::NumberList => "number_list",
            KeyspaceType::StringSet => "string_set",
            KeyspaceType::NumberSet => "number_set",
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

                let eviction_policy = match r
                    .config
                    .as_ref()
                    .and_then(|c| c.evictionPolicy.as_ref())
                {
                    Some(policy) => match policy.parse() {
                        Ok(p) => p,
                        Err(_) => {
                            policy.span().err("invalid eviction policy: must be one of noeviction, allkeys-lru, allkeys-lfu, allkeys-random, volatile-lru, volatile-lfu, volatile-ttl, or volatile-random");
                            continue;
                        }
                    },
                    None => EvictionPolicy::default(),
                };

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

        {
            for r in iter_references::<NamedStaticMethod>(&module, &names) {
                let r = report_and_continue!(r);
                let object = resolve_object_for_bind_name(
                    pass.type_checker,
                    pass.module.clone(),
                    &r.bind_name,
                );
                pass.add_bind(BindData {
                    range: r.range,
                    resource: ResourceOrPath::Path(ResourcePath::CacheCluster {
                        name: r.resource_name,
                    }),
                    object,
                    kind: BindKind::Reference,
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
        class_name: "StringListKeyspace",
        keyspace_type: KeyspaceType::StringList,
        num_type_params: 1, // Only key type, value is implicitly string
    },
    KeyspaceConstructorSpec {
        class_name: "NumberListKeyspace",
        keyspace_type: KeyspaceType::NumberList,
        num_type_params: 1, // Only key type, value is implicitly number
    },
    KeyspaceConstructorSpec {
        class_name: "StringSetKeyspace",
        keyspace_type: KeyspaceType::StringSet,
        num_type_params: 1, // Only key type, value is implicitly string
    },
    KeyspaceConstructorSpec {
        class_name: "NumberSetKeyspace",
        keyspace_type: KeyspaceType::NumberSet,
        num_type_params: 1, // Only key type, value is implicitly number
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
        let spec_map: HashMap<&str, &KeyspaceConstructorSpec> = KEYSPACE_CONSTRUCTORS
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

            // Validate key type - disallow any/unknown
            let is_invalid_key_type = match key_type.as_ref() {
                Type::Basic(Basic::Any) => Some("'any' is not supported as a cache key type"),
                Type::Basic(Basic::Unknown) => {
                    Some("'unknown' is not supported as a cache key type")
                }
                _ => None,
            };
            if let Some(err_msg) = is_invalid_key_type {
                key_type_ast.unwrap().span().err(err_msg);
                continue;
            }

            // Resolve the value type (if explicit from type parameter, or implicit from keyspace type)
            let value_type = if let Some(vt) = value_type_ast {
                // Explicit value type from type parameter
                Some(pass.type_checker.resolve_type(pass.module.clone(), vt))
            } else {
                // Implicit value type based on keyspace type
                match spec.keyspace_type {
                    KeyspaceType::String | KeyspaceType::StringList | KeyspaceType::StringSet => {
                        Some(Sp::new(r.expr.span(), Type::Basic(Basic::String)))
                    }
                    KeyspaceType::Int
                    | KeyspaceType::Float
                    | KeyspaceType::NumberList
                    | KeyspaceType::NumberSet => {
                        Some(Sp::new(r.expr.span(), Type::Basic(Basic::Number)))
                    }
                    // Struct keyspace requires explicit value type parameter
                    _ => None,
                }
            };

            // Validate key pattern
            let key_pattern = &r.config.keyPattern;

            // Check for reserved prefix
            if key_pattern.starts_with("__encore") {
                key_pattern
                    .span()
                    .err("the prefix `__encore` is reserved for internal use by Encore");
                continue;
            }

            // For basic (non-struct) key types, the parameter must be named `:key`
            if matches!(key_type.as_ref(), Type::Basic(_)) {
                // Check that any parameter segment is named "key"
                let mut has_invalid_param = false;
                let pattern_str: &str = key_pattern.as_str();
                for segment in pattern_str.split('/') {
                    if let Some(param_name) = segment.strip_prefix(':') {
                        if param_name != "key" {
                            key_pattern.span().err(
                                "KeyPattern parameter must be named ':key' for basic (non-struct) key types",
                            );
                            has_invalid_param = true;
                            break;
                        }
                    }
                }
                if has_invalid_param {
                    continue;
                }
            } else {
                // Struct key type validation: validate interface fields match key pattern
                let key_typ = match key_type.as_ref() {
                    Type::Named(named) => &named.underlying(pass.type_checker.state()),
                    typ => typ,
                };

                let interface_fields = match key_typ {
                    Type::Interface(iface) => &iface.fields,
                    t => {
                        key_type.span().err(&format!("unsupported key type: {t}"));
                        continue;
                    }
                };

                // Extract parameter names from key pattern
                let pattern_str: &str = key_pattern.as_str();
                let mut pattern_params: HashSet<&str> = HashSet::new();
                for segment in pattern_str.split('/') {
                    if let Some(param) = segment.strip_prefix(':') {
                        pattern_params.insert(param);
                    }
                }

                // Validate each field
                let mut field_names: HashSet<String> = HashSet::new();
                let mut has_error = false;

                for field in interface_fields {
                    let field_name = match &field.name {
                        FieldName::String(s) => s.clone(),
                        FieldName::Symbol(_) => {
                            key_type_ast
                                .unwrap()
                                .span()
                                .err("cache key type must not contain symbol fields");
                            has_error = true;
                            continue;
                        }
                    };

                    // Check field type is a basic type (string, number, boolean)
                    let is_valid_field_type = matches!(
                        &field.typ,
                        Type::Basic(Basic::String)
                            | Type::Basic(Basic::Number)
                            | Type::Basic(Basic::Boolean)
                            | Type::Basic(Basic::BigInt)
                    );

                    if !is_valid_field_type {
                        key_type_ast.unwrap().span().err(&format!(
                                "cache key field '{}' must be a basic type (string, number, or boolean)",
                                field_name
                            ));
                        has_error = true;
                        continue;
                    }

                    // Check field is used in key pattern
                    if !pattern_params.contains(field_name.as_str()) {
                        key_type_ast.unwrap().span().err(&format!(
                            "cache key field '{}' is not used in the keyPattern",
                            field_name
                        ));
                        has_error = true;
                    }

                    field_names.insert(field_name);
                }

                // Check all pattern params correspond to fields
                for param in &pattern_params {
                    if !field_names.contains(*param) {
                        key_pattern.span().err(&format!(
                            "keyPattern parameter '{}' does not exist in the key type",
                            param
                        ));
                        has_error = true;
                    }
                }

                if has_error {
                    continue;
                }
            }

            // For StructKeyspace, validate that the value type is an interface/object type
            if spec.keyspace_type == KeyspaceType::Struct {
                if let Some(ref vt) = value_type {
                    let vt = match vt.as_ref() {
                        Type::Named(named) => &named.underlying(pass.type_checker.state()),
                        typ => typ,
                    };
                    if !matches!(vt, Type::Interface(_) | Type::Class(_)) {
                        value_type_ast
                            .unwrap()
                            .span()
                            .err("StructKeyspace value type must be an interface or object type");
                        continue;
                    }
                }
            }

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
        UsageExprKind::ConstructorArg(_) => {
            // Track when cluster is passed to keyspace constructors.
            // e.g., new StringKeyspace(cluster, { ... })
            Some(Usage::CacheCluster(CacheClusterUsage {
                cluster,
                range: data.expr.range,
            }))
        }
        _ => None,
    }
}

#[derive(Debug)]
pub struct CacheClusterUsage {
    pub cluster: Lrc<CacheCluster>,
    pub range: Range,
}
