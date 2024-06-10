use std::fs;
use std::path::Path;
use std::rc::Rc;

use anyhow::Result;
use insta::glob;
use swc_common::errors::{Handler, HANDLER};
use swc_common::{Globals, SourceMap, GLOBALS};
use tempdir::TempDir;

use encore_tsparser::builder;
use encore_tsparser::builder::{Builder, ParseResult};
use encore_tsparser::parser::parser::ParseContext;

use crate::common::js_runtime_path;

mod common;

#[test]
fn test_parser() {
    env_logger::init();
    glob!("testdata/*.txt", |path| {
        let input = fs::read_to_string(path).unwrap();
        let ar = txtar::from_str(&input);
        let tmp_dir = TempDir::new("parse").unwrap();
        ar.materialize(&tmp_dir).unwrap();
        match parse_txtar(tmp_dir.path()) {
            Ok(_) => {}
            Err(e) => {
                panic!("{:#?}\n{}", e, e.backtrace());
            }
        }
    });
}

fn parse_txtar(app_root: &Path) -> Result<ParseResult> {
    let globals = Globals::new();
    let cm: Rc<SourceMap> = Default::default();
    let errs = Rc::new(Handler::with_tty_emitter(
        swc_common::errors::ColorConfig::Auto,
        true,
        false,
        Some(cm.clone()),
    ));

    GLOBALS.set(&globals, || -> Result<ParseResult> {
        HANDLER.set(&errs, || -> Result<ParseResult> {
            let builder = Builder::new()?;
            let js_runtime_path = js_runtime_path();

            let pc = ParseContext::new(
                app_root.to_path_buf(),
                js_runtime_path.clone(),
                cm,
                errs.clone(),
            )?;

            let app = builder::App {
                root: app_root.to_path_buf(),
                platform_id: None,
                local_id: "test".to_string(),
            };
            let pp = builder::ParseParams {
                app: &app,
                pc: &pc,
                working_dir: app_root,
                parse_tests: false,
            };

            builder.parse(&pp)
        })
    })
}
