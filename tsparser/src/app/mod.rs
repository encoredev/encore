use std::collections::HashMap;

use anyhow::Result;
use matchit::InsertError;
use swc_common::errors::HANDLER;

use crate::encore::parser::meta::v1;
use crate::legacymeta::compute_meta;
use crate::parser::parser::{ParseContext, ParseResult};
use crate::parser::resources::apis::api::{Method, Methods};
use crate::parser::resources::Resource;
use crate::parser::respath::Path;
use crate::parser::Range;

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

impl AppDesc {
    fn validate(&self) {
        self.validate_apis()
    }

    fn validate_apis(&self) {
        for service in self.parse.services.iter() {
            let mut router = Router::new();
            for bind in &service.binds {
                if let Resource::APIEndpoint(endpoint) = &bind.resource {
                    let encoding = &endpoint.encoding;
                    router.try_add(&encoding.methods, &encoding.path, endpoint.range);
                }
            }
        }
    }
}

pub fn validate_and_describe(pc: &ParseContext, parse: ParseResult) -> Result<AppDesc> {
    let meta = compute_meta(pc, &parse)?;
    let desc = AppDesc { parse, meta };

    desc.validate();

    Ok(desc)
}
