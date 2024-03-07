use std::collections::HashMap;
use std::fs;
use std::path::Path;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

use crate::builder::codegen::CodegenParams;
use crate::builder::compile::CmdSpec;
use crate::builder::package_mgmt::resolve_package_manager;
use crate::parser::parser::ParseContext;

use super::parse::ParseResult;
use super::{App, Builder};

#[derive(Debug)]
pub struct TestParams<'a> {
    pub js_runtime_root: &'a Path,
    pub runtime_version: &'a String,
    pub app: &'a App,
    pub pc: &'a ParseContext<'a>,
    pub working_dir: &'a Path,
    pub parse: &'a ParseResult,
    pub use_local_runtime: bool,
}

#[derive(Serialize, Debug)]
pub struct TestResult {
    pub cmd: Option<CmdSpec>,
}

impl Builder<'_> {
    pub fn test(&self, params: &TestParams) -> Result<TestResult> {
        // Is there a "test" script defined in package.json?
        {
            #[derive(Deserialize)]
            struct PackageJson {
                #[serde(default)]
                scripts: HashMap<String, String>,
            }
            let package_json_path = params.app.root.join("package.json");
            let package_json = fs::read_to_string(&package_json_path)
                .with_context(|| format!("failed to read {}", package_json_path.display()))?;
            let package_json: PackageJson = serde_json::from_str(&package_json)
                .with_context(|| format!("failed to parse {}", package_json_path.display()))?;
            if !package_json.scripts.contains_key("test") {
                log::info!("no test script defined in package.json, skipping tests");
                return Ok(TestResult { cmd: None });
            }
        }

        self.generate_code(&CodegenParams {
            js_runtime_root: params.js_runtime_root,
            app: params.app,
            pc: params.pc,
            working_dir: params.working_dir,
            parse: params.parse,
        })
        .context("generate code")?;

        // Find the node_modules dir and the relative path back to the app root.
        let pkg_mgr =
            resolve_package_manager(&params.app.root).context("resolve package manager")?;

        let cmd = pkg_mgr.run_tests().context("test packages")?;

        Ok(TestResult { cmd: Some(cmd) })
    }
}
