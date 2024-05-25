use std::collections::{HashMap, HashSet};
use std::fmt::Formatter;
use std::path::{Path, PathBuf};

use anyhow::Result;
use swc_common::errors::Handler;
use swc_common::sync::Lrc;
use swc_common::SourceMap;
use swc_ecma_loader::resolve::Resolve;
use swc_ecma_loader::resolvers::node::NodeModulesResolver;
use swc_ecma_loader::TargetEnv;

use crate::parser::module_loader::ModuleLoader;
use crate::parser::resourceparser::bind::{Bind, BindKind};
use crate::parser::resourceparser::PassOneParser;
use crate::parser::resources::apis::service_client::ServiceClient;
use crate::parser::resources::Resource;
use crate::parser::scan::scan;
use crate::parser::types::TypeChecker;
use crate::parser::usageparser::{Usage, UsageResolver};
use crate::parser::FileSet;
use crate::runtimeresolve::{EncoreRuntimeResolver, TsConfigPathResolver};

use super::resourceparser::bind::ResourceOrPath;
use super::resourceparser::UnresolvedBind;
use super::resources::ResourcePath;

pub struct ParseContext {
    /// Directory roots to parse for Encore resources.
    /// Typically this is the single directory containing the 'encore.app' file.
    pub dir_roots: Vec<PathBuf>,

    /// The module loader to use.
    pub loader: Lrc<ModuleLoader>,

    pub type_checker: Lrc<TypeChecker>,

    /// The file set to use.
    pub file_set: Lrc<FileSet>,

    /// The error handler to emit errors to.
    pub errs: Lrc<Handler>,
}

impl std::fmt::Debug for ParseContext {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("ParseContext")
            .field("dir_roots", &self.dir_roots)
            .finish()
    }
}

impl ParseContext {
    pub fn new(
        app_root: PathBuf,
        js_runtime_path: PathBuf,
        cm: Lrc<SourceMap>,
        errs: Lrc<Handler>,
    ) -> Result<Self> {
        let resolver = NodeModulesResolver::with_export_conditions(
            TargetEnv::Node,
            Default::default(),
            true,
            vec!["bun".into(), "deno".into(), "types".into()],
        );
        Self::with_resolver(app_root, js_runtime_path, resolver, cm, errs)
    }

    pub fn with_resolver<R>(
        app_root: PathBuf,
        js_runtime_path: PathBuf,
        resolver: R,
        cm: Lrc<SourceMap>,
        errs: Lrc<Handler>,
    ) -> Result<Self>
    where
        R: Resolve + 'static,
    {
        let mut resolver =
            EncoreRuntimeResolver::new(resolver, js_runtime_path, vec!["types".into()]);

        // Do we have a tsconfig.json file in the app root?
        {
            let tsconfig_path = app_root.join("tsconfig.json");
            if tsconfig_path.exists() {
                let tsconfig = TsConfigPathResolver::from_file(&tsconfig_path)?;
                resolver = resolver.with_tsconfig_resolver(tsconfig);
            }
        }

        let file_set = FileSet::new(cm.clone());
        let loader = Lrc::new(ModuleLoader::new(
            errs.clone(),
            file_set.clone(),
            Box::new(resolver),
        ));
        let type_checker = Lrc::new(TypeChecker::new(loader.clone()));

        Ok(Self {
            dir_roots: vec![app_root],
            loader,
            type_checker,
            file_set,
            errs,
        })
    }
}

pub struct Parser<'a> {
    pc: &'a ParseContext,
    pass1: PassOneParser<'a>,

    resources: Vec<Resource>,
    binds: Vec<UnresolvedBind>,
}

pub struct ParseResult {
    pub resources: Vec<Resource>,
    pub binds: Vec<Lrc<Bind>>,
    pub usages: Vec<Usage>,
}

impl<'a> Parser<'a> {
    pub fn new(pc: &'a ParseContext, pass1: PassOneParser<'a>) -> Self {
        let resources = Vec::new();
        let binds = Vec::new();
        Self {
            pc,
            pass1,
            resources,
            binds,
        }
    }

    /// Run the parser.
    pub fn parse(mut self) -> Result<ParseResult> {
        for dir_root in &self.pc.dir_roots {
            self.parse_root(dir_root)?;
        }
        self.inject_generated_service_clients();

        let binds = resolve_binds(self.binds)?;
        let resolver = UsageResolver::new(&self.pc.loader, &self.resources, &binds);
        let mut usages = Vec::new();

        for module in self.pc.loader.modules() {
            let exprs = resolver.scan_usage_exprs(&module)?;
            let u = resolver.resolve_usage(&exprs)?;
            usages.extend(u);
        }

        let result = ParseResult {
            resources: self.resources,
            binds,
            usages,
        };

        Ok(result)
    }

    /// Parse a single root directory.
    fn parse_root(&mut self, root: &std::path::Path) -> Result<()> {
        let loader = &self.pc.loader;
        scan(loader, root, |m| {
            let (resources, binds) = self.pass1.parse(m)?;

            self.resources.extend(resources);
            self.binds.extend(binds);
            Ok(())
        })
    }

    fn inject_generated_service_clients(&mut self) {
        // Find the services we have
        let mut services = HashSet::new();
        for res in &self.resources {
            if let Resource::APIEndpoint(ep) = res {
                services.insert(ep.service_name.clone());
            }
        }

        let module = self.pc.loader.encore_app_clients();
        for service_name in services {
            let client = Lrc::new(ServiceClient {
                service_name: service_name.clone(),
            });
            let resource = Resource::ServiceClient(client.clone());
            self.resources.push(resource.clone());

            let id = self.pass1.alloc_bind_id();
            self.binds.push(UnresolvedBind {
                id,
                name: Some(service_name),
                object: None,
                kind: BindKind::Create,
                resource: ResourceOrPath::Resource(resource),
                range: None,
                internal_bound_id: None,
                module_id: module.id,
            });
        }
    }
}

fn resolve_binds(binds: Vec<UnresolvedBind>) -> Result<Vec<Lrc<Bind>>> {
    // Collect the resources we support by path.
    let resource_paths = binds
        .iter()
        .filter_map(|b| match &b.resource {
            ResourceOrPath::Resource(res) => match res {
                Resource::SQLDatabase(db) => Some((
                    ResourcePath::SQLDatabase {
                        name: db.name.clone(),
                    },
                    res.clone(),
                )),
                _ => None,
            },
            ResourceOrPath::Path(_) => None,
        })
        .collect::<HashMap<ResourcePath, Resource>>();

    let mut result = Vec::with_capacity(binds.len());
    for b in binds {
        let resource = match b.resource {
            ResourceOrPath::Resource(res) => res,
            ResourceOrPath::Path(path) => {
                let res = resource_paths
                    .get(&path)
                    .ok_or_else(|| anyhow::anyhow!("resource not found: {:?}", path))?;
                res.clone()
            }
        };

        result.push(Lrc::new(Bind {
            id: b.id,
            range: b.range,
            resource,
            kind: b.kind,
            module_id: b.module_id,
            name: b.name,
            object: b.object,
            internal_bound_id: b.internal_bound_id,
        }));
    }

    Ok(result)
}
