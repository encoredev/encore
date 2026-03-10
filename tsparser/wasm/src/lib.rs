use std::collections::HashMap;
use std::fmt;
use std::io::{self, Write};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use serde::{Deserialize, Serialize};
use swc_common::errors::{Emitter, EmitterWriter, Handler, HANDLER};
use swc_common::sync::Lrc;
use swc_common::{Globals, SourceMap, SourceMapper, GLOBALS};
use wasm_bindgen::prelude::*;

use encore_tsparser::app::validate_and_describe;
use encore_tsparser::parser::memory_resolver::InMemoryResolver;
use encore_tsparser::parser::parser::{ParseContext, Parser};
use encore_tsparser::parser::resourceparser::PassOneParser;
use encore_tsparser::tsconfig::TsConfigPathResolver;

#[wasm_bindgen(start)]
pub fn init_panic_hook() {
    console_error_panic_hook::set_once();
}

#[derive(Deserialize)]
struct InputFile {
    name: String,
    content: String,
}

#[derive(Serialize)]
struct ParseOutput {
    ok: bool,
    errors: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    meta: Option<serde_json::Value>,
}

/// Parse source files.
///
/// `files_json`: JSON array of `[{name, content}]`.
/// Files with paths starting with `node_modules/` are treated as dependencies
/// (registered for module resolution but not parsed as user code).
#[wasm_bindgen]
pub fn parse(files_json: &str) -> String {
    let all_files: Vec<InputFile> = match serde_json::from_str(files_json) {
        Ok(f) => f,
        Err(e) => {
            return error_output(&format!("invalid input JSON: {e}"));
        }
    };

    let (nm_files, user_files): (Vec<_>, Vec<_>) = all_files
        .into_iter()
        .partition(|f| f.name.starts_with("node_modules/"));

    run_parse(user_files, nm_files)
}

fn run_parse(files: Vec<InputFile>, nm_files: Vec<InputFile>) -> String {
    let globals = Globals::new();
    let cm: Lrc<SourceMap> = Default::default();
    let errors: Arc<Mutex<Vec<String>>> = Default::default();

    let emitter = WasmErrorEmitter {
        cm: cm.clone(),
        errors: errors.clone(),
    };
    let errs = Lrc::new(Handler::with_emitter(true, false, Box::new(emitter)));

    let result = GLOBALS.set(&globals, || {
        HANDLER.set(&errs, || {
            let app_root = PathBuf::from("/app");

            // Build file paths for all files (user + node_modules)
            let user_file_paths: Vec<PathBuf> =
                files.iter().map(|f| app_root.join(&f.name)).collect();
            let nm_file_paths: Vec<PathBuf> =
                nm_files.iter().map(|f| app_root.join(&f.name)).collect();

            let all_file_paths: Vec<PathBuf> = user_file_paths
                .iter()
                .chain(nm_file_paths.iter())
                .cloned()
                .collect();

            // Create resolver and register package.json files
            let mut resolver = InMemoryResolver::new(app_root.clone(), all_file_paths);

            for f in &nm_files {
                if f.name.ends_with("package.json") {
                    if let Ok(value) = serde_json::from_str::<serde_json::Value>(&f.content) {
                        let path = app_root.join(&f.name);
                        resolver.register_package_json(path, value);
                    }
                }
            }

            // Set up tsconfig path aliases if tsconfig.json is provided
            if let Some(tsconfig_file) = files.iter().find(|f| f.name == "tsconfig.json") {
                match TsConfigPathResolver::from_str(&app_root, &tsconfig_file.content) {
                    Ok(tsconfig) => resolver.set_tsconfig(tsconfig),
                    Err(e) => errors
                        .lock()
                        .unwrap()
                        .push(format!("failed to parse tsconfig.json: {e}")),
                }
            }

            let pc = match ParseContext::with_boxed_resolver(
                app_root.clone(),
                Box::new(resolver),
                cm.clone(),
                errs.clone(),
            ) {
                Ok(pc) => pc,
                Err(e) => {
                    errors
                        .lock()
                        .unwrap()
                        .push(format!("failed to create parse context: {e}"));
                    return None::<serde_json::Value>;
                }
            };

            // Register node_modules file contents with the module loader
            if !nm_files.is_empty() {
                let nm_contents: HashMap<PathBuf, String> = nm_files
                    .iter()
                    .map(|f| (app_root.join(&f.name), f.content.clone()))
                    .collect();
                pc.loader.register_file_contents(nm_contents);
            }

            let pass1 = PassOneParser::new(
                pc.file_set.clone(),
                pc.type_checker.clone(),
                Default::default(),
            );
            let parser = Parser::new(&pc, pass1);

            let file_data: Vec<(PathBuf, String)> = files
                .iter()
                .map(|f| (PathBuf::from(&f.name), f.content.clone()))
                .collect();

            let parse_result = parser.parse_from_files(file_data);
            let app_desc = validate_and_describe(&pc, parse_result);

            app_desc.and_then(|desc| serde_json::to_value(&desc.meta).ok())
        })
    });

    let collected_errors: Vec<String> = errors.lock().unwrap().clone();
    format_output(result, collected_errors)
}

fn error_output(msg: &str) -> String {
    serde_json::to_string(&ParseOutput {
        ok: false,
        errors: vec![msg.to_string()],
        meta: None,
    })
    .unwrap()
}

fn format_output(meta: Option<serde_json::Value>, collected_errors: Vec<String>) -> String {
    let output = ParseOutput {
        ok: collected_errors.is_empty() && meta.is_some(),
        errors: collected_errors,
        meta,
    };

    serde_json::to_string(&output).unwrap()
}

struct WasmErrorEmitter {
    cm: Lrc<dyn SourceMapper>,
    errors: Arc<Mutex<Vec<String>>>,
}

impl Emitter for WasmErrorEmitter {
    fn emit(&mut self, db: &swc_common::errors::DiagnosticBuilder<'_>) {
        let buf: WriteBuf = Default::default();
        let mut w = EmitterWriter::new(Box::new(buf.clone()), Some(self.cm.clone()), false, false);
        w.emit(db);
        let s = buf.to_string();
        self.errors.lock().unwrap().push(s);
    }
}

#[derive(Default, Clone)]
struct WriteBuf(Arc<Mutex<Vec<u8>>>);

impl fmt::Display for WriteBuf {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", String::from_utf8_lossy(&self.0.lock().unwrap()))
    }
}

impl Write for WriteBuf {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        self.0.lock().unwrap().extend_from_slice(buf);
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}
