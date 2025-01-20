use std::collections::HashMap;
use std::fs;
use std::fs::DirBuilder;
use std::io::Write;
use std::path::{Component, Path, PathBuf};

use crate::app::AppDesc;
use anyhow::Context;
use clean_path::Clean;
use itertools::Itertools;
use serde::Serialize;
use serde_json::{json, Value};

use crate::builder::package_mgmt::resolve_package_manager;
use crate::parser::parser::ParseContext;
use crate::parser::resourceparser::bind::BindKind::Create;
use crate::parser::resources::apis::api::Methods;
use crate::parser::resources::Resource;
use crate::parser::{FilePath, Range};

use super::prepare::{PackageVersion, PrepareError};
use super::{App, Builder};

#[derive(Debug)]
pub struct CodegenParams<'a> {
    pub app: &'a App,
    pub pc: &'a ParseContext,
    pub working_dir: &'a Path,
    pub desc: &'a AppDesc,
}

pub struct CodegenResult {
    /// The path to node_modules.
    pub node_modules: PathBuf,
}

#[derive(Debug, Serialize)]
pub struct LinkResult {
    /// Whether the application is linked to the local runtime,
    /// as opposed to a published version.
    pub uses_local_runtime: bool,
}

impl Builder<'_> {
    pub fn setup_deps(
        &self,
        app_root: &Path,
        encore_dev_version: &PackageVersion,
    ) -> Result<(), PrepareError> {
        let pkg_mgr = resolve_package_manager(app_root)?;
        pkg_mgr.setup_deps(encore_dev_version)
    }

    pub fn generate_code(&self, params: &CodegenParams) -> Result<CodegenResult, PrepareError> {
        // Find the node_modules dir and the relative path back to the app root.
        let (node_modules, _rel_return_path) = self
            .find_node_modules_dir(&params.app.root)
            .ok_or(PrepareError::NodeModulesNotFound)?;

        let files = self.codegen_data(params)?;

        write_gen_encore_dir(&params.app.root, &files)?;

        Ok(CodegenResult { node_modules })
    }

    fn codegen_data(&self, params: &CodegenParams) -> Result<Vec<CodegenFile>, PrepareError> {
        let mut files = vec![];

        let mut auth_ctx = Vec::new();

        let get_svc_rel_path = |svc_root: &Path, range: Range, strip_ext: bool| -> String {
            match range.file(&params.pc.file_set) {
                FilePath::Real(mut buf) => {
                    if strip_ext {
                        buf.set_extension("");
                    }
                    let rel = match buf.strip_prefix(svc_root) {
                        Ok(p) => p,
                        Err(_) => &buf,
                    };

                    rel.to_owned()
                        .as_os_str()
                        .to_str()
                        .expect("unicode path")
                        .to_owned()
                }
                FilePath::Custom(str) => str,
            }
        };

        // Generate the files for each service.
        for svc in &params.desc.parse.services {
            let mut endpoints = Vec::new();
            let mut gateways = Vec::new();
            let mut subscriptions = Vec::new();
            let mut auth_handlers = Vec::new();
            let mut service = None;

            let svc_rel_path = params.app.rel_path_string(&svc.root)?;

            for b in &svc.binds {
                match &b.resource {
                    Resource::APIEndpoint(ep) => {
                        if ep.static_assets.is_some() {
                            continue; // Skip static assets.
                        }
                        endpoints.push(ep);
                    }
                    Resource::Gateway(gw) => {
                        let bind_name = b
                            .name
                            .as_deref()
                            .context("gateway objects must be assigned to a variable")
                            .map_err(PrepareError::Internal)?;
                        gateways.push((gw, bind_name));
                    }
                    Resource::PubSubSubscription(sub) => {
                        subscriptions.push(sub);
                    }
                    Resource::AuthHandler(ah) if b.kind == Create => {
                        auth_handlers.push(ah);
                    }
                    Resource::Service(svc) => {
                        service = Some(svc);
                    }
                    _ => {}
                }
            }

            // Add the auth handlers to the auth context.
            for ah in &auth_handlers {
                // Don't strip the extension here, so we support e.g. "auth.controller.ts" -> "auth.controller.js".
                let rel_path = get_svc_rel_path(&svc.root, ah.range, false);

                let import_path = Path::new("../../../")
                    .join(&svc_rel_path)
                    .join(rel_path)
                    .with_extension("js");
                auth_ctx.push(json!({
                    "name": ah.name,
                    "import_path": import_path,
                }));
            }

            // Service Main
            {
                let mut endpoint_ctx = Vec::new();
                let mut subscription_ctx = Vec::new();

                let service_ctx = service
                    .map(|service| {
                        let rel_path = get_svc_rel_path(&svc.root, service.range, true);
                        let import_path = Path::new("../../../../../")
                            .join(&svc_rel_path)
                            .join(rel_path);

                        json!({
                            "name": service.name,
                            "import_path": import_path,
                        })
                    })
                    .unwrap_or_else(|| json!({"name": svc.name}));

                for rpc in &endpoints {
                    let rel_path = get_svc_rel_path(&svc.root, rpc.range, true);
                    let import_path = Path::new("../../../../../")
                        .join(&svc_rel_path)
                        .join(rel_path);

                    endpoint_ctx.push(json!({
                        "name": rpc.name,
                        "raw": rpc.raw,
                        "streaming_request": rpc.streaming_request,
                        "streaming_response": rpc.streaming_response,
                        "import_path": import_path,
                        "service_name": svc.name,
                        "endpoint_options": json!({
                            "expose": rpc.expose,
                            "auth": rpc.require_auth,
                            "isRaw": rpc.raw,
                            "isStream": rpc.streaming_request || rpc.streaming_response,
                            "tags": rpc.tags,
                        }),
                    }));
                }

                for sub in &subscriptions {
                    let rel_path = get_svc_rel_path(&svc.root, sub.range, true);
                    let import_path = Path::new("../../../../../")
                        .join(&svc_rel_path)
                        .join(rel_path);

                    subscription_ctx.push(json!({
                        "topic_name": sub.topic.name,
                        "sub_name": sub.topic.name,
                        "import_path": import_path,
                    }));
                }

                let ctx = &json!({
                    "name": svc.name,
                    "endpoints": endpoint_ctx,
                    "subscriptions": subscription_ctx,
                    "service": service_ctx,
                });
                let main = self.entrypoint_service_main.render(&self.reg, ctx)?;

                files.push(CodegenFile {
                    path: PathBuf::from("internal")
                        .join("entrypoints")
                        .join("services")
                        .join(&svc.name)
                        .join("main")
                        .with_extension("ts"),
                    contents: main,
                });
            }

            // Gateway Main
            for (gw, bind_name) in &gateways {
                let name = &gw.name;

                // Compute the import path for the endpoint.
                let rel_path = get_svc_rel_path(&svc.root, gw.range, true);
                let import_path = Path::new("../../../../../")
                    .join(&svc_rel_path)
                    .join(rel_path);

                let ctx = &json!({
                    "name": name,
                    "gateways": [{
                        "encore_name": gw.name,
                        "bind_name": bind_name,
                        "import_path": import_path,
                    }],
                });
                let main = self.entrypoint_gateway_main.render(&self.reg, ctx)?;

                files.push(CodegenFile {
                    path: PathBuf::from("internal")
                        .join("entrypoints")
                        .join("gateways")
                        .join(name)
                        .join("main")
                        .with_extension("ts"),
                    contents: main,
                });
            }

            // Catalog client
            {
                let mut endpoint_ctx = Vec::new();
                let svc_rel_path = params.app.rel_path_string(&svc.root)?;
                // let node_modules_to_svc = node_modules_to_app_root.join(&svc_rel_path);

                let mut has_streams = false;

                for rpc in &endpoints {
                    if rpc.streaming_request || rpc.streaming_response {
                        has_streams = true;
                    }

                    // Don't strip the extension here, so we support e.g. "site.controller.ts" -> "site.controller.js".
                    let rel_path = get_svc_rel_path(&svc.root, rpc.range, false);

                    let import_path = Path::new("../../../../")
                        .join(&svc_rel_path)
                        .join(rel_path)
                        .with_extension("js");

                    endpoint_ctx.push(json!({
                        "name": rpc.name,
                        "raw": rpc.raw,
                        "streaming_request": rpc.streaming_request,
                        "streaming_response": rpc.streaming_response,
                        "import_path": import_path,
                        "endpoint_options": json!({
                            "expose": rpc.expose,
                            "auth": rpc.require_auth,
                            "isRaw": rpc.raw,
                            "isStream": rpc.streaming_request || rpc.streaming_response,
                            "tags": rpc.tags,
                        }),
                    }));
                }

                let service_ctx = service
                    .map(|service| {
                        let rel_path = get_svc_rel_path(&svc.root, service.range, true);
                        let import_path =
                            Path::new("../../../../").join(&svc_rel_path).join(rel_path);

                        json!({
                            "name": service.name,
                            "import_path": import_path,
                        })
                    })
                    .unwrap_or_else(|| json!({"name": svc.name}));

                let ctx = &json!({
                    "name": svc.name,
                    "endpoints": endpoint_ctx,
                    "has_streams": has_streams,
                    "service": service_ctx,
                });

                let service_d_ts = self.catalog_clients_service_d_ts.render(&self.reg, ctx)?;
                files.push(CodegenFile {
                    path: PathBuf::from("internal")
                        .join("clients")
                        .join(&svc.name)
                        .join("endpoints")
                        .with_extension("d.ts"),
                    contents: service_d_ts,
                });

                let service_js = self.catalog_clients_service_js.render(&self.reg, ctx)?;
                files.push(CodegenFile {
                    path: PathBuf::from("internal")
                        .join("clients")
                        .join(&svc.name)
                        .join("endpoints")
                        .with_extension("js"),
                    contents: service_js,
                });

                let service_testing_js = self
                    .catalog_clients_service_testing_js
                    .render(&self.reg, ctx)?;
                files.push(CodegenFile {
                    path: PathBuf::from("internal")
                        .join("clients")
                        .join(&svc.name)
                        .join("endpoints_testing")
                        .with_extension("js"),
                    contents: service_testing_js,
                });
            }
        }

        // Catalog Auth
        {
            let ctx = &json!({
               "auth_handlers": auth_ctx,
            });

            let index_d_ts = self.catalog_auth_index_ts.render(&self.reg, ctx)?;
            files.push(CodegenFile {
                path: PathBuf::from("auth").join("index").with_extension("ts"),
                contents: index_d_ts,
            });

            let auth_ts = self.catalog_auth_auth_ts.render(&self.reg, ctx)?;
            files.push(CodegenFile {
                path: PathBuf::from("internal")
                    .join("auth")
                    .join("auth")
                    .with_extension("ts"),
                contents: auth_ts,
            });
        }

        // Combined Main
        {
            let mut endpoint_ctx = Vec::new();
            let mut gateway_ctx = Vec::new();
            let mut subscription_ctx = Vec::new();
            let mut services_ctx = HashMap::new();

            for svc in &params.desc.parse.services {
                let mut endpoints = Vec::new();
                let mut gateways = Vec::new();
                let mut subscriptions = Vec::new();

                let svc_rel_path = params.app.rel_path_string(&svc.root)?;

                for b in &svc.binds {
                    match &b.resource {
                        Resource::APIEndpoint(ep) => {
                            if ep.static_assets.is_some() {
                                continue; // Skip static assets.
                            }
                            endpoints.push(ep);
                        }
                        Resource::Gateway(gw) => {
                            let bind_name = b
                                .name
                                .as_deref()
                                .context("gateway objects must be assigned to a variable")
                                .map_err(PrepareError::Internal)?;
                            gateways.push((gw, bind_name));
                        }
                        Resource::PubSubSubscription(sub) => {
                            subscriptions.push(sub);
                        }
                        Resource::Service(service) => {
                            let rel_path = get_svc_rel_path(&svc.root, service.range, true);
                            let import_path =
                                Path::new("../../../../").join(&svc_rel_path).join(rel_path);
                            services_ctx.insert(
                                svc.name.clone(),
                                json!({
                                    "name": service.name,
                                    "import_path": import_path,
                                }),
                            );
                        }
                        _ => {}
                    }
                }

                // Service Main
                for rpc in &endpoints {
                    if !services_ctx.contains_key(&svc.name) {
                        services_ctx.insert(svc.name.clone(), json!({"name": svc.name}));
                    }

                    let rel_path = get_svc_rel_path(&svc.root, rpc.range, true);
                    let import_path = Path::new("../../../../").join(&svc_rel_path).join(rel_path);

                    endpoint_ctx.push(json!({
                        "name": rpc.name,
                        "raw": rpc.raw,
                        "streaming_request": rpc.streaming_request,
                        "streaming_response": rpc.streaming_response,
                        "service_name": svc.name,
                        "import_path": import_path,
                        "endpoint_options": json!({
                            "expose": rpc.expose,
                            "auth": rpc.require_auth,
                            "isRaw": rpc.raw,
                            "isStream": rpc.streaming_request || rpc.streaming_response,
                            "tags": rpc.tags,
                        }),
                    }));
                }

                // Gateway Main
                for (gw, bind_name) in &gateways {
                    let _name = &gw.name;
                    let rel_path = get_svc_rel_path(&svc.root, gw.range, true);
                    let import_path = Path::new("../../../../").join(&svc_rel_path).join(rel_path);

                    gateway_ctx.push(json!({
                        "encore_name": gw.name,
                        "bind_name": bind_name,
                        "import_path": import_path,
                    }));
                }

                // Subscriptions
                for sub in &subscriptions {
                    let rel_path = get_svc_rel_path(&svc.root, sub.range, true);
                    let import_path = Path::new("../../../../").join(&svc_rel_path).join(rel_path);

                    subscription_ctx.push(json!({
                        "topic_name": sub.topic.name,
                        "sub_name": sub.topic.name,
                        "import_path": import_path,
                    }));
                }
            }

            let ctx = &json!({
                "endpoints": endpoint_ctx,
                "gateways": gateway_ctx,
                "subscriptions": subscription_ctx,
                "services": services_ctx,
            });
            let main = self.entrypoint_combined_main.render(&self.reg, ctx)?;

            files.push(CodegenFile {
                path: PathBuf::from("internal")
                    .join("entrypoints")
                    .join("combined")
                    .join("main")
                    .with_extension("ts"),
                contents: main,
            });
        }

        // Catalog Index
        {
            let mut services_ctx = Vec::new();
            for svc in &params.desc.parse.services {
                services_ctx.push(json!({
                    "name": svc.name,
                }));
            }

            let ctx = &json!({
                "services": services_ctx,
            });

            let index_js = self.catalog_clients_index_js.render(&self.reg, ctx)?;
            files.push(CodegenFile {
                path: PathBuf::from("clients").join("index").with_extension("js"),
                contents: index_js,
            });

            let index_d_ts = self.catalog_clients_index_d_ts.render(&self.reg, ctx)?;
            files.push(CodegenFile {
                path: PathBuf::from("clients")
                    .join("index")
                    .with_extension("d.ts"),
                contents: index_d_ts,
            });
        }

        let mut duplicates = files.iter().duplicates_by(|f| f.path.clone());
        if let Some(dup) = duplicates.next() {
            return Err(PrepareError::Internal(anyhow::anyhow!(
                "duplicate file path: {:?}",
                dup.path
            )));
        }

        Ok(files)
    }

    /// Find the node_modules_dir in parent directories of base,
    /// and at the same time compute the relative path from it to get
    /// back to base.
    pub fn find_node_modules_dir(&self, base: &Path) -> Option<(PathBuf, PathBuf)> {
        let pred = |p: &Path| p.join("node_modules").exists();
        let (ancestor, return_path) = find_ancestor(base, pred)?;

        let node_modules_dir = ancestor.join("node_modules").clean();

        // Prepend "../" to the return path since we're appending "node_modules" above.
        let return_path = Path::new("..").join(return_path).clean();
        Some((node_modules_dir, return_path))
    }
}

fn _http_methods(methods: &Methods) -> Value {
    match methods {
        Methods::All => json!("*"),
        Methods::Some(methods) => {
            let strs = methods.iter().map(|m| m.as_str()).collect::<Vec<_>>();
            json!(strs)
        }
    }
}

#[derive(Debug)]
pub struct CodegenFile {
    pub path: PathBuf,
    // relative to the node_modules/.encoredev folder
    pub contents: String,
}

fn write_gen_encore_dir(app_root: &Path, files: &[CodegenFile]) -> Result<(), PrepareError> {
    let base_dir = app_root.join("encore.gen");
    for f in files {
        if f.path.is_absolute() {
            return Err(PrepareError::Internal(anyhow::anyhow!(
                "path {:?} is not relative to the encore.gen folder",
                f.path
            )));
        }

        let file_path = base_dir.join(&f.path);
        // Create the parent of the file, if needed
        if let Some(parent) = file_path.parent() {
            DirBuilder::new()
                .recursive(true)
                .create(parent)
                .map_err(PrepareError::GenerateCode)?;
        }

        let file = fs::File::create(file_path).map_err(PrepareError::GenerateCode)?;
        let mut buf = std::io::BufWriter::new(file);
        buf.write_all(f.contents.as_bytes())
            .map_err(PrepareError::GenerateCode)?;
        buf.flush().map_err(PrepareError::GenerateCode)?;
    }

    Ok(())
}

/// Finds the first ancestor of base that satisfies predicate.
/// It returns the path to the ancestor (in the form "../../.." etc),
/// as well as the return path back to the base directory.
/// If predicate(base) is true, it reports (".", ".").
fn find_ancestor(base: &Path, predicate: fn(&Path) -> bool) -> Option<(PathBuf, PathBuf)> {
    let mut comps = base.components();
    let mut return_path = Vec::new();

    // Algorithm sketch as follows.
    // 1. Check if the predicate is satisfied on the current path. If true, return.
    // 2. Otherwise, remove the last component of the path, and add it to the return path.
    // 3. Add the component to the return path, and go to step 1.
    loop {
        let curr = comps.as_path();
        if predicate(curr) {
            // Compute the return path. If it's empty return ".".
            let return_path = if !return_path.is_empty() {
                // The components of the return path have been inserted in backwards order,
                // so reverse it now.
                return_path.iter().rev().collect::<PathBuf>()
            } else {
                PathBuf::from(".")
            };
            return Some((curr.to_path_buf(), return_path));
        }

        let comp = comps.next_back()?;

        match comp {
            Component::Normal(_) | Component::ParentDir => {
                return_path.push(comp);
            }

            // "." doesn't affect the predicate or the return path, so ignore it.
            Component::CurDir => {}

            // We've reached the beginning of the path and haven't found a match;
            // we're done.
            Component::Prefix(_) | Component::RootDir => return None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_find_ancestor() {
        {
            let pred = |p: &Path| p.ends_with("true");
            let base = Path::new("/foo/true/bar/baz");

            let (ancestor, return_path) = find_ancestor(base, pred).unwrap();
            assert_eq!(ancestor, Path::new("/foo/true"));
            assert_eq!(return_path, Path::new("bar/baz"));
        }

        {
            let pred = |_p: &Path| true;
            let base = Path::new("/foo/bar/baz");

            let (ancestor, return_path) = find_ancestor(base, pred).unwrap();
            assert_eq!(ancestor, Path::new("/foo/bar/baz"));
            assert_eq!(return_path, Path::new("."));
        }
    }
}
