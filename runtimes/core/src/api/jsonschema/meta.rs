use std::collections::HashMap;
use std::ops::Deref;
use std::sync::Arc;

use anyhow::{Context, Result};

use crate::api::jsonschema::de::{Basic, BasicOrValue, Field, Literal, Struct};
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

struct BuilderCtx<'a, 'b: 'a> {
    builder: &'a mut Builder<'b>,

    /// The ids of computed type arguments, for the current declaration being processed.
    type_args: &'a [usize],
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
            val => {
                let mut ctx = BuilderCtx {
                    builder: self,
                    type_args: &[],
                };
                ctx.reg(val)
            }
        }
    }

    #[inline]
    pub fn register_type(&mut self, typ: &schema::Type) -> Result<usize> {
        let mut ctx = BuilderCtx {
            builder: self,
            type_args: &[],
        };
        let val = ctx.typ(typ)?;

        Ok(match val {
            // If it's a ref, return its index directly.
            Value::Ref(idx) => idx,
            val => ctx.reg(val),
        })
    }

    pub fn struct_field<'b>(&mut self, f: &'b schema::Field) -> Result<(&'b String, Field)> {
        // This should be safe to do because it's only called for schema types,
        // and schema types don't include any type arguments, so we shouldn't need to worry
        // about missing type arguments.
        let ctx = &mut BuilderCtx {
            builder: self,
            type_args: &[],
        };
        ctx.struct_field(f)
    }
}

impl<'a, 'b> BuilderCtx<'a, 'b> {
    /// Computes the JSONSchema value for the given type.
    #[inline]
    fn typ<T: ToType>(&mut self, typ: T) -> Result<Value> {
        let typ = typ.tt()?;

        match typ {
            Typ::Named(named) => self.named(named),
            Typ::Builtin(builtin) => {
                let builtin = schema::Builtin::try_from(*builtin).context("invalid builtin")?;
                Ok(self.builtin(builtin))
            }

            Typ::Pointer(ptr) => self.ptr(ptr),
            Typ::Struct(st) => Ok(Value::Struct(self.struct_val(st)?)),
            Typ::Map(map) => self.map(map),
            Typ::List(list) => self.list(list),
            Typ::Union(union) => self.union(union),
            Typ::Literal(lit) => self.literal(lit),
            Typ::Config(_) => anyhow::bail!("config not yet supported"),
            Typ::TypeParameter(param) => {
                let idx = self
                    .type_args
                    .get(param.param_idx as usize)
                    .ok_or_else(|| anyhow::anyhow!("missing type argument"))?;
                Ok(Value::Ref(*idx))
            }
        }
    }

    #[inline]
    fn named(&mut self, named: &schema::Named) -> Result<Value> {
        let decl = self
            .builder
            .md
            .decls
            .get(named.id as usize)
            .context("missing decl")?;

        // Compute indices for the type arguments.
        let type_args: Result<Vec<usize>> = named
            .type_arguments
            .iter()
            .map(|t| self.typ(t).map(|v| self.reg(v)))
            .collect();
        let type_args = type_args?;

        // Create a nested context that includes the type arguments.
        let mut nested = BuilderCtx {
            builder: self.builder,
            type_args: &type_args,
        };

        let idx = nested.decl(decl)?;
        Ok(Value::Ref(idx))
    }

    #[inline]
    fn decl(&mut self, decl: &schema::Decl) -> Result<usize> {
        // Do we have a value for this decl already?
        if let Some(idx) = self.builder.decls.get(&decl.id) {
            return Ok(*idx);
        }

        // Allocate an index first to handle recursive references.
        let idx = self.builder.values.len();
        self.builder.values.push(None);
        self.builder.decls.insert(decl.id, idx);

        // Then compute the type and update the stored value.
        let typ = self.typ(&decl.r#type)?;
        self.builder.values[idx] = Some(typ);
        Ok(idx)
    }

    #[inline]
    fn ptr(&mut self, ptr: &schema::Pointer) -> Result<Value> {
        self.typ(&ptr.base)
    }

    #[inline]
    fn builtin(&mut self, b: schema::Builtin) -> Value {
        use schema::Builtin;
        Value::Basic(match b {
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
        })
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
    fn struct_field<'c>(&mut self, f: &'c schema::Field) -> Result<(&'c String, Field)> {
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
    fn literal(&mut self, literal: &schema::Literal) -> Result<Value> {
        Ok(match literal.value.clone() {
            Some(schema::literal::Value::Str(val)) => Value::Literal(Literal::Str(val)),
            Some(schema::literal::Value::Boolean(val)) => Value::Literal(Literal::Bool(val)),
            Some(schema::literal::Value::Int(val)) => Value::Literal(Literal::Int(val)),
            Some(schema::literal::Value::Float(val)) => Value::Literal(Literal::Float(val)),
            Some(schema::literal::Value::Null(_)) => Value::Basic(Basic::Null),
            None => anyhow::bail!("missing literal value"),
        })
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
        let idx = self.builder.values.len();
        self.builder.values.push(Some(value));
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
