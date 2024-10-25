use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::LitParser;
use litparser_derive::LitParser;

use crate::parser::resourceparser::bind::ResourceOrPath;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::NamedClassResourceOptionalConfig;
use crate::parser::resources::parseutil::{iter_references, TrackedNames};
use crate::parser::resources::Resource;
use crate::parser::{FilePath, Range};

#[derive(Debug, Clone)]
pub struct Service {
    pub range: Range,
    pub name: String,
    pub doc: Option<String>,
}

#[allow(dead_code)]
#[derive(LitParser, Default)]
struct DecodedServiceConfig {
    middlewares: Option<ast::Expr>,
}

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
                    r.range
                        .err("cannot have multiple service declarations in the same module");
                    continue;
                }

                // This resource is only allowed to be defined in a module named "encore.service.ts".
                // Check that that is the case.
                match &pass.module.file_path {
                    FilePath::Real(buf) if buf.ends_with("encore.service.ts") => {}
                    _ => {
                        r.range
                            .err("service declarations are only allowed in encore.service.ts");
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
                    range: r.range,
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
