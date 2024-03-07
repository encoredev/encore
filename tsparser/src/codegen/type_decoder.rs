use crate::codegen::{Generator, Writer};
use crate::parser::types::{Basic, Ctx, Interface, Literal, Object, Type};
use anyhow::Result;
use serde_json::json;
use std::collections::{HashMap, HashSet};

pub(super) struct TypeDecoder<'a> {
    ctx: &'a Ctx<'a>,

    /// The generated funcs.
    output_funcs: Vec<String>,

    /// The types we've already generated functions for.
    generated_funcs: HashMap<Type, String>,

    /// The func names we've already generated,
    /// to avoid name conflicts.
    allocated_func_names: HashSet<String>,

    // Whether we're generating typescript or javascript.
    typescript: bool,
}

impl<'a> TypeDecoder<'a> {
    pub fn new(ctx: &'a Ctx<'a>, typescript: bool) -> Self {
        Self {
            ctx,
            output_funcs: Vec::new(),
            generated_funcs: HashMap::new(),
            allocated_func_names: HashSet::new(),
            typescript,
        }
    }

    pub fn funcs(self) -> Vec<String> {
        self.output_funcs
    }

    pub fn typ(&mut self, w: &mut Writer, val: &str, path: Path, t: &Type) -> Result<()> {
        Ok(match t {
            Type::Basic(b) => self.basic(w, val, path, b)?,
            Type::Array(elem) => self.array(w, val, path, elem)?,
            Type::Interface(iface) => self.interface(w, val, path, iface)?,
            Type::Union(_) => anyhow::bail!("JSON decode into union is not yet supported"),
            Type::Tuple(elems) => self.tuple(w, val, path, elems)?,
            Type::Literal(lit) => self.literal(w, val, path, lit)?,
            Type::Class(_) => anyhow::bail!("JSON decode into class types is not yet supported"),
            Type::Signature(_) => anyhow::bail!("JSON decode into Signature type is not supported"),
            Type::Optional(opt) => {
                w.write("de.optional(");
                w.write(&val);
                w.write(", (v) => ");
                self.typ(w, "v", path, &opt)?;
                w.write(")");
            }
            Type::This => anyhow::bail!("JSON decode into This type is unsupported"),
            Type::TypeArgument(_) => anyhow::bail!("internal error: decode into TypeArgument type"),
            Type::Named(named) => {
                fn is_inline_type(ctx: &Ctx, obj: &Object) -> bool {
                    match obj.name.as_deref() {
                        Some("Path" | "Header" | "Query") => {
                            ctx.is_module_path(obj.module_id, "encore.dev/api")
                        }
                        _ => false,
                    }
                }

                if is_inline_type(self.ctx, &named.obj) {
                    // TODO type arguments
                    let underlying = self.ctx.obj_type(named.obj.clone())?;
                    self.typ(w, val, path, &underlying)?;
                } else {
                    // Get the function to invoke for this type.
                    let func_name = self.get_or_generate_json_func(t)?;
                    w.write(&func_name);
                    w.write("(");
                    w.write(val);
                    w.write(", ");
                    path.write(w);
                    w.write(")");
                }
            }
        })
    }

    fn basic(&mut self, w: &mut Writer, val: &str, path: Path, b: &Basic) -> Result<()> {
        let mut write_fn = |func| {
            w.write("de.");
            w.write(func);
            w.write("(");
            w.write(val);
            w.write(", ");
            path.write(w);
            w.write(")");
        };

        Ok(match b {
            Basic::Any => w.write(val),
            Basic::String => write_fn("string"),
            Basic::Boolean => write_fn("bool"),
            Basic::Number => write_fn("number"),
            Basic::Object => write_fn("object"),
            Basic::BigInt => write_fn("bigInt"),
            Basic::Symbol => anyhow::bail!("cannot decode JSON into Symbol"),
            Basic::Undefined => w.write("undefined"),
            Basic::Null => w.write("null"),
            Basic::Void => anyhow::bail!("cannot decode JSON into void"),
            Basic::Unknown => {
                w.write(val);
                w.write(" as unknown")
            }
            Basic::Never => anyhow::bail!("cannot decode JSON into never"),
        })
    }

    fn array(&mut self, w: &mut Writer, val: &str, path: Path, elem: &Type) -> Result<()> {
        w.write("de.array(");
        w.write(val);
        w.write(", ");
        path.write(w);
        w.write(", (v, path) => ");

        let mut segs = vec![PathSegment::Root, PathSegment::CopyPath("path".to_string())];
        self.typ(w, "v", Path::new(&mut segs), &*elem)?;

        w.write(")");
        Ok(())
    }

    fn interface(
        &mut self,
        w: &mut Writer,
        val: &str,
        mut path: Path,
        iface: &Interface,
    ) -> Result<()> {
        w.write("de.iface(");
        w.write(val);
        w.write(", ");
        path.write(w);
        w.write(", (v) => (");

        if iface.fields.is_empty() {
            w.write("{}");
        } else {
            w.writeln("{");
            {
                let mut w = w.indent();
                let num_fields = iface.fields.len();
                for (i, f) in iface.fields.iter().enumerate() {
                    w.write(&f.name);
                    w.write(": ");

                    let field_val = format!("{}.{}", val, f.name);
                    if f.optional {
                        w.write("de.optional(");
                        w.write(&field_val);
                        w.write(", (v) => ");
                    }
                    self.typ(&mut w, &field_val, path.push_lit(&f.name), &f.typ)?;
                    if f.optional {
                        w.write("");
                    }
                    if i < (num_fields - 1) {
                        w.write(",");
                    }
                    w.newline();
                }
            }
            w.write("}");
        }

        w.write("))");

        Ok(())
    }

    fn tuple(&mut self, w: &mut Writer, val: &str, mut path: Path, elems: &[Type]) -> Result<()> {
        w.write("de.tuple(");
        w.write_string(elems.len().to_string());
        w.write(", ");
        w.write(val);
        w.write(", ");
        path.write(w);
        w.write(", (v) => ");

        if elems.is_empty() {
            w.write("[]");
        } else {
            w.writeln("[");
            {
                let mut w = w.indent();
                let num_elems = elems.len();
                for (i, t) in elems.iter().enumerate() {
                    let elem_val = format!("{}[{}]", val, i);
                    self.typ(&mut w, &elem_val, path.push_expr(i.to_string()), t)?;
                    if i < (num_elems - 1) {
                        w.write(",");
                    }
                    w.newline();
                }
            }
            w.write("]");
        }

        Ok(())
    }

    fn literal(&mut self, w: &mut Writer, val: &str, path: Path, lit: &Literal) -> Result<()> {
        w.write("de.literal(");
        w.write(val);
        w.write(", ");
        path.write(w);
        w.write(", ");
        match lit {
            Literal::String(s) => w.write(s),
            Literal::Boolean(bool) => w.write_string(bool.to_string()),
            Literal::Number(num) => w.write_string(num.to_string()),
            Literal::BigInt(big) => w.write_string(big.to_string()),
        }
        w.write(")");
        Ok(())
    }

    /// Generate a function that deserializes the given type.
    /// It returns the name of the generated function.
    fn get_or_generate_json_func(&mut self, t: &Type) -> Result<String> {
        // Has a function already been allocated for this exact type?
        if let Some(name) = self.generated_funcs.get(&t) {
            return Ok(name.clone());
        }

        let func_name = self.alloc_func_name(t);
        self.generate_json_func(t, &func_name)?;

        Ok(func_name)
    }

    /// Unconditionally generates a function that deserializes the given type,
    /// using the given func_name as the name of the function.
    /// Outputs the result to self.output_funcs.
    fn generate_json_func(&mut self, t: &Type, func_name: &str) -> Result<()> {
        let mut gen = Generator::new();
        let mut w = gen.writer();
        w.write("function ");
        w.write(&func_name);
        if self.typescript {
            w.writeln("(v: any, path: Path) {");
        } else {
            w.writeln("(v, path) {");
        }

        {
            let mut w = w.indent();
            w.write("return ");
            let mut segs = vec![PathSegment::Root, PathSegment::CopyPath("path".to_string())];

            match t {
                Type::Named(named) => {
                    // Use the underlying type to avoid recursion.
                    let underlying = self.ctx.obj_type(named.obj.clone())?;
                    self.typ(&mut w, "v", Path::new(&mut segs), &underlying)?;
                }
                _ => {
                    self.typ(&mut w, "v", Path::new(&mut segs), t)?;
                }
            }

            w.writeln(";");
        }

        w.writeln("}");

        self.output_funcs.push(gen.buf);
        Ok(())
    }

    fn alloc_func_name(&mut self, t: &Type) -> String {
        let hints = t.name_hint();
        let initial = if hints.is_empty() {
            "decode".to_string()
        } else {
            hints.join("_")
        };

        let mut candidate = initial.clone();
        let mut i = 1;
        while self.allocated_func_names.contains(&candidate) {
            i += 1;
            candidate = format!("{}_{}", initial, i);
        }
        self.allocated_func_names.insert(candidate.clone());
        self.generated_funcs.insert(t.clone(), candidate.clone());
        candidate
    }
}

pub(super) enum PathSegment {
    Root,
    Lit(String),
    Expr(String),
    // Copy the path from another path.
    CopyPath(String),
}

pub(super) struct Path<'a> {
    segments: &'a mut Vec<PathSegment>,
}

impl<'a> Path<'a> {
    pub(super) fn new(segments: &'a mut Vec<PathSegment>) -> Self {
        Self { segments }
    }

    fn write(&self, w: &mut Writer) {
        // If we have only a copy-path segment, just use that directly.
        if self.segments.len() == 2 {
            match (&self.segments[0], &self.segments[1]) {
                (PathSegment::Root, PathSegment::CopyPath(name)) => {
                    w.write(name);
                    return;
                }
                _ => {}
            }
        }

        w.write("[");
        let mut first = true;
        for (_i, elem) in self.segments.iter().enumerate() {
            if !first {
                w.write(", ");
            }

            match elem {
                // Root is not part of the JS representation.
                PathSegment::Root => {}

                PathSegment::CopyPath(name) => {
                    first = false;
                    w.write("...");
                    w.write(name);
                }

                PathSegment::Lit(s) => {
                    first = false;
                    let out = json!(s);
                    w.write_string(out.to_string());
                }
                PathSegment::Expr(s) => {
                    first = false;
                    w.write(&s)
                }
            }
        }
        w.write("]");
    }

    fn push_expr<S: Into<String>>(&mut self, val: S) -> Path {
        self.segments.push(PathSegment::Expr(val.into()));
        Path {
            segments: &mut self.segments,
        }
    }

    fn push_lit<S: Into<String>>(&mut self, val: S) -> Path {
        self.segments.push(PathSegment::Lit(val.into()));
        Path {
            segments: &mut self.segments,
        }
    }
}

impl Drop for Path<'_> {
    fn drop(&mut self) {
        self.segments.pop().unwrap();
    }
}

trait NameHint {
    fn name_hint(&self) -> Vec<String>;
}

impl NameHint for Type {
    fn name_hint(&self) -> Vec<String> {
        match self {
            Type::Basic(b) => b.name_hint(),
            Type::Array(elem) => elem.name_hint(),
            Type::Interface(_iface) => vec![],
            Type::Union(types) => types.iter().flat_map(|t| t.name_hint()).collect(),
            Type::Tuple(elems) => elems.iter().flat_map(|s| s.name_hint()).collect(),
            Type::Literal(lit) => lit.name_hint(),
            Type::Class(cls) => cls.obj.name.iter().cloned().collect(),
            Type::Named(named) => {
                let mut hints = Vec::with_capacity(1 + named.type_arguments.len());
                hints.extend(named.obj.name.iter().cloned());
                hints.extend(named.type_arguments.iter().flat_map(|t| t.name_hint()));
                hints
            }
            Type::Signature(_) => vec![],
            Type::Optional(opt) => opt.name_hint(),
            Type::This => vec![],
            Type::TypeArgument(_) => vec![],
        }
    }
}

impl NameHint for Basic {
    fn name_hint(&self) -> Vec<String> {
        match self {
            Basic::Any => vec!["any".to_string()],
            Basic::String => vec!["string".to_string()],
            Basic::Boolean => vec!["boolean".to_string()],
            Basic::Number => vec!["number".to_string()],
            Basic::Object => vec!["object".to_string()],
            Basic::BigInt => vec!["bigint".to_string()],
            Basic::Symbol => vec!["symbol".to_string()],
            Basic::Undefined => vec!["undefined".to_string()],
            Basic::Null => vec!["null".to_string()],
            Basic::Void => vec!["void".to_string()],
            Basic::Unknown => vec!["unknown".to_string()],
            Basic::Never => vec!["never".to_string()],
        }
    }
}

impl NameHint for Literal {
    fn name_hint(&self) -> Vec<String> {
        self.basic().name_hint()
    }
}
