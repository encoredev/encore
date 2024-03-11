use std::collections::HashSet;
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

pub struct ParseContext<'a> {
    /// Directory roots to parse for Encore resources.
    /// Typically this is the single directory containing the 'encore.app' file.
    pub dir_roots: Vec<PathBuf>,

    /// The module loader to use.
    pub loader: Lrc<ModuleLoader<'a>>,

    pub type_checker: Lrc<TypeChecker<'a>>,

    /// The file set to use.
    pub file_set: Lrc<FileSet>,

    /// The error handler to emit errors to.
    pub errs: Lrc<Handler>,
}

impl std::fmt::Debug for ParseContext<'_> {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("ParseContext")
            .field("dir_roots", &self.dir_roots)
            .finish()
    }
}

impl<'a> ParseContext<'a> {
    pub fn new(app_root: PathBuf, js_runtime_path: &'a Path) -> Result<Self> {
        let resolver = NodeModulesResolver::with_export_conditions(
            TargetEnv::Node,
            Default::default(),
            true,
            vec!["bun".into(), "deno".into(), "types".into()],
        );
        Self::with_resolver(app_root, js_runtime_path, resolver)
    }

    pub fn with_resolver<R>(app_root: PathBuf, js_runtime_path: &'a Path, resolver: R) -> Result<Self>
    where
        R: Resolve + 'a,
    {
        let cm: Lrc<SourceMap> = Default::default();
        let errs = Lrc::new(Handler::with_tty_emitter(
            swc_common::errors::ColorConfig::Auto,
            true,
            false,
            Some(cm.clone()),
        ));

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
        let loader = Lrc::new(ModuleLoader::new(errs.clone(), file_set.clone(), Box::new(resolver)));
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
    pc: &'a ParseContext<'a>,
    pass1: PassOneParser<'a>,

    result: ParseResult,
}

pub struct ParseResult {
    pub resources: Vec<Resource>,
    pub binds: Vec<Lrc<Bind>>,
    pub usages: Vec<Usage>,
}

impl<'a> Parser<'a> {
    pub fn new(pc: &'a ParseContext<'a>, pass1: PassOneParser<'a>) -> Self {
        let result = ParseResult {
            resources: Vec::new(),
            binds: Vec::new(),
            usages: Vec::new(),
        };
        Self { pc, pass1, result }
    }

    /// Run the parser.
    pub fn parse(mut self) -> Result<ParseResult> {
        for dir_root in &self.pc.dir_roots {
            self.parse_root(dir_root)?;
        }
        self.inject_generated_service_clients();

        let resolver =
            UsageResolver::new(&self.pc.loader, &self.result.resources, &self.result.binds);

        for module in self.pc.loader.modules() {
            let exprs = resolver.scan_usage_exprs(&module)?;
            let usages = resolver.resolve_usage(&exprs)?;
            self.result.usages.extend(usages);
        }

        Ok(self.result)
    }

    /// Parse a single root directory.
    fn parse_root(&mut self, root: &std::path::Path) -> Result<()> {
        let loader = &self.pc.loader;
        scan(loader, root, |m| {
            let (resources, binds) = self.pass1.parse(m)?;

            self.result.resources.extend(resources);
            self.result.binds.extend(binds.clone());
            Ok(())
        })
    }

    fn inject_generated_service_clients(&mut self) {
        // Find the services we have
        let mut services = HashSet::new();
        for res in &self.result.resources {
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
            self.result.resources.push(resource.clone());

            let id = self.pass1.alloc_bind_id();
            self.result.binds.push(Lrc::new(Bind {
                id,
                name: Some(service_name),
                object: None,
                kind: BindKind::Create,
                resource,
                range: None,
                internal_bound_id: None,
                module_id: module.id,
            }));
        }
    }
}
