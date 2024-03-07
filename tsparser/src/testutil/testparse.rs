use swc_common::sync::Lrc;

use crate::parser::module_loader::Module;
use crate::parser::parser::ParseContext;

pub fn test_parse(src: &str) -> Lrc<Module> {
    let mut pc: ParseContext = Default::default();
    pc.loader.inject_file("test.ts".into(), src).unwrap()
}
