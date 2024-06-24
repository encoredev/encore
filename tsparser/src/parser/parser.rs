use std::collections::HashMap;
use std::ffi::OsStr;
use std::fmt::Formatter;
use std::path::{Path, PathBuf};

use anyhow::Result;
use swc_common::errors::Handler;
use swc_common::sync::Lrc;
use swc_common::SourceMap;
use swc_ecma_loader::resolve::Resolve;
use swc_ecma_loader::resolvers::node::NodeModulesResolver;
use swc_ecma_loader::TargetEnv;
use walkdir::WalkDir;

use crate::parser::module_loader::ModuleLoader;
use crate::parser::resourceparser::bind::{Bind, BindKind};
use crate::parser::resourceparser::PassOneParser;
use crate::parser::resources::apis::service_client::ServiceClient;
use crate::parser::resources::Resource;
use crate::parser::service_discovery::{discover_services, DiscoveredService};
use crate::parser::types::TypeChecker;
use crate::parser::usageparser::{Usage, UsageResolver};
use crate::parser::{FilePath, FileSet};
use crate::runtimeresolve::{EncoreRuntimeResolver, TsConfigPathResolver};

use super::resourceparser::bind::ResourceOrPath;
use super::resourceparser::UnresolvedBind;
use super::resources::ResourcePath;

pub struct ParseContext {
    /// App root to parse for Encore resources.
    /// The directory containing the 'encore.app' file.
    pub app_root: PathBuf,

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
            .field("app_root", &self.app_root)
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
                let tsconfig = Lrc::new(TsConfigPathResolver::from_file(&tsconfig_path)?);
                resolver = resolver.with_tsconfig_resolver(tsconfig.clone());
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
            app_root,
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
}

#[derive(Debug)]
pub struct ParseResult {
    pub resources: Vec<Resource>,
    pub binds: Vec<Lrc<Bind>>,
    pub usages: Vec<Usage>,
    pub services: Vec<Service>,
}

impl<'a> Parser<'a> {
    pub fn new(pc: &'a ParseContext, pass1: PassOneParser<'a>) -> Self {
        Self { pc, pass1 }
    }

    /// Run the parser.
    pub fn parse(mut self) -> Result<ParseResult> {
        fn ignored(entry: &walkdir::DirEntry) -> bool {
            match entry.file_name().to_str().unwrap_or_default() {
                "node_modules" | "encore.gen" => true,
                x => x.starts_with('.'),
            }
        }

        let walker = WalkDir::new(&self.pc.app_root)
            .sort_by(|a, b| {
                fn is_service(e: &walkdir::DirEntry) -> bool {
                    e.path().ends_with("encore.service.ts")
                }
                if is_service(a) {
                    std::cmp::Ordering::Less
                } else if is_service(b) {
                    std::cmp::Ordering::Greater
                } else {
                    a.file_name().cmp(b.file_name())
                }
            })
            .into_iter()
            .filter_entry(|e| !ignored(e));

        // Parse the modules in the app root.
        let (mut resources, binds) = {
            let loader = &self.pc.loader;
            let mut all_resources = Vec::new();
            let mut all_binds = Vec::new();

            // Keep track of the current service being parsed.
            let mut curr_service: Option<(PathBuf, String)> = None;

            for entry in walker {
                let entry = entry?;

                if entry.file_type().is_dir() {
                    // Is this directory outside the service directory?
                    // If so, unset the current service.
                    if let Some((service_dir, _)) = &curr_service {
                        if !entry.path().starts_with(service_dir) {
                            curr_service = None;
                        }
                    }
                    continue;
                }

                // Skip non-files.
                if !entry.file_type().is_file() {
                    continue;
                }

                let path = entry.path();

                // Skip non-".ts" files.
                let ext = entry.path().extension().and_then(OsStr::to_str);
                if !ext.is_some_and(|ext| ext == "ts") {
                    continue;
                }

                // Parse the module.
                let module = loader.load_fs_file(entry.path(), None)?;
                let service_name = curr_service.as_ref().map(|(_, name)| name.as_str());
                let (resources, binds) = self.pass1.parse(module, service_name)?;

                // Check if we should update the service being parsed.
                for res in &resources {
                    if let Resource::Service(svc) = res {
                        let parent = path.parent().expect("have a parent directory");
                        curr_service = Some((parent.to_path_buf(), svc.name.clone()));
                        break;
                    }
                }

                all_resources.extend(resources);
                all_binds.extend(binds);
            }

            (all_resources, all_binds)
        };

        // Resolve the initial binds.
        let mut binds = resolve_binds(&resources, binds)?;

        // Discover the services we have.
        let services = discover_services(&self.pc.file_set, &binds)?;

        // Inject additional binds for the generated services.
        let (additional_resources, additional_binds) =
            self.inject_generated_service_clients(&services);

        resources.extend(additional_resources);
        binds.extend(additional_binds);

        let resolver = UsageResolver::new(&self.pc.loader, &resources, &binds);
        let mut usages = Vec::new();

        for module in self.pc.loader.modules() {
            let exprs = resolver.scan_usage_exprs(&module)?;
            let u = resolver.resolve_usage(&exprs)?;
            usages.extend(u);
        }

        let services = collect_services(&self.pc.file_set, &binds, services);

        let result = ParseResult {
            resources,
            binds,
            usages,
            services,
        };

        Ok(result)
    }

    fn inject_generated_service_clients(
        &mut self,
        services: &[DiscoveredService],
    ) -> (Vec<Resource>, Vec<Lrc<Bind>>) {
        // Find the services we have
        let mut resources = Vec::new();
        let mut binds = Vec::new();

        let module = self.pc.loader.encore_app_clients();
        for svc in services {
            let client = Lrc::new(ServiceClient {
                service_name: svc.name.clone(),
            });
            let resource = Resource::ServiceClient(client.clone());
            resources.push(resource.clone());

            let id = self.pass1.alloc_bind_id();
            binds.push(Lrc::new(Bind {
                id,
                name: Some(svc.name.clone()),
                object: None,
                kind: BindKind::Create,
                resource,
                range: None,
                internal_bound_id: None,
                module_id: module.id,
            }));
        }

        (resources, binds)
    }
}

fn resolve_binds(resources: &[Resource], binds: Vec<UnresolvedBind>) -> Result<Vec<Lrc<Bind>>> {
    // Collect the resources we support by path.
    let resource_paths = resources
        .iter()
        .filter_map(|r| match r {
            Resource::SQLDatabase(db) => Some((
                ResourcePath::SQLDatabase {
                    name: db.name.clone(),
                },
                r,
            )),
            _ => None,
        })
        .collect::<HashMap<ResourcePath, &Resource>>();

    let mut result = Vec::with_capacity(binds.len());
    for b in binds {
        let resource: Resource = match b.resource {
            ResourceOrPath::Resource(res) => res,
            ResourceOrPath::Path(path) => {
                let res = resource_paths
                    .get(&path)
                    .ok_or_else(|| anyhow::anyhow!("resource not found: {:?}", path))?;
                (*res).to_owned()
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

/// Describes an Encore service.
#[derive(Debug)]
pub struct Service {
    pub name: String,

    /// The root directory of the service.
    pub root: PathBuf,

    /// The binds defined within the service.
    pub binds: Vec<Lrc<Bind>>,
}

fn collect_services(
    file_set: &FileSet,
    binds: &[Lrc<Bind>],
    discovered: Vec<DiscoveredService>,
) -> Vec<Service> {
    let mut services = Vec::with_capacity(discovered.len());
    for svc in discovered.into_iter() {
        services.push(Service {
            name: svc.name,
            root: svc.root,
            binds: vec![],
        });
    }

    // Sort the services by path so we can do a binary search below.
    services.sort_by(|a, b| a.root.cmp(&b.root));

    for b in binds {
        let Some(range) = b.range else { continue };
        let file_path = range.file(file_set);
        let path: &Path = match file_path {
            FilePath::Real(ref buf) => buf.as_path(),
            FilePath::Custom(_) => continue,
        };

        // found is where the bind would be inserted:
        // - Ok(idx) means an exact path match
        // - Err(idx) means where the path would be inserted.
        //   For this case we need to check if the path is a subdirectory of the service root.
        let found = services.binary_search_by_key(&path, |s| s.root.as_path());
        match found {
            Ok(idx) => services[idx].binds.push(b.clone()),
            Err(idx) => {
                // Is this path a subdirectory of the preceding service root?
                if idx > 0 {
                    let svc = &mut services[idx - 1];
                    if path.starts_with(&svc.root) {
                        svc.binds.push(b.clone());
                        continue;
                    }
                }
            }
        }
    }

    services
}
