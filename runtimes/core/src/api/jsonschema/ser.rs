use serde::{Serializer, Serialize, ser::SerializeMap};
use crate::api::jsonschema::{JSONSchema, Struct};
use crate::api::schema::JSONPayload;

struct SchemaSerializeWrapper<'a>
{
    schema: &'a Struct,
    payload: &'a JSONPayload,
}

impl<'a> Serialize for SchemaSerializeWrapper<'a>
{
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
        where S: Serializer
    {
        let mut map = serializer.serialize_map(None)?;
        if let Some(payload) = self.payload {
            for (key, _value) in self.schema.fields.iter() {
                if let Some(value) = payload.get(key) {
                    map.serialize_entry(key, value)?;
                }
            }
        }
        map.end()
    }
}

impl JSONSchema {
    pub fn to_json(&self, payload: &JSONPayload) -> serde_json::Result<String> {
        serde_json::to_string(&self.serialize(payload))
    }

    pub fn to_json_pretty(&self, payload: &JSONPayload) -> serde_json::Result<String> {
        serde_json::to_string_pretty(&self.serialize(payload))
    }

    pub fn to_vec(&self, payload: &JSONPayload) -> serde_json::Result<Vec<u8>> {
        serde_json::to_vec(&self.serialize(payload))
    }

    pub fn to_vec_pretty(&self, payload: &JSONPayload) -> serde_json::Result<Vec<u8>> {
        serde_json::to_vec_pretty(&self.serialize(payload))
    }

    pub fn serialize<'a>(&'a self, payload: &'a JSONPayload) -> impl Serialize + 'a {
        SchemaSerializeWrapper {
            schema: self.root(),
            payload,
        }
    }
}