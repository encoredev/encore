use std::collections::HashMap;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use duct::cmd;
use serde::Deserialize;

use crate::builder::compile::CmdSpec;

use super::prepare::PackageVersion;

#[derive(Deserialize)]
struct PackageJson {
    #[serde(default, rename = "packageManager")]
    package_manager: Option<String>,

    #[serde(default)]
    dependencies: HashMap<String, String>,
}

fn parse_package_json(package_json_path: &Path) -> Result<PackageJson> {
    let package_json = std::fs::read_to_string(package_json_path)
        .with_context(|| format!("failed to read {}", package_json_path.display()))?;

    serde_json::from_str(&package_json)
        .with_context(|| format!("failed to parse {}", package_json_path.display()))
}

fn find_workspace_package_manager(mut dir: PathBuf) -> Result<Option<String>> {
    while dir.pop() {
        let package_json_path = dir.join("package.json");
        if package_json_path.exists() {
            let package_json = parse_package_json(&package_json_path)?;
            if let Some(pm) = package_json.package_manager.as_deref() {
                if !pm.is_empty() {
                    return Ok(package_json.package_manager);
                }
            }
        }
    }

    Ok(None)
}

pub(super) fn resolve_package_manager(package_dir: &Path) -> Result<Box<dyn PackageManager>> {
    let package_json_path = package_dir.join("package.json");
    let package_json = parse_package_json(&package_json_path)?;

    let package_manager = match package_json.package_manager {
        Some(ref pm) if !pm.is_empty() => Some(pm.clone()),
        _ => find_workspace_package_manager(package_dir.to_path_buf())?,
    };

    let package_manager = package_manager.as_deref().unwrap_or("npm");
    let package_manager = match package_manager.split_once('@') {
        Some((name, _)) => name,
        None => package_manager,
    };

    match package_manager.to_lowercase().as_ref() {
        "npm" => Ok(Box::new(NpmPackageManager {
            pkg_json: package_json,
            dir: package_dir.to_path_buf(),
        })),
        "yarn" => Ok(Box::new(YarnPackageManager {
            pkg_json: package_json,
            dir: package_dir.to_path_buf(),
        })),
        "pnpm" => Ok(Box::new(PnpmPackageManager {
            pkg_json: package_json,
            dir: package_dir.to_path_buf(),
        })),
        _ => Err(anyhow::anyhow!(
            "unsupported package manager: {}",
            package_manager
        )),
    }
}

pub(super) trait PackageManager {
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<()>;

    fn run_tests(&self) -> Result<CmdSpec>;

    #[allow(dead_code)]
    fn mgr_name(&self) -> &'static str;
}

struct NpmPackageManager {
    pkg_json: PackageJson,
    dir: PathBuf,
}

impl PackageManager for NpmPackageManager {
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<()> {
        // If we don't have a node_modules folder, install everything.
        if !self.dir.join("node_modules").exists() {
            cmd!("npm", "install")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("npm install failed")?;
        }

        // Install `encore.dev` if necessary
        let installed = self.pkg_json.dependencies.get("encore.dev");
        if installed.is_none_or(|v| !encore_dev_version.is_installed(v)) {
            log::info!("not installed, installing");
            let pkg = match encore_dev_version {
                PackageVersion::Local(buf) => buf.display().to_string(),
                PackageVersion::Published(ver) => format!("encore.dev@{ver}"),
            };
            cmd!("npm", "install", pkg)
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("npm install failed")?;
        }

        Ok(())
    }

    fn run_tests(&self) -> Result<CmdSpec> {
        Ok(CmdSpec {
            command: vec![
                "npm".to_string(),
                "run".to_string(),
                "test".to_string(),
                // Specify '--' so that additional arguments added from the test runner
                // aren't interpreted by npm.
                "--".to_string(),
            ],
            env: vec![],
            prioritized_files: vec![],
        })
    }

    fn mgr_name(&self) -> &'static str {
        "npm"
    }
}

struct YarnPackageManager {
    pkg_json: PackageJson,
    dir: PathBuf,
}

impl PackageManager for YarnPackageManager {
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<()> {
        self.ensure_nodelinker()
            .context("unable to update .yarnrc.yml to set nodeLinker")?;

        // If we don't have a node_modules folder, install everything.
        if !self.dir.join("node_modules").exists() {
            cmd!("yarn", "install")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("yarn install failed")?;
        }

        // Install `encore.dev` if necessary
        let installed = self.pkg_json.dependencies.get("encore.dev");
        if installed.is_none_or(|v| !encore_dev_version.is_installed(v)) {
            log::info!("not installed, installing");
            let pkg = match encore_dev_version {
                PackageVersion::Local(buf) => buf.display().to_string(),
                PackageVersion::Published(ver) => format!("encore.dev@{ver}"),
            };
            cmd!("yarn", "add", pkg,)
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("npm install failed")?;
        }

        Ok(())
    }

    fn run_tests(&self) -> Result<CmdSpec> {
        Ok(CmdSpec {
            command: vec!["yarn".to_string(), "run".to_string(), "test".to_string()],
            env: vec![],
            prioritized_files: vec![],
        })
    }

    fn mgr_name(&self) -> &'static str {
        "yarn"
    }
}

impl YarnPackageManager {
    /// Ensures the .yarnrc.yml file exists in the app root and has the nodeLinker set to "node-modules".
    fn ensure_nodelinker(&self) -> Result<()> {
        let file_path = self.dir.join(".yarnrc.yml");
        if !file_path.exists() {
            // Create the file with our desired contents.
            let content = "nodeLinker: node-modules\n";
            std::fs::write(&file_path, content)
                .with_context(|| format!("failed to write {}", file_path.display()))?;
            return Ok(());
        }

        // Read the file as yaml.
        let contents = std::fs::read_to_string(&file_path)
            .with_context(|| format!("failed to read {}", file_path.display()))?;
        let mut map = serde_yaml::from_str::<serde_yaml::Mapping>(&contents)
            .with_context(|| format!("failed to parse {}", file_path.display()))?;

        // Modify the map and write it back out.
        map.insert(
            serde_yaml::Value::String("nodeLinker".into()),
            serde_yaml::Value::String("node-modules".into()),
        );
        let contents = serde_yaml::to_string(&map)
            .with_context(|| format!("failed to serialize {}", file_path.display()))?;
        std::fs::write(&file_path, contents)
            .with_context(|| format!("failed to write {}", file_path.display()))?;

        Ok(())
    }
}

struct PnpmPackageManager {
    pkg_json: PackageJson,
    dir: PathBuf,
}

impl PackageManager for PnpmPackageManager {
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<()> {
        // If we don't have a node_modules folder, install everything.
        if !self.dir.join("node_modules").exists() {
            cmd!("pnpm", "install")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("pnpm install failed")?;
        }

        // Install `encore.dev` if necessary
        let installed = self.pkg_json.dependencies.get("encore.dev");
        if installed.is_none_or(|v| !encore_dev_version.is_installed(v)) {
            log::info!("not installed, installing");
            let pkg = match encore_dev_version {
                PackageVersion::Local(buf) => buf.display().to_string(),
                PackageVersion::Published(ver) => format!("encore.dev@{ver}"),
            };
            cmd!("pnpm", "install", pkg)
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("npm install failed")?;
        }

        Ok(())
    }

    fn run_tests(&self) -> Result<CmdSpec> {
        Ok(CmdSpec {
            command: vec![
                "pnpm".to_string(),
                "run".to_string(),
                "test".to_string(),
                // Specify '--' so that additional arguments added from the test runner
                // aren't interpreted by npm.
                "--".to_string(),
            ],
            env: vec![],
            prioritized_files: vec![],
        })
    }

    fn mgr_name(&self) -> &'static str {
        "pnpm"
    }
}
