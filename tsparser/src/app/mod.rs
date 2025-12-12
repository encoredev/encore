use std::collections::{HashMap, HashSet};

use itertools::Itertools;
use matchit::InsertError;
use swc_common::errors::HANDLER;
use swc_common::Span;

use crate::encore::parser::meta::v1;
use crate::legacymeta::compute_meta;
use crate::parser::parser::{ParseContext, ParseResult};
use crate::parser::resources::apis::api::{Endpoint, Method, Methods};
use crate::parser::resources::apis::encoding::{Param, ParamData};
use crate::parser::resources::Resource;
use crate::parser::respath::Path;
use crate::parser::types::visitor::VisitWith;
use crate::parser::types::{
    validation, visitor, Basic, Custom, Interface, ObjectId, ResolveState, Type, Validated,
    WireLocation, WireSpec,
};
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
        self.validate_pubsub();
        self.validate_sqldb();
        self.validate_metrics();
    }

    fn validate_apis(&self) {
        let mut seen = std::collections::HashMap::new();
        for resource in &self.parse.resources {
            if let Resource::APIEndpoint(ep) = resource {
                let key = (ep.service_name.clone(), ep.name.clone());
                if let Some(prev) = seen.insert(key, ep.name_range) {
                    HANDLER.with(|handler| {
                        handler
                            .struct_span_err(
                                ep.name_range,
                                "api endpoints with conflicting names defined within the same service",
                            )
                            .span_note(prev, "previously defined here")
                            .emit();
                    })
                }
            }
        }

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
        if let Some(req_enc) = &ep.encoding.handshake {
            self.validate_req_params(&req_enc.params);
        }
        if !ep.streaming_request {
            for req_enc in &ep.encoding.req {
                self.validate_req_params(&req_enc.params);
            }
        }
        if !ep.streaming_response {
            self.validate_resp_params(&ep.encoding.resp.params);
        }
        if let Some(schema) = &ep.encoding.raw_handshake_schema {
            self.validate_schema_type(schema);
            self.validate_validations(schema);
        }
        if let Some(schema) = &ep.encoding.raw_req_schema {
            self.validate_schema_type(schema);
            self.validate_validations(schema);
        }
        if let Some(schema) = &ep.encoding.raw_resp_schema {
            self.validate_schema_type(schema);
            self.validate_validations(schema);
        }
    }

    fn validate_req_params(&self, params: &Vec<Param>) {
        for param in params {
            if let ParamData::Query { .. } = param.loc {
                fn is_valid_query_type(state: &ResolveState, typ: &Type) -> bool {
                    match resolve_to_concrete(state, typ) {
                        Type::Basic(_) | Type::Literal(_) => true,
                        Type::Enum(_) => true,
                        Type::Array(ref t) => is_valid_query_type(state, &t.0),
                        Type::Union(ref u) => u.types.iter().all(|t| is_valid_query_type(state, t)),
                        Type::Custom(Custom::Decimal) => true,
                        Type::Custom(Custom::WireSpec(WireSpec {
                            location: WireLocation::Query,
                            underlying: typ,
                            ..
                        })) => is_valid_query_type(state, &typ),
                        _ => false,
                    }
                }

                if !is_valid_query_type(self.pc.type_checker.state(), &param.typ) {
                    HANDLER.with(|handler| {
                        handler.span_err(param.range, "type not supported for query parameters")
                    });
                }
            };
        }
    }

    fn validate_resp_params(&self, params: &[Param]) {
        let http_status_params: Vec<_> = params
            .iter()
            .filter(|p| matches!(p.loc, ParamData::HTTPStatus))
            .sorted_by(|a, b| a.range.cmp(&b.range))
            .collect();

        if http_status_params.len() > 1 {
            let first = http_status_params[0];
            HANDLER.with(|handler| {
                let mut err = handler.struct_span_err(
                    first.range,
                    "http status can only be defined once per response type",
                );

                for param in &http_status_params[1..] {
                    err.span_note(param.range, "also defined here");
                }

                err.emit();
            });
        }
    }

    fn validate_schema_type(&self, schema: &Sp<Type>) {
        let state = self.pc.type_checker.state();
        let concrete = resolve_to_concrete(state, schema.get());

        let error_msg = match concrete {
            Type::Interface(Interface { index: Some(_), .. }) => {
                Some("type index is not supported in schema types")
            }
            Type::Interface(Interface { call: Some(_), .. }) => {
                Some("call signatures are not supported in schema types")
            }
            Type::Interface(_) | Type::Basic(Basic::Void) => None,
            _ => Some("request and response types must be interfaces or void"),
        };

        if let Some(msg) = error_msg {
            HANDLER.with(|handler| handler.span_err(schema.span(), msg));
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
                        &format!("unsupported standalone validation expression: {node}"),
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

    fn validate_sqldb(&self) {
        let mut seen = std::collections::HashMap::new();
        for resource in &self.parse.resources {
            if let Resource::SQLDatabase(db) = resource {
                if let Some(prev_range) = seen.insert(db.name.clone(), db.span) {
                    HANDLER.with(|handler| {
                        handler
                            .struct_span_err(db.span, "SQL Database with this name already defined")
                            .span_note(prev_range, "previously defined here")
                            .emit();
                    })
                }
            }
        }
    }

    fn validate_metrics(&self) {
        let mut seen = std::collections::HashMap::new();
        for resource in &self.parse.resources {
            if let Resource::Metric(metric) = resource {
                if let Some(prev_span) = seen.insert(metric.name.clone(), metric.span) {
                    HANDLER.with(|handler| {
                        handler
                            .struct_span_err(
                                metric.span,
                                &format!(
                                    "metric '{}' is defined multiple times; metrics must have unique names across the entire application",
                                    metric.name
                                ),
                            )
                            .span_note(prev_span, "previously defined here")
                            .emit();
                    })
                }
            }
        }
    }
}

fn resolve_to_concrete(state: &ResolveState, typ: &Type) -> Type {
    match typ {
        Type::Optional(opt) => resolve_to_concrete(state, &opt.0),
        Type::Validated(v) => resolve_to_concrete(state, &v.typ),
        Type::Named(named) => {
            let underlying = named.underlying(state);
            resolve_to_concrete(state, &underlying)
        }
        _ => typ.clone(),
    }
}
