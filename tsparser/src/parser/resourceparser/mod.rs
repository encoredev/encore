use std::borrow::Cow;
use std::rc::Rc;

use anyhow::Result;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::BindData;
use crate::parser::resourceparser::resource_parser::ResourceParserRegistry;
use crate::parser::resources::Resource;
use crate::parser::types::TypeChecker;
use crate::parser::{FileSet, Pos};

use self::bind::BindKind;

use super::module_loader::ModuleId;
use super::types::Object;
use super::Range;

pub mod bind;
pub mod paths;
pub mod resource_parser;

#[derive(Debug)]
pub struct PassOneParser<'a> {
    file_set: Lrc<FileSet>,
    type_checker: Lrc<TypeChecker>,
    registry: ResourceParserRegistry<'a>,
    next_id: u32,
}

#[derive(Debug)]
pub struct UnresolvedBind {
    pub id: bind::Id,
    pub range: Option<Range>,
    pub resource: bind::ResourceOrPath,
    pub kind: BindKind,

    /// The module the bind is defined in.
    pub module_id: ModuleId,

    /// The identifier it is bound to, if any.
    /// None means it's an anonymous bind (e.g. `_`).
    pub name: Option<String>,

    /// The object it is bound to, if any.
    pub object: Option<Rc<Object>>,

    /// The identifier it's bound to in the source module.
    /// None means it's an anonymous bind (e.g. `_`).
    /// It's used for computing usage within the module itself,
    /// where we need to know its id.
    pub internal_bound_id: Option<ast::Id>,
}

impl<'a> PassOneParser<'a> {
    pub fn new(
        file_set: Lrc<FileSet>,
        type_checker: Lrc<TypeChecker>,
        registry: ResourceParserRegistry<'a>,
    ) -> Self {
        Self {
            file_set,
            type_checker,
            registry,
            next_id: 0,
        }
    }

    pub fn alloc_bind_id(&mut self) -> bind::Id {
        self.next_id += 1;
        self.next_id.into()
    }

    pub fn parse(
        &mut self,
        module: Lrc<Module>,
        service_name: Option<&str>,
    ) -> Result<(Vec<Resource>, Vec<UnresolvedBind>)> {
        let parsers = self.registry.interested_parsers(&module);

        let mut ctx = ResourceParseContext::new(
            &self.file_set,
            &self.type_checker,
            module.clone(),
            service_name.map(Cow::Borrowed),
        );

        log::debug!(
            "parsing module {} with svc name {:?}",
            module.file_path,
            ctx.service_name
        );

        for parser in parsers {
            let num_resources = ctx.resources.len();
            (parser.run)(&mut ctx)?;

            // Look at any new resources to see if we have a new service.
            for res in &ctx.resources[num_resources..] {
                if let Resource::Service(svc) = res {
                    log::debug!("setting service name to {}", svc.name);
                    ctx.service_name = Some(Cow::Owned(svc.name.clone()));
                }
            }
        }

        let mut binds = Vec::with_capacity(ctx.binds.len());
        for b in ctx.binds {
            self.next_id += 1;
            let name = b.ident.as_ref().map(|x| x.sym.as_ref().to_string());
            binds.push(UnresolvedBind {
                id: self.next_id.into(),
                name,
                object: b.object,
                kind: b.kind,
                resource: b.resource,
                range: Some(b.range),
                internal_bound_id: b.ident.map(|i| i.to_id()),
                module_id: module.id,
            });
        }

        Ok((ctx.resources, binds))
    }
}

/// Encompasses the necessary information for parsing resources from a single TS file.
#[derive(Debug)]
pub struct ResourceParseContext<'a> {
    pub module: Lrc<Module>,
    pub type_checker: &'a TypeChecker,
    pub service_name: Option<Cow<'a, str>>,
    file_set: &'a FileSet,
    resources: Vec<Resource>,
    binds: Vec<BindData>,
}

impl<'a> ResourceParseContext<'a> {
    pub fn new(
        file_set: &'a FileSet,
        type_checker: &'a TypeChecker,
        module: Lrc<Module>,
        service_name: Option<Cow<'a, str>>,
    ) -> Self {
        Self {
            module,
            type_checker,
            file_set,
            service_name,
            resources: Vec::new(),
            binds: Vec::new(),
        }
    }

    /// Register a resource.
    pub fn add_resource(&mut self, res: Resource) {
        log::debug!("found resource {}", res);
        self.resources.push(res);
    }

    /// Register a bind.
    pub fn add_bind(&mut self, bind: BindData) {
        // Treat "_" as an anonymous bind.
        let ident = match &bind.ident {
            Some(name) if name.sym == "_" => None,
            x => x.to_owned(),
        };
        self.binds.push(BindData { ident, ..bind });
    }

    pub fn preceding_comments(&self, pos: Pos) -> Option<String> {
        self.file_set.preceding_comments(&self.module.comments, pos)
    }
}
