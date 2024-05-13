use std::fs;
use std::io::{BufRead, Write};
use std::path::Path;

use anyhow::Result;
use insta::{assert_snapshot, glob};
use prost::Message;
use serde::{Deserialize, Serialize};
use swc_common::{Globals, GLOBALS};
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
        let _parse = parse_txtar(tmp_dir.path()).unwrap();
    });
}

fn parse_txtar(app_root: &Path) -> Result<ParseResult> {
    let globals = Globals::new();
    GLOBALS.set(&globals, || -> Result<ParseResult> {
        let builder = Builder::new()?;
        let js_runtime_path = js_runtime_path();

        let pc = ParseContext::new(app_root.to_path_buf(), &js_runtime_path)?;

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
}
