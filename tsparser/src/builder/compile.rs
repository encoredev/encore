use std::fs;
use std::path::{Path, PathBuf};

use crate::app::AppDesc;
use anyhow::{Context, Result};
use serde::Serialize;

use crate::builder::codegen::CodegenParams;
use crate::builder::transpiler::{
    EsbuildCompiler, ExternalPackages, Input, InputKind, OutputTranspiler, TranspileParams,
};
use crate::parser::parser::ParseContext;

use super::{App, Builder, DebugMode, NodeJSRuntime};

#[derive(Debug)]
pub struct CompileParams<'a> {
    pub app: &'a App,
    pub pc: &'a ParseContext,
    pub working_dir: &'a Path,
    pub desc: &'a AppDesc,
    pub debug: DebugMode,
    pub nodejs_runtime: NodeJSRuntime,
}

#[derive(Serialize, Debug)]
pub struct CompileResult {
    pub outputs: Vec<JSBuildOutput>,
}

#[derive(Serialize, Debug)]
pub struct JSBuildOutput {
    pub artifact_dir: PathBuf,
    pub entrypoints: Vec<Entrypoint>,
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
            app: params.app,
            pc: params.pc,
            working_dir: params.working_dir,
            desc: params.desc,
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
                params.desc.parse.services.len() + params.desc.meta.gateways.len(),
            );

            let service_names = params
                .desc
                .parse
                .services
                .iter()
                .map(|s| s.name.clone())
                .collect();
            let gateway_names = params
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

            inputs
        };

        let result = transpiler.transpile(TranspileParams {
            artifact_dir: build_dir.as_path(),
            cwd: params.app.root.as_path(),
            debug: params.debug,
            nodejs_runtime: params.nodejs_runtime,
            inputs,
        })?;

        Ok(CompileResult {
            outputs: vec![JSBuildOutput {
                artifact_dir: build_dir,
                entrypoints: result.entrypoints,
            }],
        })
    }
}
