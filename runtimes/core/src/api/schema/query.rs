use std::collections::HashMap;
use std::str::FromStr;

use crate::api::jsonschema::DecodeConfig;
use crate::api::schema::{JSONPayload, ToOutgoingRequest};
use crate::api::{self, PValue, PValues};
use crate::api::{jsonschema, APIResult};
use serde::de::IntoDeserializer;
use serde_json::{Map as JsonMap, Value as JsonValue};
use url::Url;

#[derive(Debug, Clone)]
pub struct Query {
    schema: jsonschema::JSONSchema,
    query_fields: HashMap<String, QueryFieldMeta>,
}

#[derive(Debug, Clone, Copy)]
struct QueryFieldMeta {
    expects_json: bool,
}

impl Query {
    pub fn new(schema: jsonschema::JSONSchema) -> Self {
        let query_fields = schema
            .root()
            .fields
            .iter()
            .map(|(name, field)| {
                let wire_name = field.name_override.as_deref().unwrap_or(name.as_str());
                let meta = QueryFieldMeta {
                    expects_json: field_expects_json(&schema, field),
                };
                (wire_name.to_string(), meta)
            })
            .collect();

        Self {
            schema,
            query_fields,
        }
    }

    pub fn parse_incoming_request_parts(
        &self,
        req: &axum::http::request::Parts,
    ) -> APIResult<Option<PValues>> {
        self.parse(req.uri.query())
    }

    pub fn parse(&self, query: Option<&str>) -> APIResult<Option<PValues>> {
        let mut values = JsonMap::new();
        for (key, value) in form_urlencoded::parse(query.unwrap_or_default().as_bytes()) {
            let parsed = parse_query_value(&self.query_fields, key.as_ref(), value.as_ref());
            insert_query_value(&mut values, key.into_owned(), parsed);
        }

        let de = JsonValue::Object(values).into_deserializer();
        let cfg = DecodeConfig {
            coerce_strings: true,
            arrays_as_repeated_fields: false,
        };

        let decoded = self
            .schema
            .deserialize(de, cfg)
            .map_err(|e| api::Error::invalid_argument("unable to decode query string", e))?;

        Ok(Some(decoded))
    }

    pub fn contains_name(&self, name: &str) -> bool {
        self.schema.root().contains_name(name)
    }

    pub fn contains_any(&self, query: &[u8]) -> bool {
        let schema = &self.schema.root();
        if schema.is_empty() {
            return false;
        }

        let parsed = form_urlencoded::parse(query);
        for (key, val) in parsed {
            // Only consider non-empty values to be present.
            if !val.is_empty() && schema.contains_name(key.as_ref()) {
                return true;
            }
        }

        false
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
}

impl ToOutgoingRequest<http::Request<()>> for Query {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut http::Request<()>,
    ) -> APIResult<()> {
        let Some(payload) = payload else {
            return Err(api::Error {
                code: api::ErrCode::InvalidArgument,
                message: "missing query parameters".to_string(),
                internal_message: Some("missing query parameters".to_string()),
                stack: None,
                details: None,
            });
        };

        // Serialize the payload.
        let mut url = Url::parse(&req.uri().to_string()).map_err(api::Error::internal)?;

        {
            let mut pairs = url.query_pairs_mut();
            for (key, value) in flatten_payload(payload) {
                pairs.append_pair(&key, &value);
            }
        }

        *req.uri_mut() = http::Uri::from_str(url.as_str()).map_err(api::Error::internal)?;

        Ok(())
    }
}

impl ToOutgoingRequest<reqwest::Request> for Query {
    fn to_outgoing_request(
        &self,
        payload: &mut JSONPayload,
        req: &mut reqwest::Request,
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
                details: None,
            });
        };

        // Serialize the payload.
        {
            let url = req.url_mut();
            let mut pairs = url.query_pairs_mut();
            for (key, value) in flatten_payload(payload) {
                pairs.append_pair(&key, &value);
            }
        }

        // If the query string is now empty, set it to None.
        if let Some("") = req.url().query() {
            req.url_mut().set_query(None);
        }

        Ok(())
    }
}

fn flatten_payload(payload: &PValues) -> Vec<(String, String)> {
    let mut pairs = Vec::new();
    for (key, value) in payload {
        push_query_value(&mut pairs, key, value);
    }
    pairs
}

fn push_query_value(pairs: &mut Vec<(String, String)>, key: &str, value: &PValue) {
    match value {
        PValue::Array(values) if values.iter().all(is_query_scalar) => {
            for value in values {
                pairs.push((key.to_string(), query_scalar_to_string(value)));
            }
        }
        PValue::Array(values) => {
            for value in values {
                pairs.push((key.to_string(), query_value_to_string(value)));
            }
        }
        _ => {
            pairs.push((key.to_string(), query_value_to_string(value)));
        }
    }
}

fn query_value_to_string(value: &PValue) -> String {
    if is_query_scalar(value) {
        query_scalar_to_string(value)
    } else {
        serde_json::to_string(value).unwrap_or_else(|_| "null".to_string())
    }
}

fn is_query_scalar(value: &PValue) -> bool {
    matches!(
        value,
        PValue::Null
            | PValue::Bool(_)
            | PValue::Number(_)
            | PValue::Decimal(_)
            | PValue::String(_)
            | PValue::DateTime(_)
    )
}

fn query_scalar_to_string(value: &PValue) -> String {
    match value {
        PValue::Null => "null".to_string(),
        PValue::Bool(v) => {
            if *v {
                "true".to_string()
            } else {
                "false".to_string()
            }
        }
        PValue::Number(v) => v.to_string(),
        PValue::Decimal(v) => v.to_string(),
        PValue::String(v) => v.to_string(),
        PValue::DateTime(v) => v.to_rfc3339(),
        PValue::Array(_) | PValue::Object(_) | PValue::Cookie(_) => query_value_to_string(value),
    }
}

fn insert_query_value(values: &mut JsonMap<String, JsonValue>, key: String, value: JsonValue) {
    match values.get_mut(&key) {
        Some(JsonValue::Array(existing)) => existing.push(value),
        Some(existing) => {
            let prev = existing.clone();
            *existing = JsonValue::Array(vec![prev, value]);
        }
        None => {
            values.insert(key, value);
        }
    }
}

fn parse_query_value(query_fields: &HashMap<String, QueryFieldMeta>, key: &str, value: &str) -> JsonValue {
    let Some(field) = query_fields.get(key) else {
        return JsonValue::String(value.to_string());
    };

    if field.expects_json {
        if let Ok(parsed) = serde_json::from_str::<JsonValue>(value) {
            return parsed;
        }
    }

    JsonValue::String(value.to_string())
}

fn field_expects_json(schema: &jsonschema::JSONSchema, field: &jsonschema::Field) -> bool {
    basic_or_value_expects_json(schema, &field.value)
}

fn basic_or_value_expects_json(
    schema: &jsonschema::JSONSchema,
    value: &jsonschema::BasicOrValue,
) -> bool {
    match value {
        jsonschema::BasicOrValue::Basic(_) => false,
        jsonschema::BasicOrValue::Value(idx) => {
            value_expects_json(schema, schema.resolve_value(*idx))
        }
    }
}

fn value_expects_json(schema: &jsonschema::JSONSchema, value: &jsonschema::Value) -> bool {
    match value {
        jsonschema::Value::Map(_) | jsonschema::Value::Struct(_) => true,
        jsonschema::Value::Array(value) => basic_or_value_expects_json(schema, value),
        jsonschema::Value::Option(value) => basic_or_value_expects_json(schema, value),
        jsonschema::Value::Union(values) => values
            .iter()
            .any(|value| basic_or_value_expects_json(schema, value)),
        jsonschema::Value::Ref(idx) => value_expects_json(schema, schema.resolve_value(*idx)),
        jsonschema::Value::Validation(validation) => {
            basic_or_value_expects_json(schema, &validation.bov)
        }
        jsonschema::Value::Basic(_) | jsonschema::Value::Literal(_) => false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::PValues;

    #[test]
    fn flatten_payload_repeats_scalar_arrays() {
        let mut payload = PValues::new();
        payload.insert(
            "ids".to_string(),
            PValue::Array(vec![
                PValue::String("1".to_string()),
                PValue::String("2".to_string()),
            ]),
        );

        let pairs = flatten_payload(&payload);
        assert_eq!(
            pairs,
            vec![
                ("ids".to_string(), "1".to_string()),
                ("ids".to_string(), "2".to_string())
            ]
        );
    }

    #[test]
    fn flatten_payload_encodes_objects_as_json() {
        let mut obj = PValues::new();
        obj.insert("eq".to_string(), PValue::String("abc".to_string()));

        let mut payload = PValues::new();
        payload.insert("where".to_string(), PValue::Object(obj));

        let pairs = flatten_payload(&payload);
        assert_eq!(
            pairs,
            vec![("where".to_string(), "{\"eq\":\"abc\"}".to_string())]
        );
    }

    #[test]
    fn parse_query_value_uses_cached_wire_name_metadata() {
        let query_fields = HashMap::from([(
            "wire_where".to_string(),
            QueryFieldMeta { expects_json: true },
        )]);

        let parsed = parse_query_value(&query_fields, "wire_where", r#"{"eq":"abc"}"#);
        assert_eq!(parsed, JsonValue::Object(JsonMap::from_iter([(
            "eq".to_string(),
            JsonValue::String("abc".to_string()),
        )])));
    }

    #[test]
    fn parse_query_value_treats_unknown_keys_as_strings() {
        let query_fields = HashMap::from([(
            "wire_where".to_string(),
            QueryFieldMeta { expects_json: true },
        )]);

        let parsed = parse_query_value(&query_fields, "where", r#"{"eq":"abc"}"#);
        assert_eq!(parsed, JsonValue::String(r#"{"eq":"abc"}"#.to_string()));
    }
}
