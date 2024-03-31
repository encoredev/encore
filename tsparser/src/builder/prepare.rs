use std::path::Path;

use anyhow::{Context, Result};

use super::{Builder};

#[derive(Debug, Clone)]
pub struct PrepareParams<'a> {
    pub js_runtime_root: &'a Path,
    pub runtime_version: &'a str,
    pub app_root: &'a Path,
    pub use_local_runtime: bool,
}

impl Builder<'_> {
    pub fn prepare(&self, params: &PrepareParams) -> Result<()> {
        let encore_dev_path = params.js_runtime_root.join("encore.dev");
        self.setup_deps(params.app_root, Some(encore_dev_path.as_path()))
            .context("setup deps")?;

        Ok(())
    }
}
