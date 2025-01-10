use std::borrow::Cow;
use std::collections::HashMap;
use std::path::Path;

use anyhow::Result;
use itertools::Itertools;
use litparser::{ParseResult, ToParseErr};
use swc_common::errors::HANDLER;

use crate::encore::parser::schema::v1::r#type as styp;
use crate::encore::parser::schema::v1::{self as schema};
use crate::legacymeta::api_schema::strip_path_params;
use crate::parser::parser::ParseContext;

use crate::parser::resources::apis::api::Endpoint;
use crate::parser::resources::apis::encoding::resolve_wire_spec;
use crate::parser::types::{
    drop_empty_or_void, unwrap_validated, Basic, Custom, EnumValue, FieldName, Generic, Interface,
    Literal, Named, ObjectId, Type, Union, WireLocation,
};
use crate::parser::{FilePath, FileSet, Range};

pub(super) struct SchemaBuilder<'a> {
    pc: &'a ParseContext,
    app_root: &'a Path,

    decls: Vec<schema::Decl>,
    obj_to_decl: HashMap<ObjectId, u32>,
}

struct BuilderCtx<'a, 'b> {
    builder: &'a mut SchemaBuilder<'b>,

    // The id of the current decl being built.
    // Used for generating TypeParameterRefs.
    decl_id: Option<u32>,
}

impl<'a> SchemaBuilder<'a> {
    pub(super) fn new(pc: &'a ParseContext, app_root: &'a Path) -> Self {
        SchemaBuilder {
            pc,
            app_root,
            decls: Vec::new(),
            obj_to_decl: HashMap::new(),
        }
    }

    pub(super) fn into_decls(self) -> Vec<schema::Decl> {
        self.decls
    }

    pub(super) fn typ(&mut self, typ: &Type) -> Result<schema::Type> {
        let mut ctx = BuilderCtx {
            builder: self,
            decl_id: None,
        };
        ctx.typ(typ)
    }

    pub fn transform_handshake(&mut self, ep: &Endpoint) -> ParseResult<Option<schema::Type>> {
        let mut ctx = BuilderCtx {
            builder: self,
            decl_id: None,
        };
        ctx.transform_handshake(ep)
    }
    pub fn transform_request(&mut self, ep: &Endpoint) -> ParseResult<Option<schema::Type>> {
        let mut ctx = BuilderCtx {
            builder: self,
            decl_id: None,
        };
        ctx.transform_request(ep)
    }

    pub fn transform_response(&mut self, typ: Option<Type>) -> Result<Option<schema::Type>> {
        match typ {
            Some(typ) => Ok(Some(self.typ(&typ)?)),
            None => Ok(None),
        }
    }
}

impl BuilderCtx<'_, '_> {
    #[tracing::instrument(skip(self), ret, level = "trace")]
    fn typ(&mut self, typ: &Type) -> Result<schema::Type> {
        Ok(match typ {
            Type::Basic(tt) => self.basic(tt),
            Type::Array(tt) => {
                let elem = self.typ(&tt.0)?;
                schema::Type {
                    typ: Some(styp::Typ::List(Box::new(schema::List {
                        elem: Some(Box::new(elem)),
                    }))),
                    validation: None,
                }
            }
            Type::Interface(tt) => self.interface(tt)?,

            Type::Enum(tt) => schema::Type {
                // Treat this as a union.
                typ: Some(styp::Typ::Union(schema::Union {
                    types: tt
                        .members
                        .iter()
                        .cloned()
                        .map(|m| schema::Type {
                            typ: Some(styp::Typ::Literal(schema::Literal {
                                value: Some(match m.value {
                                    EnumValue::String(str) => schema::literal::Value::Str(str),
                                    EnumValue::Number(n) => schema::literal::Value::Int(n),
                                }),
                            })),
                            validation: None,
                        })
                        .collect(),
                })),
                validation: None,
            },

            Type::Union(union) => schema::Type {
                typ: Some(styp::Typ::Union(schema::Union {
                    types: self.types(&union.types)?,
                })),
                validation: None,
            },
            Type::Tuple(_) => anyhow::bail!("tuple types are not yet supported in schemas"),
            Type::Literal(tt) => schema::Type {
                typ: Some(styp::Typ::Literal(self.literal(tt))),
                validation: None,
            },
            Type::Class(_) => anyhow::bail!("class types are not yet supported in schemas"),
            Type::Named(tt) => {
                let state = self.builder.pc.type_checker.state();
                if state.is_universe(tt.obj.module_id) {
                    let underlying = tt.underlying(state);
                    self.typ(&underlying)?
                } else if !tt.type_arguments.is_empty() {
                    tracing::trace!(
                        "got named type with type arguments, resolving to underlying type"
                    );
                    // The type is a generic type.
                    // To avoid having to reproduce the full generic type resolution,
                    // concretize the type here.
                    let underlying = tt.underlying(state);
                    tracing::trace!(underlying = ?underlying, "underlying type");
                    self.typ(&underlying)?
                } else {
                    schema::Type {
                        typ: Some(styp::Typ::Named(self.named(tt)?)),
                        validation: None,
                    }
                }
            }
            Type::Optional(_) => anyhow::bail!("optional types are not yet supported in schemas"),
            Type::This(_) => anyhow::bail!("this types are not yet supported in schemas"),
            Type::Generic(typ) => match typ {
                Generic::TypeParam(param) => {
                    let decl_id = self
                        .decl_id
                        .ok_or_else(|| anyhow::anyhow!("missing decl_id"))?;
                    schema::Type {
                        typ: Some(styp::Typ::TypeParameter(schema::TypeParameterRef {
                            decl_id,
                            param_idx: param.idx as u32,
                        })),
                        validation: None,
                    }
                }

                typ => {
                    anyhow::bail!(
                        "unresolved generic types are not supported in schemas, got: {:#?}",
                        typ
                    )
                }
            },

            Type::Validation(expr) => {
                anyhow::bail!(
                    "unresolved standalone validation expression not supported in api schema: {:#?}",
                    expr
                )
            }

            Type::Validated(validated) => {
                let mut typ = self.typ(&validated.typ)?;
                // Simplify the validation expression, if possible.
                let expr = validated.expr.clone().simplify();
                typ.validation = Some(expr.to_pb());
                typ
            }

            Type::Custom(Custom::WireSpec(spec)) => self.typ(&spec.underlying)?,
        })
    }

    fn basic(&self, typ: &Basic) -> schema::Type {
        let b = |b: schema::Builtin| schema::Type {
            typ: Some(styp::Typ::Builtin(b as i32)),
            validation: None,
        };
        match typ {
            Basic::Any | Basic::Unknown => b(schema::Builtin::Any),
            Basic::String => b(schema::Builtin::String),
            Basic::Boolean => b(schema::Builtin::Bool),
            Basic::Date => b(schema::Builtin::Time),
            Basic::Number => {
                // TODO handle float/int distinction somehow
                b(schema::Builtin::Float64)
            }
            Basic::Null => schema::Type {
                typ: Some(styp::Typ::Literal(schema::Literal {
                    value: Some(schema::literal::Value::Null(true)),
                })),
                validation: None,
            },

            Basic::Void
            | Basic::Object
            | Basic::BigInt
            | Basic::Symbol
            | Basic::Undefined
            | Basic::Never => {
                HANDLER.with(|h| h.err(&format!("unsupported basic type in schema: {:?}", typ)));
                b(schema::Builtin::Any)
            }
        }
    }

    fn literal(&self, typ: &Literal) -> schema::Literal {
        use schema::literal::Value;
        let val = match typ.clone() {
            Literal::String(val) => Value::Str(val),
            Literal::Boolean(bool) => Value::Boolean(bool),
            Literal::Number(float) => {
                // If this can be represented as an int64, do that.
                let int = float as i64;
                if float == (int as f64) {
                    Value::Int(int)
                } else {
                    Value::Float(float)
                }
            }
            Literal::BigInt(str) => Value::Str(str),
        };
        schema::Literal { value: Some(val) }
    }

    fn interface(&mut self, typ: &Interface) -> Result<schema::Type> {
        // Is this an index signature?
        if let Some((key, value)) = typ.index.as_ref() {
            if !typ.fields.is_empty() {
                anyhow::bail!("index signature with additional fields is not supported");
            }
            return Ok(schema::Type {
                typ: Some(styp::Typ::Map(Box::new(schema::Map {
                    key: Some(Box::new(self.typ(key)?)),
                    value: Some(Box::new(self.typ(value)?)),
                }))),
                validation: None,
            });
        }

        let mut fields = Vec::with_capacity(typ.fields.len());
        for f in &typ.fields {
            let FieldName::String(field_name) = &f.name else {
                continue;
            };
            let (tt, had_undefined) = drop_undefined_union(&f.typ);
            let optional = f.optional || had_undefined;

            let mut tags = vec![];

            // Tag it as `encore:"optional"` if the field is optional.
            if optional {
                tags.push(schema::Tag {
                    key: "encore".into(),
                    name: "optional".into(),
                    options: vec![],
                });
            }

            let mut query_string_name = String::new();

            // Resolve any wire spec overrides.
            let (tt, validation_expr) = unwrap_validated(&tt);
            let (mut typ, wire) = if let Some(spec) = resolve_wire_spec(tt) {
                (
                    self.typ(&spec.underlying)?,
                    match &spec.location {
                        WireLocation::Header => {
                            let name = spec.name_override.clone().unwrap_or(field_name.clone());
                            tags.push(schema::Tag {
                                key: "header".into(),
                                name,
                                options: if f.optional {
                                    vec!["optional".into()]
                                } else {
                                    vec![]
                                },
                            });

                            Some(schema::WireSpec {
                                location: Some(schema::wire_spec::Location::Header(
                                    schema::wire_spec::Header {
                                        name: spec.name_override.clone(),
                                    },
                                )),
                            })
                        }
                        WireLocation::Query => {
                            query_string_name =
                                spec.name_override.clone().unwrap_or(field_name.clone());
                            tags.push(schema::Tag {
                                key: "query".into(),
                                name: query_string_name.clone(),
                                options: if f.optional {
                                    vec!["optional".into()]
                                } else {
                                    vec![]
                                },
                            });

                            Some(schema::WireSpec {
                                location: Some(schema::wire_spec::Location::Query(
                                    schema::wire_spec::Query {
                                        name: spec.name_override.clone(),
                                    },
                                )),
                            })
                        }

                        WireLocation::PubSubAttr => {
                            let name = spec.name_override.clone().unwrap_or(field_name.clone());
                            tags.push(schema::Tag {
                                key: "pubsub-attr".into(),
                                name,
                                options: vec![],
                            });

                            None
                        }
                    },
                )
            } else {
                (self.typ(tt)?, None)
            };

            // Propagate the validation expression to the field.
            if let Some(expr) = validation_expr {
                typ.validation = Some(expr.to_pb());
            }

            let raw_tag = tags
                .iter()
                .map(|tag| {
                    let mut s = tag.key.clone();
                    s.push(':');
                    s.push('"');
                    s.push_str(&tag.name);
                    for opt in &tag.options {
                        s.push(',');
                        s.push_str(opt);
                    }
                    s.push('"');
                    s
                })
                .join(" ");

            let doc = self
                .builder
                .pc
                .loader
                .module_containing_pos(f.range.start)
                .and_then(|module| module.preceding_comments(f.range.start));
            fields.push(schema::Field {
                typ: Some(typ),
                name: field_name.clone(),
                json_name: field_name.clone(),
                optional,
                wire,
                tags,
                raw_tag,
                query_string_name,
                doc: doc.unwrap_or_else(|| "".into()),
            });
        }

        Ok(schema::Type {
            typ: Some(styp::Typ::Struct(schema::Struct { fields })),
            validation: None,
        })
    }

    fn named(&mut self, typ: &Named) -> Result<schema::Named> {
        let type_arguments = self.types(&typ.type_arguments)?;
        let obj = &typ.obj;
        if let Some(decl_id) = self.builder.obj_to_decl.get(&obj.id) {
            return Ok(schema::Named {
                id: *decl_id,
                type_arguments,
            });
        }

        // Allocate a new decl.
        let id = self.builder.decls.len() as u32;
        let Some(name) = typ.obj.name.as_ref() else {
            anyhow::bail!("missing name for named object");
        };

        // Allocate the object and add it to the list without the underlying type.
        // We'll add the underlying type afterwards to properly handle recursive types.
        let loc = loc_from_range(self.builder.app_root, &self.builder.pc.file_set, obj.range)?;

        let decl = schema::Decl {
            id,
            name: name.clone(),
            r#type: None,        // computed below
            type_params: vec![], // TODO
            doc: "".into(),      // TODO
            loc: Some(loc),
        };
        self.builder.decls.push(decl);
        self.builder.obj_to_decl.insert(obj.id, id);

        let obj_typ = self.builder.pc.type_checker.resolve_obj_type(obj);
        let obj_typ = self
            .builder
            .pc
            .type_checker
            .concrete(obj.module_id, &obj_typ);

        let mut nested = BuilderCtx {
            builder: self.builder,
            decl_id: Some(id),
        };

        let schema_typ = nested.typ(&obj_typ)?;
        self.builder.decls.get_mut(id as usize).unwrap().r#type = Some(schema_typ);

        Ok(schema::Named { id, type_arguments })
    }

    fn new_named_from_type(
        &mut self,
        name: String,
        underlying: Type,
        range: Range,
        type_arguments: Vec<Type>,
    ) -> Result<schema::Named> {
        let type_arguments = self.types(&type_arguments)?;
        let underlying = self.typ(&underlying)?;

        // Allocate a new decl.
        let id = self.builder.decls.len() as u32;
        // Allocate the object and add it to the list without the underlying type.
        // We'll add the underlying type afterwards to properly handle recursive types.
        let loc = loc_from_range(self.builder.app_root, &self.builder.pc.file_set, range)?;
        let decl = schema::Decl {
            id,
            name: name.clone(),
            r#type: Some(underlying),
            type_params: vec![], // TODO
            doc: "".into(),      // TODO
            loc: Some(loc),
        };
        self.builder.decls.push(decl);
        Ok(schema::Named { id, type_arguments })
    }

    fn types(&mut self, types: &[Type]) -> Result<Vec<schema::Type>> {
        let mut result = Vec::with_capacity(types.len());
        for t in types {
            result.push(self.typ(t)?);
        }
        Ok(result)
    }

    fn transform_handshake(&mut self, ep: &Endpoint) -> ParseResult<Option<schema::Type>> {
        let schema = ep.encoding.raw_handshake_schema.as_ref().map(|s| s.get());
        self.transform_request_type(ep, schema).map_err(|err| {
            let sp = ep
                .encoding
                .raw_handshake_schema
                .as_ref()
                .map_or(ep.range.to_span(), |s| s.span());
            sp.parse_err(err.to_string())
        })
    }
    fn transform_request(&mut self, ep: &Endpoint) -> ParseResult<Option<schema::Type>> {
        let schema = ep.encoding.raw_req_schema.as_ref().map(|s| s.get());
        self.transform_request_type(ep, schema).map_err(|err| {
            let sp = ep
                .encoding
                .raw_req_schema
                .as_ref()
                .map_or(ep.range.to_span(), |s| s.span());
            sp.parse_err(err.to_string())
        })
    }

    fn transform_request_type(
        &mut self,
        ep: &Endpoint,
        raw_schema: Option<&Type>,
    ) -> Result<Option<schema::Type>> {
        let Some(typ) = raw_schema.cloned() else {
            return Ok(None);
        };

        let rs = self.builder.pc.type_checker.state();
        Ok(match typ {
            Type::Interface(mut interface) => {
                strip_path_params(&ep.encoding.path, &mut interface);
                let Some(typ) = drop_empty_or_void(Type::Interface(interface)) else {
                    return Ok(None);
                };
                Some(self.typ(&typ)?)
            }
            Type::Named(ref named) => {
                let underlying = named.underlying(rs).clone();
                if let Type::Interface(mut iface) = underlying {
                    strip_path_params(&ep.encoding.path, &mut iface);
                    let obj = &named.obj;
                    let Some(underlying) = drop_empty_or_void(Type::Interface(iface)) else {
                        return Ok(None);
                    };

                    let named = self.new_named_from_type(
                        obj.name.clone().unwrap(),
                        underlying,
                        obj.range,
                        named.type_arguments.clone(),
                    )?;

                    return Ok(Some(schema::Type {
                        typ: Some(styp::Typ::Named(named)),
                        validation: None,
                    }));
                } else {
                    match drop_empty_or_void(typ) {
                        Some(typ) => Some(self.typ(&typ)?),
                        None => None,
                    }
                }
            }
            _ => match drop_empty_or_void(typ) {
                Some(typ) => Some(self.typ(&typ)?),
                None => None,
            },
        })
    }
}

/// If typ is a union type containing, drop the undefined type and return the modified
/// union and `true` to indicate the type included "| undefined".
/// Otherwise, returns the original type and `false`.
fn drop_undefined_union(typ: &Type) -> (Cow<'_, Type>, bool) {
    if let Type::Union(union) = &typ {
        for (i, t) in union.types.iter().enumerate() {
            if let Type::Basic(Basic::Undefined) = &t {
                // If we have a union with only two types, return the other type.
                return if union.types.len() == 2 {
                    (Cow::Borrowed(&union.types[1 - i]), true)
                } else {
                    let mut types = union.types.clone();
                    types.swap_remove(i);
                    (Cow::Owned(Type::Union(Union { types })), true)
                };
            }
        }
    }

    (Cow::Borrowed(typ), false)
}

pub(super) fn loc_from_range(
    app_root: &Path,
    fset: &FileSet,
    range: Range,
) -> ParseResult<schema::Loc> {
    let loc = range.loc(fset)?;
    let (pkg_path, pkg_name, filename) = match loc.file {
        FilePath::Custom(ref str) => {
            return Err(range.parse_err(format!("unsupported file path in schema: {}", str)));
        }
        FilePath::Real(buf) => match buf.strip_prefix(app_root) {
            Ok(rel_path) => {
                let file_name = rel_path
                    .file_name()
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(range.parse_err("missing file name"))?;
                let pkg_name = rel_path
                    .parent()
                    .and_then(|p| p.file_name())
                    .or_else(|| app_root.file_name())
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(range.parse_err("missing package name"))?;
                let pkg_path = rel_path
                    .parent()
                    .map_or(".".to_string(), |s| s.to_string_lossy().to_string());
                (pkg_path, pkg_name, file_name)
            }
            Err(_) => {
                // The file is not relative to the app root.
                // Use a simplified path.
                let file_name = buf
                    .file_name()
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(range.parse_err(format!("missing file name: {}", buf.display())))?;
                let pkg_name = buf
                    .parent()
                    .and_then(|p| p.file_name())
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(
                        range.parse_err(format!("missing package name for {}", buf.display())),
                    )?;
                let pkg_path = format!("unknown/{}", pkg_name);
                (pkg_path, pkg_name, file_name)
            }
        },
    };

    Ok(schema::Loc {
        pkg_path,
        pkg_name,
        filename,
        start_pos: loc.start_pos as i32,
        end_pos: loc.end_pos as i32,
        src_line_start: loc.src_line_start as i32,
        src_line_end: loc.src_line_end as i32,
        src_col_start: loc.src_col_start as i32,
        src_col_end: loc.src_col_end as i32,
    })
}
