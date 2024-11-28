use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    extract_bind_name, iter_references, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::Range;
use crate::span_err::ErrReporter;
use litparser::{report_and_continue, LitParser, ParseResult};
use swc_common::errors::HANDLER;
use swc_common::sync::Lrc;
use swc_common::Span;
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
            let r = report_and_continue!(r);
            let resource = Resource::Secret(Lrc::new(Secret {
                range: r.range,
                name: r.secret_name,
                doc: r.doc_comment,
            }));

            let object = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &ast::Expr::Ident(r.bind_name.clone()));

            pass.add_resource(resource.clone());
            pass.add_bind(BindData {
                range: r.range,
                resource: ResourceOrPath::Resource(resource),
                object,
                kind: BindKind::Create,
                ident: Some(r.bind_name),
            });
        }
    },
};

#[derive(Debug)]
struct SecretLiteral {
    pub range: Range,
    pub doc_comment: Option<String>,
    pub secret_name: String,
    pub bind_name: ast::Ident,
}

fn inside_function(path: &swc_ecma_visit::AstNodePath) -> Option<Span> {
    for item in path.iter().rev() {
        match item {
            swc_ecma_visit::AstParentNodeRef::ArrowExpr(expr, ..) => return Some(expr.span),
            swc_ecma_visit::AstParentNodeRef::Function(expr, ..) => return Some(expr.span),
            _ => continue,
        }
    }

    None
}

impl ReferenceParser for SecretLiteral {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        for node in path.iter().rev() {
            if let swc_ecma_visit::AstParentNodeRef::CallExpr(
                expr,
                swc_ecma_visit::fields::CallExprField::Callee,
            ) = node
            {
                if let Some(fn_span) = inside_function(path) {
                    HANDLER.with(|handler| {
                        handler
                            .struct_span_err(expr.span, "secrets must be defined globally")
                            .span_note(fn_span, "secret defined within this function")
                            .emit();
                    });
                    return Ok(None);
                }

                let doc_comment = module.preceding_comments(expr.span.lo.into());
                let Some(bind_name) = extract_bind_name(path)? else {
                    expr.span.err("secrets must be bound to a variable");
                    continue;
                };

                let Some(secret_name) = &expr.args.first() else {
                    expr.span.err("secret() takes a single argument, the name of the secret as a string literal");
                    continue;
                };
                let secret_name = String::parse_lit(secret_name.expr.as_ref())?;

                return Ok(Some(Self {
                    range: expr.span.into(),
                    doc_comment,
                    secret_name,
                    bind_name,
                }));
            }
        }
        Ok(None)
    }
}
