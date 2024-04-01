use swc_common::sync::Lrc;

use crate::parser::module_loader::Module;
use crate::parser::parser::ParseContext;
use crate::testutil::JS_RUNTIME_PATH;
use assert_fs::TempDir;

pub fn test_parse(src: &str) -> Lrc<Module> {
    let root = TempDir::new().unwrap();
    let pc = ParseContext::new(root.to_path_buf(), JS_RUNTIME_PATH.as_path()).unwrap();
    pc.loader.inject_file("test.ts".into(), src).unwrap()
}
