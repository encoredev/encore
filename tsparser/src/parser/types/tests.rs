use super::*;
use std::fs;
use std::rc::Rc;

use crate::parser::parser::ParseContext;
use crate::parser::types::type_resolve::Ctx;
use crate::parser::FilePath;
use crate::testutil::testresolve::TestResolver;
use crate::testutil::JS_RUNTIME_PATH;
use indexmap::IndexMap;
use insta::{assert_debug_snapshot, glob};
use itertools::Itertools;
use object::Reexport;
use swc_common::errors::{Handler, HANDLER};
use swc_common::{Globals, SourceMap, GLOBALS};
use tempdir::TempDir;

#[test]
fn resolve_types() {
    // tracing_subscriber::fmt()
    //     .with_span_events(FmtSpan::ACTIVE)
    //     .init();
    env_logger::init();
    glob!("testdata/*", |path| {
        let globals = Globals::new();
        let errs = Rc::new(Handler::with_tty_emitter(
            swc_common::errors::ColorConfig::Auto,
            true,
            false,
            None,
        ));

        GLOBALS.set(&globals, || {
            HANDLER.set(&errs, || {
                let input = fs::read_to_string(path).unwrap();
                let ar = txtar::from_str(&input);
                let tmp_dir = TempDir::new("tsparser-test").unwrap();
                ar.materialize(&tmp_dir).unwrap();

                let resolver =
                    Box::new(TestResolver::new(tmp_dir.path().to_path_buf(), ar.clone()));
                let cm: Rc<SourceMap> = Default::default();

                let pc = ParseContext::with_resolver(
                    tmp_dir.path().to_path_buf(),
                    Some(JS_RUNTIME_PATH.clone()),
                    resolver,
                    cm,
                    errs.clone(),
                )
                .unwrap();

                let _mods = pc.loader.load_archive(tmp_dir.path(), &ar).unwrap();

                let file_name = FilePath::Real(tmp_dir.path().join("test.ts"));
                let module = pc.loader.inject_file(file_name, &ar.comment).unwrap();

                let resolve = ResolveState::new(pc.loader.clone());
                let module = resolve.get_or_init_module(module);
                let ctx = Ctx::new(&resolve, module.base.id);

                let mut result = module
                    .data
                    .named_exports
                    .iter()
                    .sorted_by(|a, b| a.1.range.cmp(&b.1.range))
                    .map(|(name, obj)| {
                        let typ = ctx.obj_type(obj);
                        let typ = ctx.underlying(&typ).into_owned();
                        (name, typ)
                    })
                    .collect::<IndexMap<_, _>>();

                let default_key = "default".to_string();
                if let Some(default_export) = module.data.default_export.as_ref() {
                    let typ = ctx.obj_type(default_export);
                    let typ = ctx.underlying(&typ).into_owned();
                    result.insert(&default_key, typ);
                }

                for re in &module.data.reexports {
                    match re {
                        Reexport::All { .. } => {}
                        Reexport::List { items, .. } => {
                            for it in items {
                                let export_name = it.renamed.as_ref().unwrap_or(&it.orig_name);
                                let obj = module
                                    .data
                                    .get_named_export(
                                        &resolve,
                                        &module.base.swc_file_path,
                                        export_name,
                                    )
                                    .unwrap();
                                let typ = ctx.obj_type(&obj);
                                let typ = ctx.underlying(&typ).into_owned();
                                result.insert(export_name, typ);
                            }
                        }
                    }
                }

                assert_debug_snapshot!(result);
            })
        })
    });
}
