use super::*;
use std::collections::HashMap;
use std::fs;
use std::io::{BufRead, Write};
use std::path::{Path, PathBuf};
use std::rc::Rc;

use crate::parser::module_loader::ModuleLoader;
use crate::parser::types::type_resolve::Ctx;
use crate::parser::{FilePath, FileSet};
use crate::testutil::testresolve::{NoopResolver, TestResolver};
use anyhow::Result;
use indexmap::IndexMap;
use insta::{assert_debug_snapshot, assert_snapshot, assert_yaml_snapshot, glob};
use itertools::Itertools;
use prost::Message;
use serde::{Deserialize, Serialize};
use swc_common::errors::{Handler, HANDLER};
use swc_common::{Globals, SourceMap, GLOBALS};
use tempdir::TempDir;

#[test]
fn resolve_types() {
    env_logger::init();
    glob!("testdata/*.ts", |path| {
        let globals = Globals::new();
        let errs = Rc::new(Handler::with_tty_emitter(
            swc_common::errors::ColorConfig::Auto,
            true,
            false,
            None,
        ));

        GLOBALS.set(&globals, || {
            HANDLER.set(&errs, || {
                let resolver = Box::new(NoopResolver);
                let cm: Rc<SourceMap> = Default::default();
                let file_set = FileSet::new(cm);
                let loader = Rc::new(ModuleLoader::new(errs.clone(), file_set.clone(), resolver));
                let resolve = ResolveState::new(loader.clone());

                let input = fs::read_to_string(path).unwrap();
                let file = loader
                    .inject_file(FilePath::Custom("test".to_string()), &input)
                    .unwrap();

                let module = resolve.get_or_init_module(file);

                let ctx = Ctx::new(&resolve, module.base.id);

                let result = module
                    .data
                    .named_exports
                    .iter()
                    .sorted_by(|a, b| a.1.range.cmp(&b.1.range))
                    .map(|(name, obj)| {
                        let typ = ctx.obj_type(&obj);
                        let typ = ctx.underlying(&typ).into_owned();
                        (name, typ)
                    })
                    .collect::<IndexMap<_, _>>();
                assert_debug_snapshot!(result);
            })
        })
    });
}
