use std::rc::Rc;
use swc_common::sync::Lrc;

use crate::parser::module_loader::Module;
use crate::parser::parser::ParseContext;
use crate::testutil::JS_RUNTIME_PATH;
use assert_fs::TempDir;
use swc_common::errors::Handler;
use swc_common::SourceMap;

pub fn test_parse(src: &str) -> Lrc<Module> {
    let root = TempDir::new().unwrap();

    let cm: Rc<SourceMap> = Default::default();
    let errs = Rc::new(Handler::with_tty_emitter(
        swc_common::errors::ColorConfig::Auto,
        true,
        false,
        Some(cm.clone()),
    ));

    let pc = ParseContext::new(root.to_path_buf(), JS_RUNTIME_PATH.clone(), cm, errs).unwrap();
    pc.loader.inject_file("test.ts".into(), src).unwrap()
}
