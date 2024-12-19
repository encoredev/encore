use std::fmt;
use std::io::{self, Write};
use std::path::PathBuf;
use std::rc::Rc;
use std::sync::{Arc, Mutex};

use swc_common::errors::{Emitter, EmitterWriter, Handler, HANDLER};
use swc_common::{Globals, SourceMap, SourceMapper, GLOBALS};

use encore_tsparser::builder;
use encore_tsparser::builder::Builder;
use encore_tsparser::parser::parser::ParseContext;

fn main() {
    env_logger::init();

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

    GLOBALS.set(&globals, || {
        HANDLER.set(&errs, || {
            let builder = Builder::new().expect("unable to construct builder");

            {
                let pp = builder::PrepareParams {
                    app_root: app_root.clone(),
                    encore_dev_version: builder::PackageVersion::Published("0.0.0".to_string()),
                };
                builder.prepare(&pp).unwrap();
            }

            let pc = ParseContext::new(app_root.clone(), None, cm.clone(), errs.clone()).unwrap();

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
                Some(_desc) => {
                    println!("successfully parsed {}", app_root.display());
                }
                None => {
                    // Get any errors from the emitter.
                    let errs = errors.lock().unwrap();
                    let mut err_msg = String::new();
                    for err in errs.iter() {
                        err_msg.push_str(err);
                        err_msg.push('\n');
                    }
                    eprintln!("{}", err_msg);
                    panic!("parse failure")
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
