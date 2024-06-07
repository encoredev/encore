use std::collections::HashMap;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use duct::cmd;
use serde::Deserialize;

use crate::builder::compile::CmdSpec;

#[derive(Deserialize)]
struct PackageJson {
    #[serde(default, rename = "packageManager")]
    package_manager: Option<String>,

    #[serde(default)]
    dependencies: HashMap<String, String>,
}

pub(super) fn resolve_package_manager(package_dir: &Path) -> Result<Box<dyn PackageManager>> {
    let package_json_path = package_dir.join("package.json");
    let package_json = std::fs::read_to_string(&package_json_path)
        .with_context(|| format!("failed to read {}", package_json_path.display()))?;
    let package_json: PackageJson = serde_json::from_str(&package_json)
        .with_context(|| format!("failed to parse {}", package_json_path.display()))?;

    let package_manager = package_json.package_manager.as_deref().unwrap_or("npm");
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
        _ => Err(anyhow::anyhow!(
            "unsupported package manager: {}",
            package_manager
        )),
    }
}

pub(super) trait PackageManager {
    fn setup_deps(&self, encore_dev_path: Option<&Path>) -> Result<()>;

    fn run_tests(&self) -> Result<CmdSpec>;

    #[allow(dead_code)]
    fn mgr_name(&self) -> &'static str;
}

struct NpmPackageManager {
    pkg_json: PackageJson,
    dir: PathBuf,
}

impl PackageManager for NpmPackageManager {
    fn setup_deps(&self, encore_dev_path: Option<&Path>) -> Result<()> {
        // If we don't have a node_modules folder, install everything.
        if !self.dir.join("node_modules").exists() {
            cmd!("npm", "install")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("npm install failed")?;
        }

        // If we don't have an `encore.dev` dependency, install it.
        if !self.pkg_json.dependencies.contains_key("encore.dev") {
            cmd!("npm", "install", "encore.dev@latest")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("npm install failed")?;
        }

        // If we have a local encore.dev path, symlink it.
        if let Some(encore_dev_path) = encore_dev_path {
            symlink_encore_dev(&self.dir, encore_dev_path)
                .context("unable to symlink local encore.dev")?;
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
    fn setup_deps(&self, encore_dev_path: Option<&Path>) -> Result<()> {
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

        // If we don't have an `encore.dev` dependency, install it.
        if !self.pkg_json.dependencies.contains_key("encore.dev") {
            cmd!("yarn", "add", "encore.dev@latest")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .context("yarn add failed")?;
        }

        // If we have a local encore.dev path, symlink it.
        if let Some(encore_dev_path) = encore_dev_path {
            symlink_encore_dev(&self.dir, encore_dev_path)
                .context("unable to symlink local encore.dev")?;
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

fn symlink_encore_dev(app_root: &Path, encore_dev_path: &Path) -> Result<()> {
    let node_modules = app_root.join("node_modules");
    let node_mod_dst = node_modules.join("encore.dev");

    // If the node_modules directory exists, symlink the encore.dev package.
    if let Ok(meta) = node_mod_dst.symlink_metadata() {
        // Is this a symlink?
        if meta.is_symlink() {
            // If the symlink is already pointing to our desired target, we're done.
            if let Ok(target) = std::fs::read_link(&node_mod_dst) {
                if target == encore_dev_path {
                    log::info!("encore.dev symlink already points to the local runtime, skipping.");
                    return Ok(());
                }
            }

            // It's a symlink pointing elsewhere. Remove it.
            symlink::remove_symlink_auto(&node_mod_dst).with_context(|| {
                format!("remove existing encore.dev symlink at {:?}", node_mod_dst)
            })?;
        } else {
            // It's not a symlink. Remove the directory so we can add a symlink.
            std::fs::remove_dir_all(&node_mod_dst).with_context(|| {
                format!("remove existing encore.dev directory at {:?}", node_mod_dst)
            })?;
        }
    }

    // Create the symlink if the node_modules directory exists.
    if node_modules.exists() {
        symlink::symlink_dir(encore_dev_path, &node_mod_dst)
            .with_context(|| format!("symlink encore.dev directory at {:?}", node_mod_dst))?;
    }

    Ok(())
}
