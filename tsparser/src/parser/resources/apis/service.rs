use litparser::{report_and_continue, ParseResult};
use swc_common::sync::Lrc;
use swc_common::Spanned;
use swc_ecma_ast::{self as ast};

use litparser::LitParser;
use litparser_derive::LitParser;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::ResourceOrPath;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::ReferenceParser;
use crate::parser::resources::parseutil::{
    extract_bind_name, extract_resource_name, iter_references, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::{FilePath, Range};
use crate::span_err::ErrReporter;

#[derive(Debug, Clone)]
pub struct Service {
    pub range: Range,
    pub name: String,
    pub doc: Option<String>,
}

#[allow(dead_code)]
#[derive(LitParser, Default, Debug)]
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
            for (i, r) in iter_references::<ServiceLiteral>(&module, &names).enumerate() {
                let r = report_and_continue!(r);

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

                let resource = Resource::Service(Lrc::new(Service {
                    range: r.range,
                    name: r.resource_name,
                    doc: r.doc_comment,
                }));
                pass.add_resource(resource.clone());
                pass.add_bind(BindData {
                    range: r.range,
                    resource: ResourceOrPath::Resource(resource),
                    object: None,
                    kind: BindKind::Create,
                    ident: None,
                });
            }
        }
    },
};

#[allow(dead_code)]
#[derive(Debug)]
struct ServiceLiteral {
    pub range: Range,
    pub doc_comment: Option<String>,
    pub resource_name: String,
    pub config: Option<DecodedServiceConfig>,
}

impl ReferenceParser for ServiceLiteral {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        for node in path.iter().rev() {
            if let swc_ecma_visit::AstParentNodeRef::NewExpr(
                expr,
                swc_ecma_visit::fields::NewExprField::Callee,
            ) = node
            {
                let Some(args) = &expr.args else {
                    expr.span().err("missing constructor arguments");
                    continue;
                };

                if let Some(bind_name) = extract_bind_name(path)? {
                    bind_name
                        .span()
                        .err("service definitions should not be bound to a variable");
                    continue;
                }

                let resource_name = extract_resource_name(expr.span, args, 0)?;
                let doc_comment = module.preceding_comments(expr.span.lo.into());

                let config = args
                    .get(1)
                    .map(|arg| DecodedServiceConfig::parse_lit(&arg.expr))
                    .transpose()?;

                if !is_default_export(path, expr) {
                    expr.span().err("service must be default export");
                    continue;
                }

                return Ok(Some(Self {
                    range: expr.span.into(),
                    doc_comment,
                    resource_name: resource_name.to_string(),
                    config,
                }));
            }
        }

        Ok(None)
    }
}

// checks if `new_expr` is the default export in `path`
fn is_default_export(path: &swc_ecma_visit::AstNodePath, new_expr: &swc_ecma_ast::NewExpr) -> bool {
    for node in path.iter().rev() {
        match node {
            swc_ecma_visit::AstParentNodeRef::ExportDefaultExpr(
                swc_ecma_ast::ExportDefaultExpr { expr, .. },
                swc_ecma_visit::fields::ExportDefaultExprField::Expr,
            ) => {
                return if let swc_ecma_ast::Expr::New(ref expr) = **expr {
                    expr == new_expr
                } else {
                    false
                }
            }
            _ => continue,
        }
    }
    false
}
