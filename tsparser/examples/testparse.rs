use std::fmt;
use std::io::{self, Write};
use std::path::PathBuf;
use std::rc::Rc;
use std::sync::{Arc, Mutex};

use anyhow::Result;
use swc_common::errors::{Emitter, EmitterWriter, Handler, HANDLER};
use swc_common::{Globals, SourceMap, SourceMapper, GLOBALS};

use encore_tsparser::builder;
use encore_tsparser::builder::Builder;
use encore_tsparser::parser::parser::ParseContext;

fn main() -> Result<()> {
    env_logger::init();

    let js_runtime_path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("runtimes")
        .join("js");

    // Read the app root from the first arg.
    let app_root = PathBuf::from(std::env::args().nth(1).expect("missing app root"));

    let globals = Globals::new();

    let cm: Rc<SourceMap> = Default::default();
    let errors: Rc<Mutex<Vec<String>>> = Default::default();
    let emitter = ErrorList {
        cm: cm.clone(),
        errors: errors.clone(),
    };

    let errs = Rc::new(Handler::with_emitter(true, false, Box::new(emitter)));

    GLOBALS.set(&globals, || -> Result<()> {
        HANDLER.set(&errs, || -> Result<()> {
            let builder = Builder::new()?;
            let _parse: Option<(builder::App, builder::ParseResult)> = None;

            {
                let pp = builder::PrepareParams {
                    js_runtime_root: &js_runtime_path,
                    app_root: &app_root,
                };
                builder.prepare(&pp).unwrap();
            }

            let pc = ParseContext::new(
                app_root.clone(),
                js_runtime_path.clone(),
                cm.clone(),
                errs.clone(),
            )
            .unwrap();

            let app = builder::App {
                root: app_root.clone(),
                platform_id: None,
                local_id: "test".to_string(),
            };
            let pp = builder::ParseParams {
                app: &app,
                pc: &pc,
                working_dir: &app_root,
                parse_tests: false,
            };

            match builder.parse(&pp) {
                Ok(_) => {
                    println!("successfully parsed {}", app_root.display());
                    Ok(())
                }
                Err(err) => {
                    log::error!("failed to parse: {:?}", err);
                    // Get any errors from the emitter.
                    let errs = errors.lock().unwrap();
                    let mut err_msg = String::new();
                    for err in errs.iter() {
                        err_msg.push_str(err);
                        err_msg.push('\n');
                    }
                    err_msg.push_str(&format!("{:?}", err));
                    eprintln!("{}", err_msg);
                    anyhow::bail!("parse failure")
                }
            }
        })
    })
}

struct ErrorList {
    cm: Rc<dyn SourceMapper>,
    errors: Rc<Mutex<Vec<String>>>,
}

impl Emitter for ErrorList {
    fn emit(&mut self, db: &swc_common::errors::DiagnosticBuilder<'_>) {
        let buf: AtomicBuf = Default::default();

        let mut w = EmitterWriter::new(Box::new(buf.clone()), Some(self.cm.clone()), false, false);
        w.emit(db);

        let s = buf.to_string();
        self.errors.lock().unwrap().push(s);
    }
}

#[derive(Default, Clone)]
struct AtomicBuf(Arc<Mutex<Vec<u8>>>);

impl fmt::Display for AtomicBuf {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", String::from_utf8_lossy(&self.0.lock().unwrap()))
    }
}

impl Write for AtomicBuf {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        self.0.lock().unwrap().extend_from_slice(buf);
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}
