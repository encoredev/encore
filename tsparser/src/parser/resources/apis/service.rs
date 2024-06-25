use swc_common::errors::HANDLER;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser_derive::LitParser;

use crate::parser::resourceparser::bind::ResourceOrPath;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::NamedClassResourceOptionalConfig;
use crate::parser::resources::parseutil::{iter_references, TrackedNames};
use crate::parser::resources::Resource;
use crate::parser::FilePath;

#[derive(Debug, Clone)]
pub struct Service {
    pub name: String,
    pub doc: Option<String>,
}

#[derive(LitParser, Default)]
struct DecodedServiceConfig {}

pub static SERVICE_PARSER: ResourceParser = ResourceParser {
    name: "service",
    interesting_pkgs: &[PkgPath("encore.dev/service")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/service", "Service")]);

        let module = pass.module.clone();
        {
            type Res = NamedClassResourceOptionalConfig<DecodedServiceConfig>;
            for (i, r) in iter_references::<Res>(&module, &names).enumerate() {
                let r = r?;

                if i > 0 {
                    HANDLER.with(|h| {
                        h.struct_span_err(
                            r.range,
                            "cannot have multiple service declarations in the same module",
                        )
                        .emit();
                    });
                    continue;
                }

                // This resource is only allowed to be defined in a module named "encore.service.ts".
                // Check that that is the case.
                match &pass.module.file_path {
                    FilePath::Real(buf) if buf.ends_with("encore.service.ts") => {}
                    _ => {
                        HANDLER.with(|h| {
                            h.struct_span_err(
                                r.range,
                                "service declarations are only allowed in encore.service.ts",
                            )
                            .emit();
                        });
                        continue;
                    }
                }

                let object = match &r.bind_name {
                    None => None,
                    Some(id) => pass
                        .type_checker
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
                };

                let resource = Resource::Service(Lrc::new(Service {
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

        Ok(())
    },
};
