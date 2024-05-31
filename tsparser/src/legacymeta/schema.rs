use std::cell::RefCell;
use std::collections::HashMap;
use std::path::Path;

use anyhow::Result;
use itertools::Itertools;

use crate::encore::parser::schema::v1 as schema;
use crate::encore::parser::schema::v1::r#type as styp;
use crate::legacymeta::api_schema::strip_path_params;
use crate::parser::parser::ParseContext;

use crate::parser::types::custom::{resolve_custom_type_named, CustomType};
use crate::parser::types::{
    drop_empty_or_void, Basic, Generic, Interface, Literal, Named, ObjectId, Type,
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

    pub fn transform_request(&mut self, typ: Option<Type>) -> Result<Option<schema::Type>> {
        let mut ctx = BuilderCtx {
            builder: self,
            decl_id: None,
        };
        ctx.transform_request(typ)
    }

    pub fn transform_response(&mut self, typ: Option<Type>) -> Result<Option<schema::Type>> {
        match typ {
            Some(typ) => Ok(Some(self.typ(&typ)?)),
            None => Ok(None),
        }
    }
}

impl<'a, 'b> BuilderCtx<'a, 'b> {
    #[tracing::instrument(skip(self), ret, level = "trace")]
    fn typ(&mut self, typ: &Type) -> Result<schema::Type> {
        Ok(match typ {
            Type::Basic(tt) => schema::Type {
                typ: Some(styp::Typ::Builtin(self.basic(tt)? as i32)),
            },
            Type::Array(tt) => {
                let elem = self.typ(&*tt)?;
                schema::Type {
                    typ: Some(styp::Typ::List(Box::new(schema::List {
                        elem: Some(Box::new(elem)),
                    }))),
                }
            }
            Type::Interface(tt) => self.interface(tt)?,

            Type::Union(types) => schema::Type {
                typ: Some(styp::Typ::Union(schema::Union {
                    types: self.types(types)?,
                })),
            },
            Type::Tuple(_) => anyhow::bail!("tuple types are not yet supported in schemas"),
            Type::Literal(tt) => schema::Type {
                typ: Some(styp::Typ::Literal(self.literal(tt))),
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
                    }
                }
            }
            Type::Optional(_) => anyhow::bail!("optional types are not yet supported in schemas"),
            Type::This => anyhow::bail!("this types are not yet supported in schemas"),
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
                    }
                }

                typ => {
                    anyhow::bail!(
                        "unresolved generic types are not supported in schemas, got: {:#?}",
                        typ
                    )
                }
            },
        })
    }

    fn basic(&self, typ: &Basic) -> Result<schema::Builtin> {
        Ok(match typ {
            Basic::Any => schema::Builtin::Any,
            Basic::String => schema::Builtin::String,
            Basic::Boolean => schema::Builtin::Bool,
            Basic::Number => {
                // TODO handle float/int distinction somehow
                schema::Builtin::Float64
            }
            Basic::Void => anyhow::bail!("TODO void"),
            Basic::Object => anyhow::bail!("TODO object"),
            Basic::BigInt => anyhow::bail!("TODO bigint"),
            Basic::Symbol => anyhow::bail!("TODO Symbol"),
            Basic::Undefined => anyhow::bail!("TODO Undefined"),
            Basic::Null => anyhow::bail!("TODO Null"),
            Basic::Unknown => anyhow::bail!("TODO Unknown"),
            Basic::Never => anyhow::bail!("TODO Never"),
        })
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
        let ctx = self.builder.pc.type_checker.state();

        // Is this an index signature?
        if let Some((key, value)) = typ.index.as_ref() {
            if typ.fields.len() > 0 {
                anyhow::bail!("index signature with additional fields is not supported");
            }
            return Ok(schema::Type {
                typ: Some(styp::Typ::Map(Box::new(schema::Map {
                    key: Some(Box::new(self.typ(key)?)),
                    value: Some(Box::new(self.typ(value)?)),
                }))),
            });
        }

        let mut fields = Vec::with_capacity(typ.fields.len());
        for f in &typ.fields {
            let custom: Option<CustomType> = if let Type::Named(named) = &f.typ {
                resolve_custom_type_named(ctx, named)?
            } else {
                None
            };

            let (typ, wire) = match &custom {
                None => (self.typ(&f.typ)?, None),
                Some(CustomType::Header { typ, name }) => (
                    self.typ(typ)?,
                    Some(schema::WireSpec {
                        location: Some(schema::wire_spec::Location::Header(
                            schema::wire_spec::Header { name: name.clone() },
                        )),
                    }),
                ),
                Some(CustomType::Query { typ, name }) => (
                    self.typ(typ)?,
                    Some(schema::WireSpec {
                        location: Some(schema::wire_spec::Location::Query(
                            schema::wire_spec::Query { name: name.clone() },
                        )),
                    }),
                ),
            };

            let mut tags = vec![];

            // Tag it as `encore:"optional"` if the field is optional.
            if f.optional {
                tags.push(schema::Tag {
                    key: "encore".into(),
                    name: "optional".into(),
                    options: vec![],
                });
            }

            let mut query_string_name = String::new();

            match custom {
                None => {}
                Some(CustomType::Header { name, .. }) => tags.push(schema::Tag {
                    key: "header".into(),
                    name: name.unwrap_or(f.name.clone()),
                    options: if f.optional {
                        vec!["optional".into()]
                    } else {
                        vec![]
                    },
                }),
                Some(CustomType::Query { name, .. }) => {
                    query_string_name = name.unwrap_or(f.name.clone());
                    tags.push(schema::Tag {
                        key: "query".into(),
                        name: query_string_name.clone(),
                        options: if f.optional {
                            vec!["optional".into()]
                        } else {
                            vec![]
                        },
                    })
                }
            };

            let raw_tag = tags
                .iter()
                .map(|tag| {
                    let mut s = tag.key.clone();
                    s.push('=');
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
                name: f.name.clone(),
                json_name: f.name.clone(),
                optional: f.optional,
                wire,
                tags,
                raw_tag,
                query_string_name,
                doc: doc.unwrap_or_else(|| "".into()),
            });
        }

        Ok(schema::Type {
            typ: Some(styp::Typ::Struct(schema::Struct { fields })),
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

        let obj_typ = self.builder.pc.type_checker.resolve_obj_type(&obj);
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

    fn transform_request(&mut self, typ: Option<Type>) -> Result<Option<schema::Type>> {
        let Some(typ) = typ else { return Ok(None) };

        let rs = self.builder.pc.type_checker.state();
        Ok(match typ {
            Type::Interface(mut interface) => {
                strip_path_params(rs, &mut interface);
                let Some(typ) = drop_empty_or_void(Type::Interface(interface)) else {
                    return Ok(None);
                };
                Some(self.typ(&typ)?)
            }
            Type::Named(ref named) => {
                let underlying = named.underlying(rs).clone();
                if let Type::Interface(mut iface) = underlying {
                    strip_path_params(rs, &mut iface);
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

    fn transform_response(&mut self, typ: Option<Type>) -> Result<Option<schema::Type>> {
        match typ {
            Some(typ) => Ok(Some(self.typ(&typ)?)),
            None => Ok(None),
        }
    }
}

pub(super) fn loc_from_range(app_root: &Path, fset: &FileSet, range: Range) -> Result<schema::Loc> {
    let loc = range.loc(fset)?;
    let (pkg_path, pkg_name, filename) = match loc.file {
        FilePath::Custom(ref str) => anyhow::bail!("unsupported file path in schema: {}", str),
        FilePath::Real(buf) => match buf.strip_prefix(app_root) {
            Ok(rel_path) => {
                let file_name = rel_path
                    .file_name()
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(anyhow::anyhow!("missing file name"))?;
                let pkg_name = rel_path
                    .parent()
                    .and_then(|p| p.file_name())
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(anyhow::anyhow!("missing package name"))?;
                let pkg_path = rel_path
                    .parent()
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(anyhow::anyhow!("missing package path"))?;
                (pkg_path, pkg_name, file_name)
            }
            Err(_) => {
                // The file is not relative to the app root.
                // Use a simplified path.
                let file_name = buf
                    .file_name()
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(anyhow::anyhow!("missing file name: {}", buf.display()))?;
                let pkg_name = buf
                    .parent()
                    .and_then(|p| p.file_name())
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(anyhow::anyhow!(
                        "missing package name for {}",
                        buf.display()
                    ))?;
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
