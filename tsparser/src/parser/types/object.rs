use std::cell::{Cell, OnceCell, RefCell};
use std::collections::HashMap;
use std::fmt::Debug;
use std::hash::Hash;
use std::ops::Deref;
use std::rc::Rc;

use anyhow::Result;
use swc_common::sync::Lrc;
use swc_common::Spanned;
use swc_ecma_ast as ast;

use crate::parser::module_loader::ModuleId;
use crate::parser::types::ast_id::AstId;
use crate::parser::types::binding::bindings;
use crate::parser::types::typ;
use crate::parser::types::type_resolve::{interface_decl, resolve_expr_type, resolve_type};
use crate::parser::{module_loader, Range};

#[derive(Debug, Clone, Copy, Hash, PartialEq, Eq)]
pub struct ObjectId(pub(super) usize);

/// An Object describes a named language entity such as a module, constant, type, variable, function, etc.
pub struct Object {
    pub id: ObjectId,
    pub range: Range,
    pub name: Option<String>,
    pub kind: ObjectKind,
    pub module_id: ModuleId,
    pub(super) state: RefCell<CheckState>,
}

impl Debug for Object {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Object")
            .field("id", &self.id)
            .field("range", &self.range)
            .field("name", &self.name)
            .field("kind", &self.kind)
            .field("module_id", &self.module_id)
            .finish()
    }
}

impl Hash for Object {
    fn hash<H: std::hash::Hasher>(&self, state: &mut H) {
        self.id.hash(state);
    }
}

impl PartialEq for Object {
    fn eq(&self, other: &Self) -> bool {
        self.id == other.id
    }
}

impl Eq for Object {}

#[derive(Debug)]
pub enum ObjectKind {
    TypeName(TypeName),
    Enum(Enum),
    Var(Var),
    Using(Using),
    Func(Func),
    Class(Class),
    Module(Module),
    Namespace(Namespace),
}

#[derive(Debug)]
pub(super) enum CheckState {
    NotStarted,
    InProgress,
    Completed(typ::Type),
}

#[derive(Debug)]
pub struct TypeName {
    decl: TypeNameDecl,
}

#[derive(Debug)]
pub enum TypeNameDecl {
    Interface(ast::TsInterfaceDecl),
    TypeAlias(ast::TsTypeAliasDecl),
}

#[derive(Debug)]
pub struct Class {
    #[allow(dead_code)]
    spec: Box<ast::Class>,
}

#[derive(Debug)]
pub struct Enum {
    members: Vec<ast::TsEnumMember>,
}

#[derive(Debug)]
pub struct Func {
    #[allow(dead_code)]
    spec: Box<ast::Function>,
}

#[derive(Debug)]
pub struct Namespace {
    data: Box<NSData>,
}

#[derive(Debug)]
pub struct Var {
    type_ann: Option<ast::TsTypeAnn>,
    expr: Option<Box<ast::Expr>>,
}

#[derive(Debug)]
pub struct Using {
    type_ann: Option<ast::TsTypeAnn>,
    expr: Option<Box<ast::Expr>>,
}

#[derive(Debug)]
struct NSData {
    /// The objects imported by the module.
    imports: HashMap<AstId, ImportedName>,

    /// Top-level objects, keyed by their id.
    top_level: HashMap<AstId, Rc<Object>>,

    /// The named exports.
    named_exports: HashMap<String, Rc<Object>>,

    /// The default export, if any.
    default_export: Option<Rc<Object>>,

    /// Export items that haven't yet been processed.
    #[allow(dead_code)]
    unprocessed_exports: Vec<ast::ModuleItem>,
}

#[derive(Debug)]
pub struct Module {
    base: Lrc<module_loader::Module>,
    data: Box<NSData>,
}

#[derive(Debug, Clone)]
pub(super) struct ImportedName {
    pub import_path: String,
    pub kind: ImportKind,
}

#[derive(Debug, Clone)]
pub enum ImportKind {
    Named(String),
    Default,
    Namespace,
}

impl NSData {
    fn new() -> Self {
        Self {
            imports: HashMap::new(),
            top_level: HashMap::new(),
            named_exports: HashMap::new(),
            default_export: None,
            unprocessed_exports: vec![],
        }
    }

    fn add_top_level(&mut self, id: AstId, obj: Rc<Object>) -> Result<Rc<Object>> {
        if let Some(other) = self.top_level.get(&id) {
            log::error!("duplicate object {:?}: unhandled overload?", id);
            return Ok(other.clone());
        }
        self.top_level.insert(id, obj.clone());
        Ok(obj)
    }

    fn add_import(&mut self, id: AstId, import: ImportedName) -> Result<()> {
        if self.imports.contains_key(&id) {
            anyhow::bail!("duplicate import: {}", id);
        }
        self.imports.insert(
            id,
            ImportedName {
                import_path: import.import_path.clone(),
                kind: import.kind.clone(),
            },
        );
        Ok(())
    }
}

fn process_module_items(ctx: &Ctx, ns: &mut NSData, items: &[ast::ModuleItem]) -> Result<()> {
    for it in items {
        match it {
            ast::ModuleItem::ModuleDecl(md) => match md {
                ast::ModuleDecl::Import(import) => process_import(ns, import)?,
                ast::ModuleDecl::ExportDecl(decl) => {
                    let objs = process_decl(ctx, ns, &decl.decl)?;
                    for obj in objs {
                        if let Some(name) = &obj.name {
                            ns.named_exports.insert(name.clone(), obj);
                        }
                    }
                }

                // TODO implement
                ast::ModuleDecl::ExportDefaultDecl(_) => {
                    log::debug!("TODO export default declaration");
                }

                // TODO(andre) Can this affect the module namespace?
                ast::ModuleDecl::ExportDefaultExpr(_) => {
                    log::debug!("TODO export default expr");
                }

                ast::ModuleDecl::ExportNamed(decl) => {
                    // Re-exporting from another module.
                    for spec in &decl.specifiers {
                        if let ast::ExportSpecifier::Named(named) = spec {
                            log::info!("re-export name {:?}", named.orig);
                        }
                    }
                    log::debug!("TODO re-export named declaration");
                }

                ast::ModuleDecl::ExportAll(_) => {
                    // Re-exporting * from another module.
                    log::debug!("TODO re-export * declaration");
                }

                ast::ModuleDecl::TsImportEquals(_) => {
                    log::debug!("TODO ts import equals");
                }

                ast::ModuleDecl::TsExportAssignment(_) => {
                    log::debug!("TODO ts export =");
                }

                ast::ModuleDecl::TsNamespaceExport(_) => {
                    log::debug!("TODO ts namespace export");
                }
            },

            ast::ModuleItem::Stmt(stmt) => {
                process_stmt(ctx, ns, stmt)?;
            }
        }
    }

    Ok(())
}

/// Process an import declaration, adding imports to the module.
fn process_import(ns: &mut NSData, import: &ast::ImportDecl) -> Result<()> {
    for specifier in &import.specifiers {
        match specifier {
            ast::ImportSpecifier::Named(named) => {
                let export_name = named.imported.as_ref().map_or_else(
                    || named.local.clone(),
                    |export_name| match export_name {
                        ast::ModuleExportName::Ident(id) => id.clone(),
                        ast::ModuleExportName::Str(str) => {
                            ast::Ident::new(str.value.clone(), str.span)
                        }
                    },
                );

                ns.add_import(
                    AstId::from(&named.local),
                    ImportedName {
                        import_path: import.src.value.to_string(),
                        kind: ImportKind::Named(export_name.sym.as_ref().to_string()),
                    },
                )?;
            }
            ast::ImportSpecifier::Default(default) => {
                ns.add_import(
                    AstId::from(&default.local),
                    ImportedName {
                        import_path: import.src.value.to_string(),
                        kind: ImportKind::Default,
                    },
                )?;
            }
            ast::ImportSpecifier::Namespace(ns_import) => {
                // import * as foo
                ns.add_import(
                    AstId::from(&ns_import.local),
                    ImportedName {
                        import_path: import.src.value.to_string(),
                        kind: ImportKind::Namespace,
                    },
                )?;
            }
        }
    }
    Ok(())
}

fn process_stmt(ctx: &Ctx, ns: &mut NSData, stmt: &ast::Stmt) -> Result<Vec<Rc<Object>>> {
    match stmt {
        ast::Stmt::Decl(decl) => process_decl(ctx, ns, decl),
        ast::Stmt::Block(block) => {
            let mut objs = vec![];
            for stmt in &block.stmts {
                objs.extend(process_stmt(ctx, ns, stmt)?);
            }
            Ok(objs)
        }

        // NOTE(andre): I believe other statements can't really declare things,
        // since they're inside blocks.
        _ => Ok(vec![]),
    }
}

fn process_decl(ctx: &Ctx, ns: &mut NSData, decl: &ast::Decl) -> Result<Vec<Rc<Object>>> {
    let range: Range = decl.span().into();
    Ok(match decl {
        ast::Decl::Class(d) => {
            let name = Some(d.ident.sym.to_string());
            let obj = ctx.new_obj(
                name,
                range,
                ObjectKind::Class(Class {
                    spec: d.class.clone(),
                }),
            );
            ns.add_top_level(AstId::from(&d.ident), obj.clone())?;
            vec![obj]
        }

        ast::Decl::Fn(d) => {
            let name = Some(d.ident.sym.to_string());
            let obj = ctx.new_obj(
                name,
                range,
                ObjectKind::Func(Func {
                    spec: d.function.clone(),
                }),
            );
            ns.add_top_level(AstId::from(&d.ident), obj.clone())?;
            vec![obj]
        }

        ast::Decl::Var(d) => {
            let mut objs = vec![];
            for var_decl in &d.decls {
                for b in bindings(&var_decl.name) {
                    let name = Some(b.name.to_string());
                    let range = var_decl.span.into();
                    let obj = ctx.new_obj(
                        name,
                        range,
                        ObjectKind::Var(Var {
                            type_ann: b.type_ann,
                            expr: var_decl.init.clone(),
                        }),
                    );
                    ns.add_top_level(AstId::new(b.id, b.name.clone()), obj.clone())?;
                    objs.push(obj);
                }
            }
            objs
        }

        ast::Decl::Using(d) => {
            let mut objs = vec![];
            for var_decl in &d.decls {
                for b in bindings(&var_decl.name) {
                    let name = Some(b.name.to_string());
                    let range = var_decl.span.into();
                    let obj = ctx.new_obj(
                        name,
                        range,
                        ObjectKind::Using(Using {
                            type_ann: b.type_ann,
                            expr: var_decl.init.clone(),
                        }),
                    );
                    ns.add_top_level(AstId::new(b.id, b.name.clone()), obj.clone())?;
                    objs.push(obj);
                }
            }
            objs
        }

        ast::Decl::TsInterface(d) => {
            let name = Some(d.id.sym.to_string());
            let obj = ctx.new_obj(
                name,
                range,
                ObjectKind::TypeName(TypeName {
                    decl: TypeNameDecl::Interface(*d.clone()),
                }),
            );
            ns.add_top_level(AstId::from(&d.id), obj.clone())?;
            vec![obj]
        }

        ast::Decl::TsTypeAlias(d) => {
            let name = d.id.sym.to_string();
            log::info!("registering ts type alias {}", name);
            let obj = ctx.new_obj(
                Some(name),
                range,
                ObjectKind::TypeName(TypeName {
                    decl: TypeNameDecl::TypeAlias(*d.clone()),
                }),
            );
            ns.add_top_level(AstId::from(&d.id), obj.clone())?;
            vec![obj]
        }

        ast::Decl::TsEnum(d) => {
            let name = Some(d.id.sym.to_string());
            let obj = ctx.new_obj(
                name,
                range,
                ObjectKind::Enum(Enum {
                    members: d.members.clone(),
                }),
            );
            ns.add_top_level(AstId::from(&d.id), obj.clone())?;
            vec![obj]
        }

        ast::Decl::TsModule(d) => {
            // Namespace declaration
            match &d.id {
                ast::TsModuleName::Ident(id) => {
                    let mut ns2 = Namespace {
                        data: Box::new(NSData::new()),
                    };
                    if let Some(body) = &d.body {
                        process_namespace_body(ctx, &mut ns2.data, body)?;
                    }

                    let name = Some(id.sym.to_string());
                    let obj = ctx.new_obj(name, range, ObjectKind::Namespace(ns2));
                    ns.add_top_level(AstId::from(id), obj.clone())?;
                    vec![obj]
                }
                ast::TsModuleName::Str(_) => {
                    // This is not valid for namespace declarations, ignore it.
                    vec![]
                }
            }
        }
    })
}

fn process_namespace_body(ctx: &Ctx, ns: &mut NSData, body: &ast::TsNamespaceBody) -> Result<()> {
    match body {
        ast::TsNamespaceBody::TsModuleBlock(block) => {
            process_module_items(ctx, ns, &block.body[..])?;
        }
        ast::TsNamespaceBody::TsNamespaceDecl(decl) => {
            let name = Some(decl.id.sym.to_string());
            let mut ns2 = Namespace {
                data: Box::new(NSData::new()),
            };
            process_namespace_body(ctx, &mut ns2.data, &decl.body)?;

            let range = decl.span.into();
            let obj = ctx.new_obj(name, range, ObjectKind::Namespace(ns2));
            ns.add_top_level(AstId::from(&decl.id), obj)?;
        }
    }
    Ok(())
}

#[derive(Debug)]
pub struct Ctx<'a> {
    loader: Lrc<module_loader::ModuleLoader<'a>>,
    modules: RefCell<HashMap<ModuleId, Rc<Module>>>,
    module_stack: RefCell<Vec<ModuleId>>,
    universe: OnceCell<Rc<Module>>,
    next_id: Cell<usize>,
}

impl<'a> Ctx<'a> {
    pub(super) fn new(loader: Lrc<module_loader::ModuleLoader<'a>>) -> Self {
        Self {
            loader,
            modules: RefCell::new(HashMap::new()),
            module_stack: RefCell::new(vec![]),
            universe: OnceCell::new(),
            next_id: Cell::new(1),
        }
    }

    pub(super) fn universe(&self) -> Rc<Module> {
        if let Some(universe) = self.universe.get() {
            return universe.to_owned();
        }

        let ast = self.loader.universe();
        let module = self.get_or_init_module(ast).unwrap();
        self.universe.set(module.clone()).unwrap();
        self.universe.get().unwrap().to_owned()
    }

    pub(super) fn new_obj(
        &self,
        name: Option<String>,
        range: Range,
        kind: ObjectKind,
    ) -> Rc<Object> {
        let obj_id = self.next_id.get();
        self.next_id.set(obj_id + 1);

        let module_id = self.module_id().expect("no current module");
        Rc::new(Object {
            id: ObjectId(obj_id),
            range,
            module_id,
            name,
            kind,
            state: RefCell::new(CheckState::NotStarted),
        })
    }

    pub(super) fn resolve(
        &self,
        module: Lrc<module_loader::Module>,
        expr: &ast::TsType,
    ) -> Result<typ::Type> {
        let module = self.get_or_init_module(module)?;
        self.with_curr_module(module.base.id, || resolve_type(self, expr))
    }

    pub(super) fn resolve_obj(
        &self,
        module: Lrc<module_loader::Module>,
        expr: &ast::Expr,
    ) -> Result<Option<Rc<Object>>> {
        let module = self.get_or_init_module(module)?;
        self.with_curr_module(module.base.id, || {
            Ok(match resolve_expr_type(self, expr)? {
                typ::Type::Named(named) => Some(named.obj.clone()),
                typ::Type::Class(cls) => Some(cls.obj.clone()),
                _ => None,
            })
        })
    }

    pub(super) fn lookup_module(&self, id: ModuleId) -> Option<Rc<Module>> {
        self.modules.borrow().get(&id).map(|m| m.clone())
    }

    pub fn is_universe(&self, id: ModuleId) -> bool {
        let universe = self.universe();
        universe.base.id == id
    }

    pub fn is_module_path(&self, id: ModuleId, name: &str) -> bool {
        if let Some(module) = self.lookup_module(id) {
            module.base.module_path.as_ref().is_some_and(|p| p == name)
        } else {
            false
        }
    }

    fn get_or_init_module(&self, module: Lrc<module_loader::Module>) -> Result<Rc<Module>> {
        let module_id = module.id;
        if let Some(m) = self.modules.borrow().get(&module_id) {
            return Ok(m.clone());
        }

        let mut data = Box::new(NSData::new());
        self.with_curr_module(module_id, || {
            process_module_items(self, &mut data, &module.ast.body[..])
        })?;

        let new_module = Rc::new(Module { base: module, data });

        self.modules
            .borrow_mut()
            .insert(module_id, new_module.clone());

        Ok(new_module)
    }

    fn with_curr_module<Fn, Res>(&self, module_id: ModuleId, f: Fn) -> Result<Res>
    where
        Fn: FnOnce() -> Result<Res>,
    {
        self.module_stack.borrow_mut().push(module_id);
        let result = f();
        self.module_stack.borrow_mut().pop();
        result
    }

    fn module_id(&self) -> Result<ModuleId> {
        let module_id = self
            .module_stack
            .borrow()
            .last()
            .ok_or_else(|| anyhow::anyhow!("internal error: no module on stack"))?
            .to_owned();
        Ok(module_id)
    }

    fn module(&self) -> Result<Rc<Module>> {
        let module_id = self.module_id()?;
        self.modules
            .borrow()
            .get(&module_id)
            .map(|m| m.clone())
            .ok_or_else(|| anyhow::anyhow!("internal error: module not found: {:?}", module_id))
    }

    pub(super) fn resolve_ident(&self, ident: &ast::Ident) -> Result<Rc<Object>> {
        let module = self.module()?;

        // Is it a top-level object in this module?
        let ast_id = AstId::from(ident);
        if let Some(obj) = module.data.top_level.get(&ast_id) {
            return Ok(obj.clone());
        }

        // Otherwise, is it an import?
        if let Some(imp_name) = module.data.imports.get(&ast_id) {
            return self.resolve_import(&module, imp_name);
        }

        // Is it in universe scope?
        {
            let universe = self.universe();
            let name = ident.sym.as_ref();
            if let Some(obj) = universe.data.named_exports.get(name) {
                return Ok(obj.clone());
            }
        }

        // Otherwise we don't know about this object.
        anyhow::bail!("object not found: {:?}", ident.sym.as_ref());
    }

    pub(super) fn resolve_import(&self, module: &Module, imp: &ImportedName) -> Result<Rc<Object>> {
        let ast_module = self.loader.resolve_import(&module.base, &imp.import_path)?;

        match &imp.kind {
            ImportKind::Named(name) => {
                let imported = self.get_or_init_module(ast_module)?;
                let obj = imported
                    .data
                    .named_exports
                    .get(name)
                    .ok_or_else(|| anyhow::anyhow!("object not found: {:?} (named import)", name))?
                    .to_owned();
                Ok(obj)
            }
            ImportKind::Default => {
                let imported = self.get_or_init_module(ast_module)?;
                let obj = imported
                    .data
                    .default_export
                    .as_ref()
                    .ok_or_else(|| {
                        anyhow::anyhow!("object not found: {:?} (default import)", imp.import_path)
                    })?
                    .to_owned();
                Ok(obj)
            }
            ImportKind::Namespace => {
                anyhow::bail!("unimplemented: namespace import");
            }
        }
    }

    pub fn obj_type(&self, obj: Rc<Object>) -> Result<typ::Type> {
        if matches!(&obj.kind, ObjectKind::Module(_)) {
            // Modules don't have a type.
            return Ok(typ::Type::Basic(typ::Basic::Never));
        };

        match obj.state.borrow().deref() {
            CheckState::Completed(typ) => return Ok(typ.clone()),
            CheckState::InProgress => {
                // TODO support certain types of circular references.
                anyhow::bail!("circular type reference");
            }
            CheckState::NotStarted => {
                // Fall through below to do actual type-checking.
                // Needs to be handled separately to avoid borrowing issues.
            }
        }
        // Post-condition: state is NotStarted.

        // Mark this object as being checked.
        *obj.state.borrow_mut() = CheckState::InProgress;

        let typ = self.with_curr_module(obj.module_id, || resolve_obj_type(self, obj.clone()))?;
        *obj.state.borrow_mut() = CheckState::Completed(typ.clone());
        Ok(typ)
    }
}

fn resolve_obj_type(ctx: &Ctx, obj: Rc<Object>) -> Result<typ::Type> {
    Ok(match &obj.kind {
        ObjectKind::TypeName(tn) => match &tn.decl {
            TypeNameDecl::Interface(iface) => {
                // TODO handle type params here
                interface_decl(ctx, iface)?
            }
            TypeNameDecl::TypeAlias(ta) => {
                // TODO handle type params here
                resolve_type(ctx, &*ta.type_ann)?
            }
        },

        ObjectKind::Enum(o) => {
            // The type of an enum is interface.
            let mut fields = Vec::with_capacity(o.members.len());
            for m in &o.members {
                let field_type = match &m.init {
                    None => typ::Type::Basic(typ::Basic::Number),
                    Some(expr) => resolve_expr_type(ctx, &*expr)?,
                };
                let name = match &m.id {
                    ast::TsEnumMemberId::Ident(id) => id.sym.as_ref().to_string(),
                    ast::TsEnumMemberId::Str(str) => str.value.as_ref().to_string(),
                };
                fields.push(typ::InterfaceField {
                    name,
                    typ: field_type,
                    optional: false,
                });
            }
            typ::Type::Interface(typ::Interface { fields })
        }

        ObjectKind::Var(o) => {
            // Do we have a type annotation? If so, use that.
            if let Some(type_ann) = &o.type_ann {
                resolve_type(ctx, &*type_ann.type_ann)?
            } else if let Some(expr) = &o.expr {
                resolve_expr_type(ctx, &*expr)?
            } else {
                typ::Type::Basic(typ::Basic::Never)
            }
        }

        ObjectKind::Using(o) => {
            // Do we have a type annotation? If so, use that.
            if let Some(type_ann) = &o.type_ann {
                resolve_type(ctx, &*type_ann.type_ann)?
            } else if let Some(expr) = &o.expr {
                resolve_expr_type(ctx, &*expr)?
            } else {
                typ::Type::Basic(typ::Basic::Never)
            }
        }

        ObjectKind::Func(_o) => {
            anyhow::bail!("TODO func type")
        }

        ObjectKind::Class(_o) => typ::Type::Class(typ::ClassType { obj: obj.clone() }),

        ObjectKind::Module(_o) => typ::Type::Basic(typ::Basic::Never),
        ObjectKind::Namespace(_o) => {
            // TODO include namespace objects in interface
            typ::Type::Basic(typ::Basic::Object)
        }
    })
}

pub trait TypeResolver {
    fn typ(&self, ctx: &Ctx) -> Result<typ::Type>;
}

impl TypeResolver for Rc<Object> {
    fn typ(&self, ctx: &Ctx) -> Result<typ::Type> {
        ctx.obj_type(self.clone())
    }
}
