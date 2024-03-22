use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use serde::Serialize;

use crate::builder::codegen::CodegenParams;
use crate::builder::transpiler::{
    EsbuildCompiler, ExternalPackages, Input, InputKind, OutputTranspiler, TranspileParams,
};
use crate::parser::parser::ParseContext;

use super::parse::ParseResult;
use super::{App, Builder};

#[derive(Debug)]
pub struct CompileParams<'a> {
    pub js_runtime_root: &'a Path,
    pub runtime_version: &'a String,
    pub app: &'a App,
    pub pc: &'a ParseContext<'a>,
    pub working_dir: &'a Path,
    pub parse: &'a ParseResult,
    pub use_local_runtime: bool,
}

#[derive(Serialize, Debug)]
pub struct CompileResult {
    pub outputs: Vec<JSBuildOutput>,
}

#[derive(Serialize, Debug)]
pub struct JSBuildOutput {
    pub node_modules: Option<PathBuf>,
    pub package_json: PathBuf,
    pub artifact_dir: PathBuf,
    pub entrypoints: Vec<Entrypoint>,
    pub uses_local_runtime: bool,
}

#[derive(Serialize, Debug)]
pub struct Entrypoint {
    pub cmd: CmdSpec,
    pub services: Vec<String>,
    pub gateways: Vec<String>,
    pub use_runtime_config_v2: bool,
}

pub type ArtifactString = String;

#[derive(Serialize, Debug)]
pub struct CmdSpec {
    pub command: Vec<ArtifactString>,
    pub env: Vec<ArtifactString>,
    pub prioritized_files: Vec<ArtifactString>,
}

impl Builder<'_> {
    pub fn compile(&self, params: &CompileParams) -> Result<CompileResult> {
        self.generate_code(&CodegenParams {
            js_runtime_root: params.js_runtime_root,
            app: params.app,
            pc: params.pc,
            working_dir: params.working_dir,
            parse: params.parse,
        })
        .context("generate code")?;

        let build_dir = params.app.root.join(".encore").join("build");
        fs::create_dir_all(&build_dir)?;

        let (node_modules, _) = self
            .find_node_modules_dir(params.app.root.as_path())
            .ok_or_else(|| anyhow::anyhow!("could not find node_modules directory"))?;

        let transpiler = EsbuildCompiler {
            node_modules_dir: node_modules.as_path(),
            external: ExternalPackages::All,
        };

        let inputs = {
            let mut inputs = Vec::with_capacity(
                params.parse.desc.services.len() + params.parse.desc.meta.gateways.len(),
            );

            let service_names = params
                .parse
                .desc
                .services
                .iter()
                .map(|s| s.name.clone())
                .collect();
            let gateway_names = params
                .parse
                .desc
                .meta
                .gateways
                .iter()
                .map(|g| g.encore_name.clone())
                .collect();

            inputs.push(Input {
                kind: InputKind::Combined(gateway_names, service_names),
                entrypoint: params
                    .app
                    .root
                    .join("encore.gen")
                    .join("internal")
                    .join("entrypoints")
                    .join("combined")
                    .join("main.ts"),
            });

            // for svc in &params.parse.desc.services {
            //     inputs.push(Input {
            //         kind: InputKind::Service(svc.name.clone()),
            //         entrypoint: params
            //             .app
            //             .root
            //             .join("encore.gen")
            //             .join("internal")
            //             .join("entrypoints")
            //             .join("services")
            //             .join(&svc.name)
            //             .join("main.ts"),
            //     });
            // }
            //
            // for gw in &params.parse.desc.meta.gateways {
            //     let name = &gw.encore_name;
            //     inputs.push(Input {
            //         kind: InputKind::Gateway(name.to_string()),
            //         entrypoint: params
            //             .app
            //             .root
            //             .join("encore.gen")
            //             .join("internal")
            //             .join("entrypoints")
            //             .join("gateways")
            //             .join(name)
            //             .join("main.ts"),
            //     });
            // }

            inputs
        };

        let result = transpiler.transpile(TranspileParams {
            artifact_dir: build_dir.as_path(),
            runtime_version: params.runtime_version,
            cwd: params.app.root.as_path(),
            inputs,
        })?;

        Ok(CompileResult {
            outputs: vec![JSBuildOutput {
                node_modules: Some(node_modules),
                package_json: params.app.root.join("package.json"),
                artifact_dir: build_dir,
                entrypoints: result.entrypoints,
                uses_local_runtime: params.use_local_runtime,
            }],
        })
    }
}
