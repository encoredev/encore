use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::LitParser;

use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{iter_references, TrackedNames, UnnamedClassResource};
use crate::parser::resources::Resource;
use crate::parser::types::Object;
use crate::parser::Range;

#[derive(Debug, Clone)]
pub struct Gateway {
    pub range: Range,
    pub name: String,
    pub doc: Option<String>,
    pub auth_handler: Option<Lrc<Object>>,
}

#[derive(Debug, LitParser)]
struct DecodedGatewayConfig {
    #[allow(non_snake_case)]
    authHandler: Option<ast::Expr>,
}

pub const GATEWAY_PARSER: ResourceParser = ResourceParser {
    name: "gateway",
    interesting_pkgs: &[PkgPath("encore.dev/api")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/api", "Gateway")]);

        let module = pass.module.clone();
        type Res = UnnamedClassResource<DecodedGatewayConfig>;
        for r in iter_references::<Res>(&module, &names) {
            let r = r?;
            let object = match &r.bind_name {
                None => None,
                Some(id) => pass
                    .type_checker
                    .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone()))?,
            };

            let auth_handler = if let Some(expr) = r.config.authHandler {
                let obj = pass
                    .type_checker
                    .resolve_obj(pass.module.clone(), &expr)?
                    .ok_or(anyhow::anyhow!("can't resolve endpoint"))?;
                Some(obj)
            } else {
                None
            };

            let resource = Resource::Gateway(Lrc::new(Gateway {
                range: r.range,
                name: "api-gateway".into(),
                doc: r.doc_comment,
                auth_handler,
            }));
            pass.add_resource(resource.clone());
            pass.add_bind(BindData {
                range: r.range,
                resource,
                object,
                kind: BindKind::Create,
                ident: r.bind_name,
            });
        }
        Ok(())
    },
};
