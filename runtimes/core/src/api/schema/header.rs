use crate::api;
use crate::api::schema::{JSONPayload, ParseResponse, ToOutgoingRequest, ToResponse};
use crate::api::{jsonschema, APIResult};
use std::str::FromStr;

#[derive(Debug, Clone)]
pub struct Header {
    schema: jsonschema::JSONSchema,
}

impl Header {
    pub fn new(schema: jsonschema::JSONSchema) -> Self {
        Self { schema }
    }

    pub fn contains_any(&self, headers: &impl HTTPHeaders) -> bool {
        for (name, field) in self.schema.root().fields.iter() {
            let header_name = field.name_override.as_deref().unwrap_or(name.as_str());

            if let Some(val) = headers.get(header_name) {
                // Only consider non-empty values to be present.
                if !val.is_empty() {
                    return true;
                }
            }
        }
        false
    }

    pub fn contains_key(&self, key: &str) -> bool {
        self.schema.root().fields.contains_key(key)
    }

    pub fn len(&self) -> usize {
        self.schema.root().fields.len()
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    pub fn fields(&self) -> impl Iterator<Item = (&String, &jsonschema::Field)> {
        self.schema.root().fields.iter()
    }

    /// Returns an iterator that yields the header names that are expected by the schema.
    pub fn header_names(&self) -> impl Iterator<Item = axum::http::HeaderName> + '_ {
        self.schema
            .root()
            .fields
            .iter()
            .filter_map(|(name, field)| {
                let header_name = field.name_override.as_deref().unwrap_or(name.as_str());
                axum::http::HeaderName::from_str(header_name).ok()
            })
    }
}

pub trait AsStr {
    fn as_str(&self) -> &str;
}

pub trait ToHeaderStr {
    type Error: std::error::Error;

    fn to_str(&self) -> Result<&str, Self::Error>;
    fn is_empty(&self) -> bool;
}

impl AsStr for &axum::http::header::HeaderName {
    fn as_str(&self) -> &str {
        <axum::http::header::HeaderName>::as_str(self)
    }
}

impl ToHeaderStr for &axum::http::header::HeaderValue {
    type Error = axum::http::header::ToStrError;

    fn to_str(&self) -> Result<&str, Self::Error> {
        <axum::http::header::HeaderValue>::to_str(self)
    }
    fn is_empty(&self) -> bool {
        <axum::http::header::HeaderValue>::is_empty(self)
    }
}

pub trait HTTPHeaders {
    type Name: AsStr;
    type Value: ToHeaderStr;
    type Iter: Iterator<Item = (Self::Name, Self::Value)>;
    type GetAll: Iterator<Item = Self::Value>;

    fn headers(&self) -> Self::Iter;
    fn get(&self, key: &str) -> Option<Self::Value>;
    fn get_all(&self, key: &str) -> Self::GetAll;
    fn contains_key(&self, key: &str) -> bool;
}

impl<'a> HTTPHeaders for &'a axum::http::HeaderMap {
    type Name = &'a axum::http::header::HeaderName;
    type Value = &'a axum::http::header::HeaderValue;
    type Iter = axum::http::header::Iter<'a, axum::http::header::HeaderValue>;
    type GetAll = axum::http::header::ValueIter<'a, axum::http::header::HeaderValue>;

    fn headers(&self) -> Self::Iter {
        self.iter()
    }

    fn get(&self, key: &str) -> Option<Self::Value> {
        <axum::http::HeaderMap>::get(self, key)
    }

    fn get_all(&self, key: &str) -> Self::GetAll {
        <axum::http::HeaderMap>::get_all(self, key).iter()
    }

    fn contains_key(&self, key: &str) -> bool {
        <axum::http::HeaderMap>::contains_key(self, key)
    }
}

impl Header {
    pub fn parse_incoming_request_parts(
        &self,
        req: &axum::http::request::Parts,
    ) -> APIResult<Option<serde_json::Map<String, serde_json::Value>>> {
        self.parse(&req.headers)
    }

    pub fn parse(
        &self,
        headers: &axum::http::HeaderMap,
    ) -> APIResult<Option<serde_json::Map<String, serde_json::Value>>> {
        if self.schema.root().is_empty() {
            return Ok(None);
        }

        match self.schema.parse(headers) {
            Ok(decoded) => Ok(Some(decoded)),
            Err(err) => Err(err),
        }
    }
}

impl ParseResponse for Header {
    type Output = Option<serde_json::Map<String, serde_json::Value>>;

    fn parse_response(&self, resp: &mut reqwest::Response) -> APIResult<Self::Output> {
        if self.schema.root().is_empty() {
            return Ok(None);
        }

        match self.schema.parse(resp.headers()) {
            Ok(decoded) => Ok(Some(decoded)),
            Err(err) => Err(err),
        }
    }
}

impl ToOutgoingRequest<http::HeaderMap> for Header {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        headers: &mut http::HeaderMap,
    ) -> APIResult<()> {
        if self.schema.root().is_empty() {
            return Ok(());
        }

        let Some(payload) = payload else {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing query parameters".to_string(),
                internal_message: Some("missing query parameters".to_string()),
                stack: None,
            });
        };

        let schema = self.schema.root();
        for (key, value) in payload.iter() {
            let key = key.as_str();
            let header_name = schema
                .fields
                .get(key)
                .and_then(|f| f.name_override.as_deref())
                .unwrap_or(key);
            let header_name =
                reqwest::header::HeaderName::from_str(header_name).map_err(api::Error::internal)?;
            match to_reqwest_header_value(value)? {
                ReqwestHeaders::Single(value) => {
                    headers.append(header_name, value);
                }
                ReqwestHeaders::Multi(values) => {
                    for value in values {
                        headers.append(header_name.clone(), value);
                    }
                }
            }
        }

        Ok(())
    }
}

impl ToOutgoingRequest<reqwest::Request> for Header {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut reqwest::Request,
    ) -> APIResult<()> {
        let headers = req.headers_mut();
        self.to_outgoing_request(payload, headers)
    }
}

impl<B> ToOutgoingRequest<http::Request<B>> for Header {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut http::Request<B>,
    ) -> APIResult<()> {
        let headers = req.headers_mut();
        self.to_outgoing_request(payload, headers)
    }
}

impl ToResponse for Header {
    fn to_response(
        &self,
        payload: &JSONPayload,
        mut resp: axum::http::response::Builder,
    ) -> APIResult<axum::http::response::Builder> {
        if self.schema.root().is_empty() {
            return Ok(resp);
        }

        let Some(payload) = payload else {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing query parameters".to_string(),
                internal_message: Some("missing query parameters".to_string()),
                stack: None,
            });
        };

        let schema = self.schema.root();
        for (key, value) in payload.iter() {
            let key = key.as_str();
            let Some(field) = schema.fields.get(key) else {
                continue; // Not a header.
            };
            let header_name = field.name_override.as_deref().unwrap_or(key);
            let header_name = axum::http::header::HeaderName::from_str(header_name)
                .map_err(api::Error::internal)?;

            match to_axum_header_value(value)? {
                AxumHeaders::Single(value) => resp = resp.header(header_name, value),
                AxumHeaders::Multi(values) => {
                    for value in values {
                        resp = resp.header(header_name.clone(), value);
                    }
                }
            }
        }

        Ok(resp)
    }
}

enum ReqwestHeaders {
    Single(reqwest::header::HeaderValue),
    Multi(Vec<reqwest::header::HeaderValue>),
}

fn to_reqwest_header_value(value: &serde_json::Value) -> APIResult<ReqwestHeaders> {
    use serde_json::Value::*;
    use ReqwestHeaders::*;

    Ok(Single(match value {
        Null => reqwest::header::HeaderValue::from_static("null"),

        Bool(bool) => {
            reqwest::header::HeaderValue::from_static(if *bool { "true" } else { "false" })
        }

        String(str) => reqwest::header::HeaderValue::from_str(str).map_err(|e| api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "unable to convert string to header value".to_string(),
            internal_message: Some(format!("unable to convert string to header value: {}", e)),
            stack: None,
        })?,

        Number(num) => {
            let str = num.to_string();
            reqwest::header::HeaderValue::from_str(&str).map_err(|e| api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "unable to convert number to header value".to_string(),
                internal_message: Some(format!("unable to convert number to header value: {}", e)),
                stack: None,
            })?
        }

        Array(arr) => {
            let mut values = Vec::with_capacity(arr.len());
            for value in arr.iter() {
                match to_reqwest_header_value(value)? {
                    Single(value) => values.push(value),
                    Multi(_) => {
                        return Err(api::Error {
                            code: api::ErrCode::InvalidArgument,
                            message: "nested array type unsupported as header value".into(),
                            internal_message: None,
                            stack: None,
                        })
                    }
                }
            }
            return Ok(Multi(values));
        }

        Object(_) => {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "map type unsupported as header value".into(),
                internal_message: None,
                stack: None,
            })
        }
    }))
}

enum AxumHeaders {
    Single(axum::http::HeaderValue),
    Multi(Vec<axum::http::HeaderValue>),
}

fn to_axum_header_value(value: &serde_json::Value) -> APIResult<AxumHeaders> {
    use serde_json::Value::*;

    Ok(AxumHeaders::Single(match value {
        Null => axum::http::HeaderValue::from_static("null"),

        Bool(bool) => axum::http::HeaderValue::from_static(if *bool { "true" } else { "false" }),

        String(str) => axum::http::HeaderValue::from_str(str).map_err(|e| api::Error {
            code: api::ErrCode::InvalidArgument,
            message: "unable to convert string to header value".to_string(),
            internal_message: Some(format!("unable to convert string to header value: {}", e)),
            stack: None,
        })?,

        Number(num) => {
            let str = num.to_string();
            axum::http::HeaderValue::from_str(&str).map_err(|e| api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "unable to convert number to header value".to_string(),
                internal_message: Some(format!("unable to convert number to header value: {}", e)),
                stack: None,
            })?
        }

        Array(arr) => {
            let mut values = Vec::with_capacity(arr.len());
            for value in arr.iter() {
                match to_axum_header_value(value)? {
                    AxumHeaders::Single(value) => values.push(value),
                    AxumHeaders::Multi(_) => {
                        return Err(api::Error {
                            code: api::ErrCode::InvalidArgument,
                            message: "nested array type unsupported as header value".into(),
                            internal_message: None,
                            stack: None,
                        })
                    }
                }
            }
            return Ok(AxumHeaders::Multi(values));
        }

        Object(_) => {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "map type unsupported as header value".into(),
                internal_message: None,
                stack: None,
            })
        }
    }))
}
