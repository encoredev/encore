use std::{io, path::PathBuf};

use thiserror::Error;

use super::Builder;

#[derive(Debug, Clone)]
pub struct PrepareParams {
    pub encore_dev_version: PackageVersion,
    pub app_root: PathBuf,
}

#[derive(Debug, Clone)]
pub enum PackageVersion {
    Local(PathBuf),
    Published(String),
}

#[derive(Debug, Error)]
pub enum PrepareError {
    #[error("package.json file not found (expected at {0})")]
    PackageJsonNotFound(PathBuf),
    #[error("failed to read package.json: {0}")]
    ReadPackageJson(#[source] io::Error),
    #[error("failed to update package.json: {0}")]
    WritePackageJson(#[source] io::Error),
    #[error("invalid package.json: {source}")]
    InvalidPackageJson {
        source: serde_json::Error,
        path: PathBuf,
    },
    #[error("package manager '{0}' not supported")]
    UnsupportedPackageManagerError(String),

    #[error(
        "installed 'encore.dev' package version ({0}) newer than 'encore' release, run 'encore version update' first"
    )]
    EncoreDevTooNew(String),

    #[error("installing node_modules failed: {0}, run 'npm install' manually to see the error")]
    InstallNodeModules(#[source] io::Error),

    #[error("failed to install 'encore.dev' package: {0}")]
    InstallEncoreDev(#[source] io::Error),

    #[error("failed to configure yarn nodeLinker to node-modules: {0}")]
    SetupYarnNodeLinker(#[source] io::Error),

    #[error("node_modules directory not found")]
    NodeModulesNotFound,

    #[error("unable to generate code")]
    GenerateCode(#[source] io::Error),

    #[error("internal error: {0}")]
    Internal(#[source] anyhow::Error),
}

impl Builder<'_> {
    pub fn prepare(&self, params: &PrepareParams) -> Result<(), PrepareError> {
        self.setup_deps(&params.app_root, &params.encore_dev_version)
    }
}
