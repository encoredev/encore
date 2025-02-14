use std::fmt::{Display, Write};

use super::{validation, Basic, Custom, Generic, Interface, Type, Validated, WireSpec};

impl Display for Type {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let mut renderer = TypeRenderer { buf: f };
        renderer.render_type(self)
    }
}

struct TypeRenderer<B> {
    buf: B,
}

impl<B> TypeRenderer<B>
where
    B: Write,
{
    fn render_type(&mut self, typ: &Type) -> std::fmt::Result {
        match typ {
            Type::Basic(b) => self.render_basic(b),
            Type::Array(arr) => {
                self.buf.write_str("Array<")?;
                self.render_type(&arr.0)?;
                self.buf.write_char('>')
            }
            Type::Interface(iface) => self.render_iface(iface),
            Type::Union(union) => {
                for (i, typ) in union.types.iter().enumerate() {
                    if i > 0 {
                        self.buf.write_str(" | ")?;
                    }
                    self.render_type(typ)?;
                }
                Ok(())
            }
            Type::Tuple(tup) => {
                self.buf.write_char('[')?;
                for (i, typ) in tup.types.iter().enumerate() {
                    if i > 0 {
                        self.buf.write_str(", ")?;
                    }
                    self.render_type(typ)?;
                }
                self.buf.write_char(']')
            }
            Type::Literal(lit) => self.render_literal(lit),
            Type::Class(cls) => self.render_class(cls),
            Type::Enum(e) => self.render_enum(e),
            Type::Named(named) => self.render_named(named),
            Type::Optional(opt) => {
                self.render_type(&opt.0)?;
                self.buf.write_char('?')
            }
            Type::This(_) => self.buf.write_str("this"),
            Type::Generic(g) => self.render_generic(g),
            Type::Validation(v) => self.render_validation(v),
            Type::Validated(v) => self.render_validated(v),
            Type::Custom(c) => self.render_custom(c),
        }
    }

    fn render_basic(&mut self, b: &Basic) -> std::fmt::Result {
        use Basic::*;
        let s = match b {
            Any => "any",
            String => "string",
            Boolean => "boolean",
            Number => "number",
            Object => "object",
            BigInt => "bigint",
            Date => "Date",
            Symbol => "symbol",
            Undefined => "undefined",
            Null => "null",
            Void => "void",
            Unknown => "unknown",
            Never => "never",
        };
        self.buf.write_str(s)
    }

    fn render_iface(&mut self, iface: &Interface) -> std::fmt::Result {
        self.buf.write_str("interface { ")?;
        for (i, field) in iface.fields.iter().enumerate() {
            if i > 0 {
                self.buf.write_str("; ")?;
            }

            use super::FieldName;
            self.buf.write_str(match &field.name {
                FieldName::String(s) => s,
                FieldName::Symbol(_) => "symbol",
            })?;
            if field.optional {
                self.buf.write_char('?')?;
            }
            self.buf.write_str(": ")?;
            self.render_type(&field.typ)?;
        }
        self.buf.write_str(" }")
    }

    fn render_literal(&mut self, lit: &super::Literal) -> std::fmt::Result {
        use super::Literal;
        match lit {
            Literal::String(s) => self.buf.write_fmt(format_args!("{:#?}", s)),
            Literal::Boolean(b) => self.buf.write_fmt(format_args!("{}", b)),
            Literal::Number(n) => self.buf.write_fmt(format_args!("{}", n)),
            Literal::BigInt(n) => self.buf.write_str(n),
        }
    }

    fn render_class(&mut self, _cls: &super::ClassType) -> std::fmt::Result {
        self.buf.write_str("class {}")
    }

    fn render_enum(&mut self, e: &super::EnumType) -> std::fmt::Result {
        self.buf.write_str("enum { ")?;
        for (i, mem) in e.members.iter().enumerate() {
            if i > 0 {
                self.buf.write_str(", ")?;
            }
            self.buf.write_str(&mem.name)?;
        }
        self.buf.write_str(" }")
    }

    fn render_named(&mut self, named: &super::Named) -> std::fmt::Result {
        let name = named.obj.name.as_deref().unwrap_or("UnknownObject");
        self.buf.write_str(name)?;

        if !named.type_arguments.is_empty() {
            self.buf.write_char('<')?;
            for (i, arg) in named.type_arguments.iter().enumerate() {
                if i > 0 {
                    self.buf.write_str(", ")?;
                }
                self.render_type(arg)?;
            }
            self.buf.write_char('>')?;
        }
        Ok(())
    }

    fn render_validation(&mut self, v: &validation::Expr) -> std::fmt::Result {
        self.buf.write_fmt(format_args!("{}", v))
    }

    fn render_validated(&mut self, v: &Validated) -> std::fmt::Result {
        self.render_type(&v.typ)?;
        self.buf.write_fmt(format_args!(" & {}", v.expr))
    }

    fn render_custom(&mut self, c: &Custom) -> std::fmt::Result {
        match c {
            Custom::WireSpec(s) => self.render_wire_spec(s),
        }
    }

    fn render_wire_spec(&mut self, s: &WireSpec) -> std::fmt::Result {
        match &s.location {
            super::WireLocation::Query => self.buf.write_str("Query<")?,
            super::WireLocation::Header => self.buf.write_str("Header<")?,
            super::WireLocation::PubSubAttr => self.buf.write_str("Attribute<")?,
        }
        self.render_type(&s.underlying)?;
        if let Some(name) = &s.name_override {
            self.buf.write_fmt(format_args!(", {:#?}", name))?;
        }
        self.buf.write_char('>')
    }

    fn render_generic(&mut self, g: &Generic) -> std::fmt::Result {
        match g {
            Generic::TypeParam(tp) => self.buf.write_fmt(format_args!("TypeParam#{}", tp.idx)),
            Generic::Index(idx) => {
                self.render_type(&idx.source)?;
                self.buf.write_char('[')?;
                self.render_type(&idx.index)?;
                self.buf.write_char(']')
            }
            Generic::Mapped(m) => {
                self.buf.write_str("{ [P in ")?;
                self.render_type(&m.in_type)?;
                self.buf.write_str("]: ")?;
                self.render_type(&m.value_type)?;
                self.buf.write_str(" }")
            }
            Generic::MappedKeyType(_) => self.buf.write_char('P'),
            Generic::Keyof(k) => {
                self.buf.write_str("keyof ")?;
                self.render_type(&k.0)
            }
            Generic::Conditional(c) => {
                self.render_type(&c.check_type)?;
                self.buf.write_str(" extends ")?;
                self.render_type(&c.extends_type)?;
                self.buf.write_str(" ? ")?;
                self.render_type(&c.true_type)?;
                self.buf.write_str(" : ")?;
                self.render_type(&c.false_type)
            }
            Generic::Inferred(i) => self.buf.write_fmt(format_args!("Inferred#{}", i.0)),
            Generic::Intersection(i) => {
                self.render_type(&i.x)?;
                self.buf.write_str(" & ")?;
                self.render_type(&i.y)
            }
        }
    }
}
