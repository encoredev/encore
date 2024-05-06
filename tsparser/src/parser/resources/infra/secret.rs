use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    extract_bind_name, iter_references, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::Range;
use anyhow::Result;
use litparser::LitParser;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

#[derive(Debug, Clone)]
pub struct Secret {
    pub range: Range,
    pub name: String,
    pub doc: Option<String>,
}

pub const SECRET_PARSER: ResourceParser = ResourceParser {
    name: "secret",
    interesting_pkgs: &[PkgPath("encore.dev/config")],

    run: |pass| {
        let module = pass.module.clone();
        let names = TrackedNames::new(&[("encore.dev/config", "secret")]);

        for r in iter_references::<SecretLiteral>(&module, &names) {
            let r = r?;
            let resource = Resource::Secret(Lrc::new(Secret {
                range: r.range,
                name: r.secret_name,
                doc: r.doc_comment,
            }));

            let object = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &ast::Expr::Ident(r.bind_name.clone()))?;

            pass.add_resource(resource.clone());
            pass.add_bind(BindData {
                range: r.range,
                resource: ResourceOrPath::Resource(resource),
                object,
                kind: BindKind::Create,
                ident: Some(r.bind_name),
            });
        }
        Ok(())
    },
};

#[derive(Debug)]
struct SecretLiteral {
    pub range: Range,
    pub doc_comment: Option<String>,
    pub secret_name: String,
    pub bind_name: ast::Ident,
}

impl ReferenceParser for SecretLiteral {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        for node in path.iter().rev() {
            match node {
                swc_ecma_visit::AstParentNodeRef::CallExpr(
                    expr,
                    swc_ecma_visit::fields::CallExprField::Callee,
                ) => {
                    let doc_comment = module.preceding_comments(expr.span.lo.into());
                    let Some(bind_name) = extract_bind_name(path)? else {
                        anyhow::bail!("Secrets must be bound to a variable")
                    };

                    let Some(secret_name) = &expr.args.get(0) else {
                        anyhow::bail!("secret() takes a single argument, the name of the secret as a string literal")
                    };
                    let secret_name = String::parse_lit(secret_name.expr.as_ref())?;

                    return Ok(Some(Self {
                        range: expr.span.into(),
                        doc_comment,
                        secret_name,
                        bind_name,
                    }));
                }

                _ => {}
            }
        }
        Ok(None)
    }
}
