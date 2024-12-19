use std::collections::HashMap;
use std::io;
use std::path::{Path, PathBuf};

use duct::cmd;
use serde::Deserialize;

use crate::builder::compile::CmdSpec;

use super::prepare::{PackageVersion, PrepareError};

#[derive(Deserialize)]
struct PackageJson {
    #[serde(default, rename = "packageManager")]
    package_manager: Option<String>,

    #[serde(default)]
    dependencies: HashMap<String, String>,
}

fn parse_package_json(package_json_path: &Path) -> Result<PackageJson, PrepareError> {
    let package_json =
        std::fs::read_to_string(package_json_path).map_err(PrepareError::ReadPackageJson)?;

    serde_json::from_str(&package_json).map_err(|source| PrepareError::InvalidPackageJson {
        source,
        path: package_json_path.to_path_buf(),
    })
}

#[derive(Debug, Clone)]
pub enum InstalledVersion {
    /// The package is not installed.
    NotInstalled,
    /// The package is installed but older than the required version.
    Older(String),
    /// The package is installed but different than the required version,
    /// in a way that cannot be compared (i.e. not semver but "local development" version).
    Different(String),
    /// The package is equal to the required version.
    Current,
    /// The package is newer than the required version.
    Newer(String),
}

impl PackageVersion {
    /// Reports whether the package is installed and if it is, whether it's the correct version.
    /// The `package_path` is needed to resolve local paths when running in development mode,
    /// since `npm install /path/to/package.json` rewrites it to a path relative from the package.json directory.
    pub fn is_installed(&self, ver: &str, package_path: &Path) -> InstalledVersion {
        use InstalledVersion::*;
        match self {
            Self::Local(want) => {
                if let Some(path) = ver.strip_prefix("file:") {
                    let got = PathBuf::from(path);

                    // Check for exact match or cleaned match.
                    if got == *want || clean_path::clean(&got) == clean_path::clean(want) {
                        return Current;
                    } else if got.is_relative() {
                        // Check if the paths are equal after resolving relative to the package.json directory.
                        let abs = package_path.join(&got);
                        if abs == *want || clean_path::clean(abs) == clean_path::clean(want) {
                            return Current;
                        }
                    }
                }

                Different(ver.to_string())
            }

            Self::Published(want) => {
                let got = ver.trim_start_matches(['^', '=', '~']);

                // Check if the version is an exact match.
                if got == want {
                    return Current;
                }

                // Parse the version and check if it's equal or greater, semver-wise.
                let installed = semver::Version::parse(got).ok();
                let want = semver::Version::parse(want).ok();
                if let (Some(installed), Some(want)) = (installed, want) {
                    use std::cmp::Ordering;
                    match installed.cmp(&want) {
                        Ordering::Less => Older(got.to_string()),
                        Ordering::Equal => Current,
                        Ordering::Greater => Newer(got.to_string()),
                    }
                } else {
                    Different(got.to_string())
                }
            }
        }
    }
}

fn find_workspace_package_manager(mut dir: PathBuf) -> Result<Option<String>, PrepareError> {
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

pub(super) fn resolve_package_manager(
    package_dir: &Path,
) -> Result<Box<dyn PackageManager>, PrepareError> {
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
        _ => Err(PrepareError::UnsupportedPackageManagerError(
            package_manager.to_string(),
        )),
    }
}

pub(super) trait PackageManager {
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<(), PrepareError>;

    fn run_tests(&self) -> CmdSpec;

    #[allow(dead_code)]
    fn mgr_name(&self) -> &'static str;
}

struct NpmPackageManager {
    pkg_json: PackageJson,
    dir: PathBuf,
}

impl PackageManager for NpmPackageManager {
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<(), PrepareError> {
        // If we don't have a node_modules folder, install everything.
        if !self.dir.join("node_modules").exists() {
            cmd!("npm", "install")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .map_err(PrepareError::InstallNodeModules)?;
        }

        // Install `encore.dev` if necessary
        let installed = self.pkg_json.dependencies.get("encore.dev");
        let v = installed.map_or(InstalledVersion::NotInstalled, |v| {
            encore_dev_version.is_installed(v, &self.dir)
        });

        match v {
            InstalledVersion::Current => Ok(()),
            InstalledVersion::Newer(v) => Err(PrepareError::EncoreDevTooNew(v)),
            InstalledVersion::Older(_)
            | InstalledVersion::Different(_)
            | InstalledVersion::NotInstalled => {
                let pkg = match encore_dev_version {
                    PackageVersion::Local(buf) => buf.display().to_string(),
                    PackageVersion::Published(ver) => format!("encore.dev@{ver}"),
                };
                cmd!("npm", "install", pkg)
                    .dir(&self.dir)
                    .stdout_to_stderr()
                    .run()
                    .map_err(PrepareError::InstallEncoreDev)?;
                Ok(())
            }
        }
    }

    fn run_tests(&self) -> CmdSpec {
        CmdSpec {
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
        }
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
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<(), PrepareError> {
        self.ensure_nodelinker()?;

        // If we don't have a node_modules folder, install everything.
        if !self.dir.join("node_modules").exists() {
            cmd!("yarn", "install")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .map_err(PrepareError::InstallNodeModules)?;
        }

        // Install `encore.dev` if necessary
        let installed = self.pkg_json.dependencies.get("encore.dev");
        let v = installed.map_or(InstalledVersion::NotInstalled, |v| {
            encore_dev_version.is_installed(v, &self.dir)
        });

        match v {
            InstalledVersion::Current => Ok(()),
            InstalledVersion::Newer(v) => Err(PrepareError::EncoreDevTooNew(v)),
            InstalledVersion::Older(_)
            | InstalledVersion::Different(_)
            | InstalledVersion::NotInstalled => {
                let pkg = match encore_dev_version {
                    PackageVersion::Local(buf) => buf.display().to_string(),
                    PackageVersion::Published(ver) => format!("encore.dev@{ver}"),
                };
                cmd!("yarn", "add", pkg)
                    .dir(&self.dir)
                    .stdout_to_stderr()
                    .run()
                    .map_err(PrepareError::InstallEncoreDev)?;
                Ok(())
            }
        }
    }

    fn run_tests(&self) -> CmdSpec {
        CmdSpec {
            command: vec!["yarn".to_string(), "run".to_string(), "test".to_string()],
            env: vec![],
            prioritized_files: vec![],
        }
    }

    fn mgr_name(&self) -> &'static str {
        "yarn"
    }
}

impl YarnPackageManager {
    /// Ensures the .yarnrc.yml file exists in the app root and has the nodeLinker set to "node-modules".
    fn ensure_nodelinker(&self) -> Result<(), PrepareError> {
        let file_path = self.dir.join(".yarnrc.yml");
        if !file_path.exists() {
            // Create the file with our desired contents.
            let content = "nodeLinker: node-modules\n";
            std::fs::write(&file_path, content).map_err(PrepareError::SetupYarnNodeLinker)?;
            return Ok(());
        }

        // Read the file as yaml.
        let contents =
            std::fs::read_to_string(&file_path).map_err(PrepareError::SetupYarnNodeLinker)?;
        let mut map = serde_yaml::from_str::<serde_yaml::Mapping>(&contents).map_err(|err| {
            PrepareError::SetupYarnNodeLinker(io::Error::new(io::ErrorKind::InvalidData, err))
        })?;

        // Modify the map and write it back out.
        map.insert(
            serde_yaml::Value::String("nodeLinker".into()),
            serde_yaml::Value::String("node-modules".into()),
        );
        let contents = serde_yaml::to_string(&map).map_err(|err| {
            PrepareError::SetupYarnNodeLinker(io::Error::new(io::ErrorKind::InvalidData, err))
        })?;
        std::fs::write(&file_path, contents).map_err(PrepareError::SetupYarnNodeLinker)?;

        Ok(())
    }
}

struct PnpmPackageManager {
    pkg_json: PackageJson,
    dir: PathBuf,
}

impl PackageManager for PnpmPackageManager {
    fn setup_deps(&self, encore_dev_version: &PackageVersion) -> Result<(), PrepareError> {
        // If we don't have a node_modules folder, install everything.
        if !self.dir.join("node_modules").exists() {
            cmd!("pnpm", "install")
                .dir(&self.dir)
                .stdout_to_stderr()
                .run()
                .map_err(PrepareError::InstallNodeModules)?;
        }

        // Install `encore.dev` if necessary
        let installed = self.pkg_json.dependencies.get("encore.dev");
        let v = installed.map_or(InstalledVersion::NotInstalled, |v| {
            encore_dev_version.is_installed(v, &self.dir)
        });

        match v {
            InstalledVersion::Current => Ok(()),
            InstalledVersion::Newer(v) => Err(PrepareError::EncoreDevTooNew(v)),
            InstalledVersion::Older(_)
            | InstalledVersion::Different(_)
            | InstalledVersion::NotInstalled => {
                let pkg = match encore_dev_version {
                    PackageVersion::Local(buf) => buf.display().to_string(),
                    PackageVersion::Published(ver) => format!("encore.dev@{ver}"),
                };
                cmd!("pnpm", "install", pkg)
                    .dir(&self.dir)
                    .stdout_to_stderr()
                    .run()
                    .map_err(PrepareError::InstallEncoreDev)?;
                Ok(())
            }
        }
    }

    fn run_tests(&self) -> CmdSpec {
        CmdSpec {
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
        }
    }

    fn mgr_name(&self) -> &'static str {
        "pnpm"
    }
}
