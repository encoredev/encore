use std::cell::RefCell;
use std::collections::HashMap;
use std::path::Path;

use anyhow::Result;
use itertools::Itertools;

use crate::encore::parser::schema::v1 as schema;
use crate::encore::parser::schema::v1::r#type as styp;
use crate::legacymeta::api_schema::strip_path_params;
use crate::parser::parser::ParseContext;
use crate::parser::resources::apis::encoding::FieldMap;
use crate::parser::types::custom::{resolve_custom_type_named, CustomType};
use crate::parser::types::{
    drop_empty_or_void, Basic, Interface, Literal, Named, ObjectId, Type, TypeResolver,
};
use crate::parser::{FilePath, FileSet, Range};

pub(super) struct SchemaBuilder<'a> {
    pc: &'a ParseContext<'a>,
    app_root: &'a Path,

    decls: RefCell<Vec<schema::Decl>>,
    obj_to_decl: RefCell<HashMap<ObjectId, u32>>,
}

impl<'a> SchemaBuilder<'a> {
    pub(super) fn new(pc: &'a ParseContext, app_root: &'a Path) -> Self {
        SchemaBuilder {
            pc,
            app_root,
            decls: RefCell::new(Vec::new()),
            obj_to_decl: RefCell::new(HashMap::new()),
        }
    }

    pub(super) fn into_decls(self) -> Vec<schema::Decl> {
        self.decls.take()
    }

    pub(super) fn typ(&self, typ: &Type) -> Result<schema::Type> {
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
            Type::Interface(tt) => schema::Type {
                typ: Some(styp::Typ::Struct(self.interface(tt)?)),
            },
            Type::Union(_) => anyhow::bail!("union types are not yet supported in schemas"),
            Type::Tuple(_) => anyhow::bail!("tuple types are not yet supported in schemas"),
            Type::Literal(tt) => schema::Type {
                typ: Some(styp::Typ::Builtin(self.literal(tt)? as i32)),
            },
            Type::Class(_) => anyhow::bail!("class types are not yet supported in schemas"),
            Type::Named(tt) => schema::Type {
                typ: Some(styp::Typ::Named(self.named(tt)?)),
            },
            Type::Signature(_) => anyhow::bail!("signature types are not yet supported in schemas"),
            Type::Optional(_) => anyhow::bail!("optional types are not yet supported in schemas"),
            Type::This => anyhow::bail!("this types are not yet supported in schemas"),
            Type::TypeArgument(_) => {
                anyhow::bail!("type argument types are not yet supported in schemas")
            }
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

    fn literal(&self, typ: &Literal) -> Result<schema::Builtin> {
        Ok(match typ {
            Literal::String(_) => schema::Builtin::String,
            Literal::Boolean(_) => schema::Builtin::Bool,
            Literal::Number(_) => schema::Builtin::Float64, // TODO figure out how to handle
            Literal::BigInt(_) => anyhow::bail!("TODO bigint"),
        })
    }

    fn interface(&self, typ: &Interface) -> Result<schema::Struct> {
        let mut fields = Vec::with_capacity(typ.fields.len());
        let ctx = self.pc.type_checker.ctx();
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

            fields.push(schema::Field {
                typ: Some(typ),
                name: f.name.clone(),
                json_name: f.name.clone(),
                optional: f.optional,
                wire,
                tags,
                raw_tag,
                query_string_name,

                doc: "".into(), // TODO
            });
        }
        Ok(schema::Struct { fields })
    }

    fn named(&self, typ: &Named) -> Result<schema::Named> {
        let type_arguments = self.types(&typ.type_arguments)?;
        let obj = &typ.obj;
        if let Some(decl_id) = self.obj_to_decl.borrow().get(&obj.id) {
            return Ok(schema::Named {
                id: *decl_id,
                type_arguments,
            });
        }

        // Allocate a new decl.
        let id = self.decls.borrow().len() as u32;
        let Some(name) = typ.obj.name.as_ref() else {
            anyhow::bail!("missing name for named object");
        };

        // Allocate the object and add it to the list without the underlying type.
        // We'll add the underlying type afterwards to properly handle recursive types.
        let loc = loc_from_range(self.app_root, &self.pc.file_set, obj.range)?;
        let decl = schema::Decl {
            id,
            name: name.clone(),
            r#type: None,        // computed below
            type_params: vec![], // TODO
            doc: "".into(),      // TODO
            loc: Some(loc),
        };
        self.decls.borrow_mut().push(decl);
        self.obj_to_decl.borrow_mut().insert(obj.id, id);

        let ctx = self.pc.type_checker.ctx();
        let obj_typ = obj.typ(ctx)?;
        let schema_typ = self.typ(&obj_typ)?;
        self.decls.borrow_mut().get_mut(id as usize).unwrap().r#type = Some(schema_typ);

        Ok(schema::Named { id, type_arguments })
    }

    fn new_named_from_type(
        &self,
        name: String,
        underlying: Type,
        range: Range,
        type_arguments: Vec<Type>,
    ) -> Result<schema::Named> {
        let type_arguments = self.types(&type_arguments)?;
        let underlying = self.typ(&underlying)?;

        // Allocate a new decl.
        let id = self.decls.borrow().len() as u32;
        // Allocate the object and add it to the list without the underlying type.
        // We'll add the underlying type afterwards to properly handle recursive types.
        let loc = loc_from_range(self.app_root, &self.pc.file_set, range)?;
        let decl = schema::Decl {
            id,
            name: name.clone(),
            r#type: Some(underlying),
            type_params: vec![], // TODO
            doc: "".into(),      // TODO
            loc: Some(loc),
        };
        self.decls.borrow_mut().push(decl);
        Ok(schema::Named { id, type_arguments })
    }

    fn types(&self, types: &[Type]) -> Result<Vec<schema::Type>> {
        let mut result = Vec::with_capacity(types.len());
        for t in types {
            result.push(self.typ(t)?);
        }
        Ok(result)
    }

    pub fn transform_request(&self, typ: Option<Type>) -> Result<Option<schema::Type>> {
        let Some(typ) = typ else { return Ok(None) };

        let ctx = self.pc.type_checker.ctx();
        Ok(match typ {
            Type::Interface(mut interface) => {
                strip_path_params(ctx, &mut interface);
                let Some(typ) = drop_empty_or_void(Type::Interface(interface)) else {
                    return Ok(None);
                };
                Some(self.typ(&typ)?)
            }
            Type::Named(named) => {
                if let Ok(Type::Interface(mut iface)) = ctx.obj_type(named.obj.clone()) {
                    strip_path_params(ctx, &mut iface);
                    let obj = named.obj;
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
                    match drop_empty_or_void(Type::Named(named)) {
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

    pub fn transform_response(&self, typ: Option<Type>) -> Result<Option<schema::Type>> {
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
                    .ok_or(anyhow::anyhow!("missing file name"))?;
                let pkg_name = buf
                    .parent()
                    .and_then(|p| p.file_name())
                    .map(|s| s.to_string_lossy().to_string())
                    .ok_or(anyhow::anyhow!("missing package name"))?;
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
