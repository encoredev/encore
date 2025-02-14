use std::collections::HashMap;
use std::fs;
use std::path::Path;

use crate::app::AppDesc;
use serde::{Deserialize, Serialize};

use crate::builder::codegen::CodegenParams;
use crate::builder::compile::CmdSpec;
use crate::builder::package_mgmt::resolve_package_manager;
use crate::parser::parser::ParseContext;

use super::{prepare::PrepareError, App, Builder};

#[derive(Debug)]
pub struct TestParams<'a> {
    pub app: &'a App,
    pub pc: &'a ParseContext,
    pub working_dir: &'a Path,
    pub parse: &'a AppDesc,
}

#[derive(Serialize, Debug)]
pub struct TestResult {
    pub cmd: Option<CmdSpec>,
}

impl Builder<'_> {
    pub fn test(&self, params: &TestParams) -> Result<TestResult, PrepareError> {
        // Is there a "test" script defined in package.json?
        {
            #[derive(Deserialize)]
            struct PackageJson {
                #[serde(default)]
                scripts: HashMap<String, String>,
            }
            let package_json_path = params.app.root.join("package.json");
            let package_json =
                fs::read_to_string(&package_json_path).map_err(PrepareError::ReadPackageJson)?;
            let package_json: PackageJson =
                serde_json::from_str(&package_json).map_err(|source| {
                    PrepareError::InvalidPackageJson {
                        source,
                        path: package_json_path.to_path_buf(),
                    }
                })?;
            if !package_json.scripts.contains_key("test") {
                log::info!("no test script defined in package.json, skipping tests");
                return Ok(TestResult { cmd: None });
            }
        }

        self.generate_code(&CodegenParams {
            app: params.app,
            pc: params.pc,
            working_dir: params.working_dir,
            desc: params.parse,
        })?;

        let pkg_mgr = resolve_package_manager(&params.app.root)?;
        let cmd = pkg_mgr.run_tests();
        Ok(TestResult { cmd: Some(cmd) })
    }
}
