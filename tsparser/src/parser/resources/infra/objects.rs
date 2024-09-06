use anyhow::Result;
use litparser_derive::LitParser;
use swc_common::errors::HANDLER;
use swc_ecma_ast as ast;
use swc_common::sync::Lrc;

use crate::parser::resourceparser::bind::ResourceOrPath;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{iter_references, TrackedNames};
use crate::parser::resources::parseutil::{NamedClassResourceOptionalConfig, NamedStaticMethod};
use crate::parser::resources::Resource;
use crate::parser::resources::ResourcePath;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;

#[derive(Debug, Clone)]
pub struct Bucket {
    pub name: String,
    pub doc: Option<String>,
}

#[derive(LitParser, Default)]
struct DecodedBucketConfig {}

pub const OBJECTS_PARSER: ResourceParser = ResourceParser {
    name: "objects",
    interesting_pkgs: &[PkgPath("encore.dev/storage/objects")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/storage/objects", "Bucket")]);

        let module = pass.module.clone();
        {
            type Res = NamedClassResourceOptionalConfig<DecodedBucketConfig>;
            for r in iter_references::<Res>(&module, &names) {
                let r = r?;

                // Not yet used.
                let _cfg = r.config.unwrap_or_default();

                let object = match &r.bind_name {
                    None => None,
                    Some(id) => pass
                        .type_checker
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
                };

                let resource = Resource::Bucket(Lrc::new(Bucket {
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
        }

        {
            for r in iter_references::<NamedStaticMethod>(&module, &names) {
                let r = r?;
                let object = match &r.bind_name {
                    None => None,
                    Some(id) => pass
                        .type_checker
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
                };

                pass.add_bind(BindData {
                    range: r.range,
                    resource: ResourceOrPath::Path(ResourcePath::Bucket {
                        name: r.resource_name,
                    }),
                    object,
                    kind: BindKind::Reference,
                    ident: r.bind_name,
                });
            }
        }

        Ok(())
    },
};

pub fn resolve_bucket_usage(data: &ResolveUsageData, bucket: Lrc<Bucket>) -> Result<Option<Usage>> {
    Ok(match &data.expr.kind {
        UsageExprKind::MethodCall(_)
        | UsageExprKind::FieldAccess(_)
        | UsageExprKind::CallArg(_)
        | UsageExprKind::ConstructorArg(_) => Some(Usage::AccessBucket(AccessBucketUsage {
            range: data.expr.range,
            bucket,
        })),

        _ => {
            HANDLER
                .with(|h| h.span_err(data.expr.range.to_span(), "invalid use of bucket resource"));
            None
        }
    })
}

#[derive(Debug)]
pub struct AccessBucketUsage {
    pub range: Range,
    pub bucket: Lrc<Bucket>,
}
