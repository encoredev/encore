use litparser::report_and_continue;
use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_common::Span;

use crate::parser::resourceparser::bind::ResourceOrPath;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    iter_references, resolve_object_for_bind_name, validate_snake_case_name, TrackedNames,
};
use crate::parser::resources::parseutil::{NamedClassResourceOptionalConfig, NamedStaticMethod};
use crate::parser::resources::Resource;
use crate::parser::resources::ResourcePath;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;
use crate::span_err::ErrReporter;

#[derive(Debug, Clone)]
pub struct MongoDatabase {
    pub span: Span,
    pub name: String,
    pub doc: Option<String>,
}

/// The config for a MongoDB database. It has no fields yet.
#[derive(LitParser, Default, Debug)]
struct DecodedDatabaseConfig {}

pub const MONGODB_PARSER: ResourceParser = ResourceParser {
    name: "mongodb",
    interesting_pkgs: &[PkgPath("encore.dev/storage/mongodb")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/storage/mongodb", "MongoDatabase")]);

        let module = pass.module.clone();
        type Res = NamedClassResourceOptionalConfig<DecodedDatabaseConfig>;
        for r in iter_references::<Res>(&module, &names) {
            let r = report_and_continue!(r);

            if let Err(err_msg) = validate_snake_case_name(&r.resource_name, None) {
                r.range.err(&format!(
                    "invalid MongoDB database name '{}': {}.",
                    r.resource_name, err_msg
                ));
                continue;
            }

            let object =
                resolve_object_for_bind_name(pass.type_checker, pass.module.clone(), &r.bind_name);

            let resource = Resource::MongoDatabase(Lrc::new(MongoDatabase {
                span: r.range.to_span(),
                name: r.resource_name,
                doc: r.doc_comment,
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

        for r in iter_references::<NamedStaticMethod>(&module, &names) {
            let r = report_and_continue!(r);
            let object =
                resolve_object_for_bind_name(pass.type_checker, pass.module.clone(), &r.bind_name);
            pass.add_bind(BindData {
                range: r.range,
                resource: ResourceOrPath::Path(ResourcePath::MongoDatabase {
                    name: r.resource_name,
                }),
                object,
                kind: BindKind::Reference,
                ident: r.bind_name,
            });
        }
    },
};

pub fn resolve_database_usage(data: &ResolveUsageData, db: Lrc<MongoDatabase>) -> Option<Usage> {
    match &data.expr.kind {
        UsageExprKind::MethodCall(_)
        | UsageExprKind::FieldAccess(_)
        | UsageExprKind::CallArg(_)
        | UsageExprKind::ConstructorArg(_) => {
            Some(Usage::AccessMongoDatabase(AccessMongoDatabaseUsage {
                range: data.expr.range,
                db,
            }))
        }

        UsageExprKind::TemplateCall(_) | UsageExprKind::Other(_) | UsageExprKind::Callee(_) => {
            data.expr.err("invalid use of MongoDB database resource");
            None
        }
    }
}

#[derive(Debug)]
pub struct AccessMongoDatabaseUsage {
    pub range: Range,
    pub db: Lrc<MongoDatabase>,
}
