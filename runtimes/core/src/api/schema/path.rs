use std::future::Future;
use std::pin::Pin;
use std::ptr;
use std::task::{Poll, RawWaker, RawWakerVTable, Waker};

use anyhow::Context;
use axum::extract::rejection::PathRejection;
use axum::extract::FromRequestParts;
use axum::http::request::Parts;
use indexmap::IndexMap;

use crate::api::jsonschema::Basic;
use crate::api::schema::JSONPayload;
use crate::api::{self, pvalue::PValue};
use crate::api::{jsonschema, APIResult};
use crate::encore::parser::meta::v1 as meta;
use crate::encore::parser::meta::v1::path_segment::ParamType;

/// The URL path to an endpoint, e.g. ("/foo/bar/:id").
#[derive(Debug, Clone)]
pub struct Path {
    /// The path segments.
    segments: Vec<Segment>,
    dynamic_segments: Vec<(Basic, Option<jsonschema::validation::Expr>)>,

    /// The capacity to use for generating requests.
    capacity: usize,
}

impl Path {
    pub fn from_meta(path: &meta::Path) -> anyhow::Result<Self> {
        let mut segments = Vec::with_capacity(path.segments.len());
        for seg in &path.segments {
            use meta::path_segment::SegmentType;
            let validation = seg
                .validation
                .as_ref()
                .map(|v| {
                    jsonschema::validation::Expr::try_from(v)
                        .context("invalid path segment validation")
                })
                .transpose()?;

            match SegmentType::try_from(seg.r#type).context("invalid path segment type")? {
                SegmentType::Literal => {
                    segments.push(Segment::Literal(seg.value.clone().into_boxed_str()))
                }
                SegmentType::Param => {
                    let name = &seg.value;
                    let typ = match ParamType::try_from(seg.value_type)
                        .context("invalid path parameter type")?
                    {
                        ParamType::String => Basic::String,
                        ParamType::Bool => Basic::Bool,
                        ParamType::Uuid => Basic::String,
                        ParamType::Int
                        | ParamType::Int8
                        | ParamType::Int16
                        | ParamType::Int32
                        | ParamType::Int64
                        | ParamType::Uint
                        | ParamType::Uint8
                        | ParamType::Uint16
                        | ParamType::Uint32
                        | ParamType::Uint64 => Basic::Number,
                    };

                    segments.push(Segment::Param {
                        name: name.clone().into_boxed_str(),
                        typ,
                        validation,
                    });
                }

                SegmentType::Wildcard => segments.push(Segment::Wildcard {
                    name: seg.clone().value.into_boxed_str(),
                    validation,
                }),
                SegmentType::Fallback => segments.push(Segment::Fallback {
                    name: seg.clone().value.into_boxed_str(),
                    validation,
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
                Param {
                    typ, validation, ..
                } => {
                    capacity += 10; // assume path parameters on average are 10 characters long
                    dynamic_segments.push((*typ, validation.clone()));
                }
                Wildcard { validation, .. } | Fallback { validation, .. } => {
                    // Assume path parameters on average are 10 characters long.
                    capacity += 10;
                    dynamic_segments.push((jsonschema::Basic::String, validation.clone()));
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
        validation: Option<jsonschema::validation::Expr>,
    },
    Wildcard {
        name: Box<str>,
        validation: Option<jsonschema::validation::Expr>,
    },
    Fallback {
        name: Box<str>,
        validation: Option<jsonschema::validation::Expr>,
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
                Literal(lit) => path.push_str(lit),
                Param { name, .. } | Wildcard { name, .. } | Fallback { name, .. } => {
                    let Some(payload) = payload else {
                        return Err(api::Error {
                            code: api::ErrCode::InvalidArgument,
                            message: "missing field in request payload".into(),
                            internal_message: Some(format!(
                                "missing field in request payload: {}",
                                &name
                            )),
                            stack: None,
                            details: None,
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
                            details: None,
                        });
                    };

                    match value {
                        PValue::String(str) => {
                            // URL-encode the string, so it doesn't get reinterpreted
                            // as multiple path segments.
                            let encoded = urlencoding::encode(str);
                            path.push_str(&encoded);
                        }
                        PValue::Null => path.push_str("null"),
                        PValue::Bool(bool) => path.push_str(if *bool { "true" } else { "false" }),
                        PValue::Number(num) => {
                            let str = num.to_string();
                            path.push_str(&str);
                        }
                        PValue::DateTime(dt) => {
                            let encoded = dt.to_rfc3339();
                            path.push_str(&encoded);
                        }
                        PValue::Array(_) | PValue::Object(_) => {
                            return Err(api::Error {
                                code: api::ErrCode::InvalidArgument,
                                message: "unsupported type in request payload".into(),
                                internal_message: Some(format!(
                                    "unsupported type in request payload for field {}",
                                    &name
                                )),
                                stack: None,
                                details: None,
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
    ) -> APIResult<Option<IndexMap<String, PValue>>> {
        if self.dynamic_segments.is_empty() {
            return Ok(None);
        }

        let fut = axum::extract::Path::<Vec<(String, String)>>::from_request_parts(req, &());

        let result = resolve_immediate_future(fut).map_err(|_| api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "unable to parse path params".into(),
            internal_message: Some("polling path params returned pending".into()),
            stack: None,
            details: None,
        })?;

        match result {
            Ok(axum::extract::Path(params)) => {
                let mut map = IndexMap::with_capacity(params.len());

                // For each param, find the corresponding segment and deserialize it.
                for (idx, (name, val)) in params.into_iter().enumerate() {
                    if let Some((typ, validation)) = self.dynamic_segments.get(idx) {
                        // Decode it into the correct type based on the type.
                        let val = match &typ {
                            // For strings and any, use the value directly.
                            Basic::String | Basic::Any => PValue::String(val),

                            // For numbers and booleans, use the JSON parser.
                            Basic::Number => {
                                let val = serde_json::from_str::<serde_json::Number>(&val)
                                    .map_err(|err| api::Error {
                                        code: api::ErrCode::InvalidArgument,
                                        message: "path parameter is not a valid number".into(),
                                        internal_message: Some(err.to_string()),
                                        stack: None,
                                        details: None,
                                    })?;
                                PValue::Number(val)
                            }
                            Basic::Bool => {
                                let val = serde_json::from_str::<bool>(&val).map_err(|err| {
                                    api::Error {
                                        code: api::ErrCode::InvalidArgument,
                                        message: "path parameter is not a valid boolean".into(),
                                        internal_message: Some(err.to_string()),
                                        stack: None,
                                        details: None,
                                    }
                                })?;
                                PValue::Bool(val)
                            }

                            Basic::DateTime => {
                                let val =
                                    api::DateTime::parse_from_rfc3339(&val).map_err(|err| {
                                        api::Error {
                                            code: api::ErrCode::InvalidArgument,
                                            message: "path parameter is not a valid datetime"
                                                .into(),
                                            internal_message: Some(err.to_string()),
                                            stack: None,
                                            details: None,
                                        }
                                    })?;
                                PValue::DateTime(val)
                            }

                            // We shouldn't have null here, but handle it just in case.
                            Basic::Null => PValue::Null,
                        };

                        // Validate the value, if we have a validation expression.
                        if let Some(validation) = validation.as_ref() {
                            if let Err(err) = validation.validate_pval(&val) {
                                return Err(api::Error {
                                    code: api::ErrCode::InvalidArgument,
                                    message: format!("invalid path parameter {}: {}", name, err),
                                    internal_message: None,
                                    stack: None,
                                    details: None,
                                });
                            }
                        }

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
                    details: None,
                },
                PathRejection::MissingPathParams(err) => api::Error {
                    code: api::ErrCode::InvalidArgument,
                    message: "missing path params".into(),
                    internal_message: Some(err.to_string()),
                    stack: None,
                    details: None,
                },
                err => api::Error {
                    code: api::ErrCode::Internal,
                    message: "unable to parse path params".into(),
                    internal_message: Some(err.to_string()),
                    stack: None,
                    details: None,
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
