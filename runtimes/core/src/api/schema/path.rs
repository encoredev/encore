use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::ptr;
use std::task::{Poll, RawWaker, RawWakerVTable, Waker};

use anyhow::Context;
use axum::extract::rejection::PathRejection;
use axum::extract::FromRequestParts;
use axum::http::request::Parts;
use indexmap::IndexMap;
use serde::de::DeserializeSeed;

use crate::api;
use crate::api::jsonschema::Basic;
use crate::api::schema::JSONPayload;
use crate::api::{jsonschema, APIResult};
use crate::encore::parser::meta::v1 as meta;

/// The URL path to an endpoint, e.g. ("/foo/bar/:id").
#[derive(Debug, Clone)]
pub struct Path {
    /// The path segments.
    segments: Vec<Segment>,
    dynamic_segments: Vec<jsonschema::Basic>,

    /// The capacity to use for generating requests.
    capacity: usize,
}

impl Path {
    pub fn from_meta(
        path: &meta::Path,
        schema: Option<jsonschema::JSONSchema>,
    ) -> anyhow::Result<Self> {
        let empty_fields = jsonschema::Struct {
            fields: HashMap::new(),
        };
        let fields = match &schema {
            Some(schema) => schema.root(),
            None => &empty_fields,
        };

        let mut segments = Vec::with_capacity(path.segments.len());
        for seg in &path.segments {
            use meta::path_segment::SegmentType;
            match SegmentType::try_from(seg.r#type).context("invalid path segment type")? {
                SegmentType::Literal => {
                    segments.push(Segment::Literal(seg.value.clone().into_boxed_str()))
                }
                SegmentType::Param => {
                    let name = &seg.value;
                    let field = fields
                        .fields
                        .get(name)
                        .ok_or_else(|| anyhow::anyhow!("missing field in request schema"))?;
                    let jsonschema::BasicOrValue::Basic(typ) = &field.value else {
                        anyhow::bail!("invalid field type in request schema");
                    };
                    segments.push(Segment::Param {
                        name: name.clone().into_boxed_str(),
                        typ: typ.clone(),
                    });
                }

                SegmentType::Wildcard => segments.push(Segment::Wildcard {
                    name: seg.clone().value.into_boxed_str(),
                }),
                SegmentType::Fallback => segments.push(Segment::Fallback {
                    name: seg.clone().value.into_boxed_str(),
                }),
            }
        }

        Ok(Self::from_segments(segments))
    }

    pub fn from_segments(segments: Vec<Segment>) -> Self {
        let mut capacity = 0;
        let mut dynamic_segments = Vec::new();
        for seg in segments.iter() {
            use Segment::*;
            capacity += 1; // slash
            match seg {
                Literal(lit) => capacity += lit.len(),
                Param { typ, .. } => {
                    capacity += 10; // assume path parameters on average are 10 characters long
                    dynamic_segments.push(*typ);
                }
                Wildcard { .. } | Fallback { .. } => {
                    // Assume path parameters on average are 10 characters long.
                    capacity += 10;
                    dynamic_segments.push(jsonschema::Basic::String);
                }
            }
        }

        Self {
            segments,
            dynamic_segments,
            capacity,
        }
    }
}

/// Represents a path segment.
#[derive(Debug, Clone)]
pub enum Segment {
    Literal(Box<str>),
    Param {
        name: Box<str>,
        /// The type of the path parameter.
        typ: jsonschema::Basic,
    },
    Wildcard {
        name: Box<str>,
    },
    Fallback {
        name: Box<str>,
    },
}

impl Path {
    /// Computes the request path to use for making an API call to this path with the given payload.
    pub fn to_request_path(&self, payload: &mut JSONPayload) -> Result<String, api::Error> {
        let mut path = String::with_capacity(self.capacity);
        for seg in self.segments.iter() {
            path.push('/');

            use Segment::*;
            match seg {
                Literal(lit) => path.push_str(&lit),
                Param { name, .. } | Wildcard { name } | Fallback { name } => {
                    let Some(payload) = payload else {
                        return Err(api::Error {
                            code: api::ErrCode::InvalidArgument,
                            message: "missing field in request payload".into(),
                            internal_message: Some(format!(
                                "missing field in request payload: {}",
                                &name
                            )),
                            stack: None,
                        });
                    };

                    // Find the data in the payload.
                    let Some(value) = payload.get(name.as_ref()) else {
                        return Err(api::Error {
                            code: api::ErrCode::InvalidArgument,
                            message: "missing field in request payload".into(),
                            internal_message: Some(format!(
                                "missing field in request payload: {}",
                                &name
                            )),
                            stack: None,
                        });
                    };

                    use serde_json::Value::*;
                    match value {
                        String(str) => path.push_str(str),
                        Null => path.push_str("null"),
                        Bool(bool) => path.push_str(if *bool { "true" } else { "false" }),
                        Number(num) => {
                            let str = num.to_string();
                            path.push_str(&str);
                        }
                        Array(_) | Object(_) => {
                            return Err(api::Error {
                                code: api::ErrCode::InvalidArgument,
                                message: "unsupported type in request payload".into(),
                                internal_message: Some(format!(
                                    "unsupported type in request payload for field {}",
                                    &name
                                )),
                                stack: None,
                            })
                        }
                    }
                }
            }
        }

        Ok(path)
    }
}

impl Path {
    pub fn parse_incoming_request_parts(
        &self,
        req: &mut Parts,
    ) -> APIResult<Option<IndexMap<String, serde_json::Value>>> {
        if self.dynamic_segments.is_empty() {
            return Ok(None);
        }

        let fut = axum::extract::Path::<Vec<(String, String)>>::from_request_parts(req, &());

        let result = resolve_immediate_future(fut).map_err(|_| api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "unable to parse path params".into(),
            internal_message: Some("polling path params returned pending".into()),
            stack: None,
        })?;

        match result {
            Ok(axum::extract::Path(params)) => {
                let mut map = IndexMap::with_capacity(params.len());

                // For each param, find the corresponding segment and deserialize it.
                for (idx, (name, val)) in params.into_iter().enumerate() {
                    if let Some(typ) = self.dynamic_segments.get(idx) {
                        // Decode it into the correct type based on the type.
                        let val = match &typ {
                            // For strings and any, use the value directly.
                            Basic::String | Basic::Any => serde_json::Value::String(val),

                            // For numbers and booleans, use the JSON parser.
                            Basic::Number | Basic::Bool => {
                                let mut de = serde_json::Deserializer::from_str(&val);
                                let val =
                                    DeserializeSeed::deserialize(*typ, &mut de).map_err(|err| {
                                        let expected = match typ {
                                            Basic::Number => "number",
                                            Basic::Bool => "boolean",
                                            _ => "value", // shouldn't happen
                                        };

                                        api::Error {
                                            code: api::ErrCode::InvalidArgument,
                                            message: format!(
                                                "path parameter is not a valid {}",
                                                expected
                                            ),
                                            internal_message: Some(err.to_string()),
                                            stack: None,
                                        }
                                    })?;
                                val
                            }

                            // We shouldn't have null here, but handle it just in case.
                            Basic::Null => serde_json::Value::Null,
                        };

                        map.insert(name, val);
                    }
                }

                Ok(Some(map))
            }
            Err(err) => Err(match err {
                PathRejection::FailedToDeserializePathParams(err) => api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: "unable to parse path params".into(),
                    internal_message: Some(err.to_string()),
                    stack: None,
                },
                PathRejection::MissingPathParams(err) => api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: "missing path params".into(),
                    internal_message: Some(err.to_string()),
                    stack: None,
                },
                err => api::Error {
                    code: api::ErrCode::Internal,
                    message: "unable to parse path params".into(),
                    internal_message: Some(err.to_string()),
                    stack: None,
                },
            }),
        }
    }
}

struct FuturePendingError;

fn resolve_immediate_future<F, T>(mut fut: F) -> Result<T, FuturePendingError>
where
    F: Future<Output = T> + Unpin,
{
    let waker = noop_waker();
    let mut cx = std::task::Context::from_waker(&waker);

    match Pin::new(&mut fut).poll(&mut cx) {
        Poll::Ready(value) => Ok(value),
        Poll::Pending => Err(FuturePendingError),
    }
}

fn noop_waker() -> Waker {
    const VTABLE: RawWakerVTable = RawWakerVTable::new(
        // Cloning just returns a new no-op raw waker
        |_| RAW,
        // `wake` does nothing
        |_| {},
        // `wake_by_ref` does nothing
        |_| {},
        // Dropping does nothing as we don't allocate anything
        |_| {},
    );
    const RAW: RawWaker = RawWaker::new(ptr::null(), &VTABLE);

    // SAFETY: This is copied from https://github.com/rust-lang/rust/pull/96875.
    unsafe { Waker::from_raw(RAW) }
}
