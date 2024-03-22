use crate::parser::{FileSet, Pos};
use anyhow::Result;
use swc_common::sync::Lrc;

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{Bind, BindData};
use crate::parser::resourceparser::resource_parser::ResourceParserRegistry;
use crate::parser::resources::Resource;
use crate::parser::types::TypeChecker;

pub mod bind;
pub mod paths;
pub mod resource_parser;

#[derive(Debug)]
pub struct PassOneParser<'a> {
    file_set: Lrc<FileSet>,
    type_checker: Lrc<TypeChecker<'a>>,
    registry: ResourceParserRegistry<'a>,
    next_id: u32,
}

impl<'a> PassOneParser<'a> {
    pub fn new(
        file_set: Lrc<FileSet>,
        type_checker: Lrc<TypeChecker<'a>>,
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

    pub fn parse(&mut self, module: Lrc<Module>) -> Result<(Vec<Resource>, Vec<Lrc<Bind>>)> {
        let parsers = self.registry.interested_parsers(&module);
        let mut ctx = ResourceParseContext::new(&self.file_set, &self.type_checker, module.clone());
        for parser in parsers {
            (parser.run)(&mut ctx)?;
        }

        let mut binds = Vec::with_capacity(ctx.binds.len());
        for b in ctx.binds {
            self.next_id += 1;
            let name = b.ident.as_ref().map(|x| x.sym.as_ref().to_string());
            binds.push(Lrc::new(Bind {
                id: self.next_id.into(),
                name,
                object: b.object,
                kind: b.kind,
                resource: b.resource,
                range: Some(b.range),
                internal_bound_id: b.ident.map(|i| i.to_id()),
                module_id: module.id,
            }));
        }

        Ok((ctx.resources, binds))
    }
}

/// Encompasses the necessary information for parsing resources from a single TS file.
#[derive(Debug)]
pub struct ResourceParseContext<'a, 'b> {
    pub module: Lrc<Module>,
    pub type_checker: &'a TypeChecker<'b>,
    file_set: &'a FileSet,
    resources: Vec<Resource>,
    binds: Vec<BindData>,
}

impl<'a, 'b: 'a> ResourceParseContext<'a, 'b> {
    pub fn new(
        file_set: &'a FileSet,
        type_checker: &'a TypeChecker<'b>,
        module: Lrc<Module>,
    ) -> Self {
        Self {
            module,
            type_checker,
            file_set,
            resources: Vec::new(),
            binds: Vec::new(),
        }
    }

    /// Register a resource.
    pub fn add_resource(&mut self, res: Resource) {
        log::debug!("Found resource {}", res);
        self.resources.push(res);
    }

    /// Register a bind.
    pub fn add_bind(&mut self, bind: BindData) {
        // Treat "_" as an anonymous bind.
        let ident = match &bind.ident {
            Some(name) if name.sym.to_string() == "_" => None,
            x => x.to_owned(),
        };
        self.binds.push(BindData { ident, ..bind });
    }

    pub fn preceding_comments(&self, pos: Pos) -> Option<String> {
        self.file_set.preceding_comments(&self.module.comments, pos)
    }
}
