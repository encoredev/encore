use std::collections::HashMap;
use std::ops::Deref;
use std::sync::Arc;

use anyhow::{Context, Result};

use crate::api::jsonschema::de::{Basic, BasicOrValue, Field, Struct};
use crate::api::jsonschema::{JSONSchema, Registry, Value};
use crate::encore::parser::meta::v1 as meta;
use crate::encore::parser::schema::v1 as schema;
use crate::encore::parser::schema::v1::r#type::Typ;

impl Registry {
    pub fn schema(self: &Arc<Self>, id: usize) -> JSONSchema {
        JSONSchema {
            registry: self.clone(),
            root: id,
        }
    }
}

/// Builder builds a JSONSchema registry.
pub struct Builder<'a> {
    md: &'a meta::Data,

    /// Values that have been computed so far.
    /// None indicates a declaration that's being computed;
    /// it's stored as None until it's computed to be able to handle
    /// recursive references.
    values: Vec<Option<Value>>,

    /// Map of declaration ids to value indices.
    decls: HashMap<u32, usize>,
}

impl<'a> Builder<'a> {
    pub fn new(md: &'a meta::Data) -> Self {
        Self {
            md,
            values: Vec::new(),
            decls: HashMap::new(),
        }
    }

    pub fn build(self) -> Arc<Registry> {
        // Ensure all values have been computed.
        let mut values = Vec::with_capacity(self.values.len());
        for v in self.values {
            values.push(v.expect("missing value"));
        }

        Arc::new(Registry { values })
    }

    #[inline]
    pub fn get(&self, idx: usize) -> Option<&Value> {
        self.values.get(idx).and_then(|v| v.as_ref())
    }

    #[inline]
    pub fn register_value(&mut self, val: Value) -> usize {
        match val {
            // If it's already a ref, return it unmodified.
            Value::Ref(idx) => idx,
            val => self.reg(val),
        }
    }

    #[inline]
    pub fn register_type(&mut self, typ: &schema::Type) -> Result<usize> {
        let val = self.typ(typ)?;

        Ok(match val {
            // If it's a ref, return its index directly.
            Value::Ref(idx) => idx,
            val => self.reg(val),
        })
    }

    /// Computes the JSONSchema value for the given type.
    #[inline]
    fn typ<T: ToType>(&mut self, typ: T) -> Result<Value> {
        let typ = typ.tt()?;

        match typ {
            Typ::Named(named) => self.named(named),
            Typ::Builtin(builtin) => {
                let builtin = schema::Builtin::try_from(*builtin).context("invalid builtin")?;
                self.builtin(builtin)
            }

            Typ::Pointer(ptr) => self.ptr(ptr),
            Typ::Struct(st) => Ok(Value::Struct(self.struct_val(st)?)),
            Typ::Map(map) => self.map(map),
            Typ::List(list) => self.list(list),
            Typ::Union(union) => self.union(union),
            Typ::Config(_) => anyhow::bail!("config not yet supported"),
            Typ::TypeParameter(_) => anyhow::bail!("type params not yet supported"),
        }
    }

    #[inline]
    fn named(&mut self, named: &schema::Named) -> Result<Value> {
        let decl = self
            .md
            .decls
            .get(named.id as usize)
            .context("missing decl")?;
        let idx = self.decl(decl)?;
        Ok(Value::Ref(idx))
    }

    #[inline]
    fn decl(&mut self, decl: &schema::Decl) -> Result<usize> {
        // Do we have a value for this decl already?
        if let Some(idx) = self.decls.get(&decl.id) {
            return Ok(*idx);
        }

        // Allocate an index first to handle recursive references.
        let idx = self.values.len();
        self.values.push(None);
        self.decls.insert(decl.id, idx);

        // Then compute the type and update the stored value.
        let typ = self.typ(&decl.r#type)?;
        self.values[idx] = Some(typ);
        Ok(idx)
    }

    #[inline]
    fn ptr(&mut self, ptr: &schema::Pointer) -> Result<Value> {
        self.typ(&ptr.base)
    }

    #[inline]
    fn builtin(&mut self, b: schema::Builtin) -> Result<Value> {
        use schema::Builtin;
        Ok(Value::Basic(match b {
            Builtin::Any | Builtin::Json => Basic::Any,
            Builtin::Bool => Basic::Bool,
            Builtin::String | Builtin::Bytes | Builtin::Time | Builtin::Uuid | Builtin::UserId => {
                Basic::String
            }

            Builtin::Int
            | Builtin::Uint
            | Builtin::Int8
            | Builtin::Int16
            | Builtin::Int32
            | Builtin::Int64
            | Builtin::Uint8
            | Builtin::Uint16
            | Builtin::Uint32
            | Builtin::Uint64
            | Builtin::Float32
            | Builtin::Float64 => Basic::Number,
        }))
    }

    #[inline]
    pub fn struct_val(&mut self, st: &schema::Struct) -> Result<Struct> {
        Ok(Struct {
            fields: {
                let mut map = HashMap::with_capacity(st.fields.len());
                for f in &st.fields {
                    let (k, v) = self.struct_field(f)?;
                    map.insert(k.to_owned(), v);
                }
                map
            },
        })
    }

    #[inline]
    pub fn struct_field<'b>(&mut self, f: &'b schema::Field) -> Result<(&'b String, Field)> {
        // Note: Our JS/TS support don't include the ability to change
        // the JSON name from the field name, so we use the field name unconditionally.

        let typ = self.typ(&f.typ)?;
        let value = match typ {
            Value::Basic(basic) => BasicOrValue::Basic(basic),
            val => self.bov(val),
        };

        Ok((
            &f.name,
            Field {
                value,
                optional: f.optional,
                name_override: None,
            },
        ))
    }

    #[inline]
    fn map(&mut self, map: &schema::Map) -> Result<Value> {
        // Note: JSON doesn't support anything but string keys,
        // so we don't actually track the key type for the purpose
        // of JSON schemas. Ignore it here.
        let value = self.typ(map.value.tt()?)?;
        Ok(Value::Map(self.bov(value)))
    }

    #[inline]
    fn list(&mut self, list: &schema::List) -> Result<Value> {
        let value = self.typ(list.elem.tt()?)?;
        Ok(Value::Array(self.bov(value)))
    }

    #[inline]
    fn union(&mut self, union: &schema::Union) -> Result<Value> {
        let values: Result<Vec<BasicOrValue>> = union
            .types
            .iter()
            .map(|t| self.typ(t).map(|v| self.bov(v)))
            .collect();
        Ok(Value::Union(values?))
    }

    #[inline]
    fn bov(&mut self, value: Value) -> BasicOrValue {
        match value {
            Value::Basic(basic) => BasicOrValue::Basic(basic),
            val => BasicOrValue::Value(self.reg(val)),
        }
    }

    #[inline]
    fn reg(&mut self, value: Value) -> usize {
        let idx = self.values.len();
        self.values.push(Some(value));
        idx
    }
}

trait ToType {
    fn tt(&self) -> Result<&Typ>;
}

impl<T> ToType for Option<T>
where
    T: ToType,
{
    fn tt(&self) -> Result<&Typ> {
        self.as_ref().context("missing type")?.tt()
    }
}

impl<T> ToType for Box<T>
where
    T: ToType,
{
    fn tt(&self) -> Result<&Typ> {
        self.deref().tt()
    }
}

impl ToType for schema::Type {
    fn tt(&self) -> Result<&Typ> {
        self.typ.as_ref().context("missing type")
    }
}

impl ToType for Typ {
    fn tt(&self) -> Result<&Typ> {
        Ok(self)
    }
}

impl<T: ToType> ToType for &T {
    fn tt(&self) -> Result<&Typ> {
        (*self).tt()
    }
}
