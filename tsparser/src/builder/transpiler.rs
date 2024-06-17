use std::ffi::OsStr;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};

use crate::builder::compile::{CmdSpec, Entrypoint};

#[allow(dead_code)]
pub enum ExternalPackages<'a> {
    All,
    Some(&'a [&'a str]),
    None,
}

#[allow(dead_code)]
pub enum InputKind {
    Combined(Vec<String>, Vec<String>),
    Service(String),
    Gateway(String),
}

pub struct Input {
    // What kind of input is it.
    pub kind: InputKind,

    /// The path to the pre-compiled entrypoint of the service.
    pub entrypoint: PathBuf,
}

pub struct TranspileParams<'a> {
    /// Where the built artifacts should be written.
    pub artifact_dir: &'a Path,

    /// The working directory to use to run the transpiler,
    /// for generating useful error messages.
    pub cwd: &'a Path,

    /// The Encore CLI runtime version
    #[allow(dead_code)]
    pub runtime_version: &'a String,

    /// The services and gateways to transpile.
    pub inputs: Vec<Input>,
}

pub struct TranspileResult {
    /// The compiled entrypoints.
    pub entrypoints: Vec<Entrypoint>,
}

pub(super) trait OutputTranspiler {
    fn transpile(&self, params: TranspileParams) -> Result<TranspileResult>;
}

#[allow(dead_code)]
pub struct EsbuildCompiler<'a> {
    pub node_modules_dir: &'a Path,
    pub external: ExternalPackages<'a>,
}

impl OutputTranspiler for EsbuildCompiler<'_> {
    fn transpile(&self, p: TranspileParams) -> Result<TranspileResult> {
        let bundle = move |inputs: Vec<Input>, name_prefix| -> Result<Vec<Entrypoint>> {
            // let program = self.node_modules_dir.join(".bin").join("esbuild");
            let tsbundler_path = std::env::var("ENCORE_TSBUNDLER_PATH")
                .ok()
                .unwrap_or("tsbundler-encore".into());

            let mut cmd = std::process::Command::new(tsbundler_path);
            cmd.current_dir(p.cwd);
            cmd.arg("--bundle")
                .arg("--engine=node:21")
                // .arg("--format=esm")
                // .arg("--platform=node")
                // .arg("--sourcemap")
                // .arg("--out-extension:.js=.mjs")
                // .arg("--entry-names=[dir]/[name]")
                .arg(format!(
                    "--outdir={}",
                    p.artifact_dir.join(name_prefix).to_string_lossy(),
                ));

            for input in &inputs {
                cmd.arg(&input.entrypoint);
            }

            log::info!("running tsbundler-encore: {:?}", cmd);
            let output = cmd.output().context("failed to tsbundler-encore")?;
            if !output.status.success() {
                anyhow::bail!("failed to bundle: {}", String::from_utf8(output.stderr)?);
            }
            log::info!("successfully bundled");

            let mut entrypoints = Vec::new();
            for i in inputs {
                let entrypoint_path = {
                    let (file_stem, dir) = file_stem_and_dir(&i.entrypoint)?;
                    format!(
                        "$ARTIFACT_DIR/{}/{}/{}.mjs",
                        name_prefix,
                        dir.to_string_lossy(),
                        file_stem.to_string_lossy(),
                    )
                };

                let mut command = vec![
                    "node".to_string(),
                    "--enable-source-maps".into(),
                    "--preserve-symlinks".into(),
                ];

                // Finally we want to add the path to the bundled app
                command.push(entrypoint_path.clone());

                let (services, gateways) = match i.kind {
                    InputKind::Service(name) => (vec![name], vec![]),
                    InputKind::Gateway(name) => (vec![], vec![name]),
                    InputKind::Combined(gateways, services) => (services, gateways),
                };
                entrypoints.push(Entrypoint {
                    cmd: CmdSpec {
                        command,
                        env: vec![],
                        prioritized_files: vec![entrypoint_path],
                    },
                    services,
                    gateways,
                    use_runtime_config_v2: true,
                });
            }

            Ok(entrypoints)
        };

        let (service_inputs, gateway_inputs, combined_inputs) = {
            let mut service_inputs = Vec::new();
            let mut gateway_inputs = Vec::new();
            let mut combined_inputs = Vec::new();
            for i in p.inputs {
                match i.kind {
                    InputKind::Service(_) => service_inputs.push(i),
                    InputKind::Gateway(_) => gateway_inputs.push(i),
                    InputKind::Combined(_, _) => {
                        combined_inputs.push(i);
                    }
                }
            }
            (service_inputs, gateway_inputs, combined_inputs)
        };

        if !combined_inputs.is_empty() {
            let entrypoints = bundle(combined_inputs, "combined")?;
            Ok(TranspileResult { entrypoints })
        } else {
            let services = bundle(service_inputs, "services")?;
            let gateways = bundle(gateway_inputs, "gateways")?;
            let entrypoints = services.into_iter().chain(gateways).collect();

            Ok(TranspileResult { entrypoints })
        }
    }
}

pub struct BunBuildCompiler<'a> {
    #[allow(dead_code)]
    pub node_modules_dir: &'a Path,
    pub external: ExternalPackages<'a>,
}

impl OutputTranspiler for BunBuildCompiler<'_> {
    fn transpile(&self, p: TranspileParams) -> Result<TranspileResult> {
        let bundle = move |inputs: Vec<Input>, name_prefix| -> Result<Vec<Entrypoint>> {
            let mut cmd = std::process::Command::new("bun");
            cmd.current_dir(p.cwd);
            cmd.arg("build")
                .arg("--target=bun")
                .arg("--sourcemap")
                .arg("--format=esm")
                .arg("--entry-naming=[dir]/[name].ext")
                .arg(format!(
                    "--outdir={}",
                    p.artifact_dir.join(name_prefix).to_string_lossy(),
                ));

            match self.external {
                ExternalPackages::All => {
                    cmd.arg("--packages=external");
                }
                ExternalPackages::Some(pkgs) => {
                    for pkg in pkgs {
                        cmd.arg(format!("--external:{}", pkg));
                    }
                }
                ExternalPackages::None => {}
            }

            for input in &inputs {
                cmd.arg(&input.entrypoint);
            }

            log::info!("running bun build: {:?}", cmd);

            let output = cmd.output().context("failed to run bun build")?;
            if !output.status.success() {
                anyhow::bail!("failed to bundle: {}", String::from_utf8(output.stderr)?);
            }
            log::info!("successfully bundled");

            let mut entrypoints = Vec::new();
            for i in inputs {
                let entrypoint_path = {
                    let (file_stem, dir) = file_stem_and_dir(&i.entrypoint)?;
                    format!(
                        "$ARTIFACT_DIR/{}/{}/{}.mjs",
                        name_prefix,
                        dir.to_string_lossy(),
                        file_stem.to_string_lossy(),
                    )
                };

                let (services, gateways) = match i.kind {
                    InputKind::Service(name) => (vec![name], vec![]),
                    InputKind::Gateway(name) => (vec![], vec![name]),
                    InputKind::Combined(gateways, services) => (services, gateways),
                };
                entrypoints.push(Entrypoint {
                    cmd: CmdSpec {
                        command: vec!["bun".to_string(), entrypoint_path.clone()],
                        env: vec![],
                        prioritized_files: vec![entrypoint_path],
                    },
                    services,
                    gateways,
                    use_runtime_config_v2: true,
                });
            }

            Ok(entrypoints)
        };

        let (service_inputs, gateway_inputs, combined_inputs) = {
            let mut service_inputs = Vec::new();
            let mut gateway_inputs = Vec::new();
            let mut combined_inputs = Vec::new();
            for i in p.inputs {
                match i.kind {
                    InputKind::Service(_) => service_inputs.push(i),
                    InputKind::Gateway(_) => gateway_inputs.push(i),
                    InputKind::Combined(_, _) => {
                        combined_inputs.push(i);
                    }
                }
            }
            (service_inputs, gateway_inputs, combined_inputs)
        };

        if !combined_inputs.is_empty() {
            let entrypoints = bundle(combined_inputs, "combined")?;
            Ok(TranspileResult { entrypoints })
        } else {
            let services = bundle(service_inputs, "services")?;
            let gateways = bundle(gateway_inputs, "gateways")?;
            let entrypoints = services.into_iter().chain(gateways).collect();

            Ok(TranspileResult { entrypoints })
        }
    }
}

fn file_stem_and_dir(p: &Path) -> Result<(&OsStr, &OsStr)> {
    let file_stem = p.file_stem().context("no file name in entrypoint")?;
    let dir_name = p
        .parent()
        .and_then(|parent| parent.file_name())
        .context("no parent")?;
    Ok((file_stem, dir_name))
}
