use std::collections::{HashMap, HashSet};

use matchit::InsertError;
use swc_common::errors::HANDLER;
use swc_common::Span;

use crate::encore::parser::meta::v1;
use crate::legacymeta::compute_meta;
use crate::parser::parser::{ParseContext, ParseResult};
use crate::parser::resources::apis::api::{Endpoint, Method, Methods};
use crate::parser::resources::Resource;
use crate::parser::respath::Path;
use crate::parser::types::visitor::VisitWith;
use crate::parser::types::{validation, visitor, ObjectId, ResolveState, Type, Validated};
use crate::parser::Range;
use crate::span_err::ErrReporter;
use litparser::Sp;

#[derive(Debug)]
pub struct AppDesc {
    pub parse: ParseResult,
    pub meta: v1::Data,
}

struct Router {
    methods: HashMap<Method, matchit::Router<Range>>,
}

impl Router {
    fn new() -> Self {
        Router {
            methods: HashMap::new(),
        }
    }
}

impl Router {
    fn try_add(&mut self, methods: &Methods, path: &Path, range: Range) {
        let methods = match methods {
            Methods::All => Method::all().to_vec(),
            Methods::Some(vec) => vec.to_vec(),
        };

        for method in methods {
            let method_router = self.methods.entry(method).or_default();
            if let Err(e) = method_router.insert(path.to_string(), range) {
                match e {
                    InsertError::Conflict { with } => {
                        let prev_range = *method_router.at(&with).unwrap().value;
                        HANDLER.with(|handler| {
                            handler
                                .struct_span_err(
                                    range,
                                    "api endpoints with conflicting paths defined",
                                )
                                .span_note(prev_range, "previously defined here")
                                .emit()
                        })
                    }
                    _ => HANDLER.with(|handler| handler.span_err(range, &e.to_string())),
                }
            }
        }
    }
}

pub fn validate_and_describe(pc: &ParseContext, parse: ParseResult) -> Option<AppDesc> {
    AppValidator { pc, parse: &parse }.validate();

    if pc.errs.has_errors() {
        return None;
    }

    match compute_meta(pc, &parse) {
        Ok(meta) => Some(AppDesc { parse, meta }),
        Err(err) => {
            err.report();
            None
        }
    }
}

struct AppValidator<'a> {
    pc: &'a ParseContext,
    parse: &'a ParseResult,
}

impl AppValidator<'_> {
    fn validate(&self) {
        self.validate_apis();
        self.validate_pubsub()
    }

    fn validate_apis(&self) {
        for service in self.parse.services.iter() {
            let mut router = Router::new();
            for bind in &service.binds {
                if let Resource::APIEndpoint(endpoint) = &bind.resource {
                    let encoding = &endpoint.encoding;
                    router.try_add(&encoding.methods, &encoding.path, endpoint.range);

                    self.validate_endpoint(endpoint);
                }
            }
        }
    }

    fn validate_endpoint(&self, ep: &Endpoint) {
        if let Some(schema) = &ep.encoding.raw_req_schema {
            self.validate_validations(schema);
        }
        if let Some(schema) = &ep.encoding.raw_resp_schema {
            self.validate_validations(schema);
        }
    }

    fn validate_validations(&self, schema: &Sp<Type>) {
        struct Visitor<'a> {
            state: &'a ResolveState,
            span: Span,
            seen_decls: HashSet<ObjectId>,
        }

        impl visitor::Visit for Visitor<'_> {
            fn resolve_state(&self) -> &ResolveState {
                self.state
            }
            fn seen_decls(&mut self) -> &mut HashSet<ObjectId> {
                &mut self.seen_decls
            }

            fn visit_validated(&mut self, node: &Validated) {
                if let Err(err) = node.expr.supports_type(&node.typ) {
                    let s = err.to_string();
                    self.span.err(&s);
                } else {
                    // Don't recurse into the validation expression, as it would report an error
                    // below as if the expression was standalone.
                    node.typ.visit_with(self);
                }
            }

            fn visit_validation(&mut self, node: &validation::Expr) {
                HANDLER.with(|h| {
                    h.struct_span_err(
                        self.span,
                        &format!("unsupported standalone validation expression: {}", node),
                    )
                    .note("validation expressions must be attached to a regular type using '&'")
                    .emit()
                });
            }
        }

        let state = self.pc.type_checker.state();
        let mut visitor = Visitor {
            state,
            span: schema.span(),
            seen_decls: HashSet::new(),
        };
        schema.visit_with(&mut visitor);
    }

    fn validate_pubsub(&self) {
        for res in self.parse.resources.iter() {
            if let Resource::PubSubTopic(topic) = &res {
                self.validate_validations(&topic.message_type);
            }
        }
    }
}
