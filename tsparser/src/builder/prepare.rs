use std::path::PathBuf;

use anyhow::{Context, Result};

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

impl PackageVersion {
    pub fn is_installed(&self, ver: &str) -> bool {
        match self {
            Self::Local(want) => {
                if let Some(path) = ver.strip_prefix("file:") {
                    let got = PathBuf::from(path);
                    // Check for exact match or cleaned match.
                    got == *want || clean_path::clean(got) == clean_path::clean(want)
                } else {
                    false
                }
            }

            Self::Published(want) => {
                let ver = ver.trim_start_matches(['^', '=', '~']);

                // Check if the version is an exact match.
                if ver == want {
                    return true;
                }

                // Parse the version and check if it's equal or greater, semver-wise.
                let a = semver::Version::parse(&ver[1..]).ok();
                let b = semver::Version::parse(want).ok();
                if let (Some(a), Some(b)) = (a, b) {
                    a >= b
                } else {
                    false
                }
            }
        }
    }
}

impl Builder<'_> {
    pub fn prepare(&self, params: &PrepareParams) -> Result<()> {
        self.setup_deps(&params.app_root, &params.encore_dev_version)
            .context("setup deps")?;

        Ok(())
    }
}
