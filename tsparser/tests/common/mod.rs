use clean_path::Clean;
use std::path::PathBuf;

pub fn js_runtime_path() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../runtimes/js")
        .clean()
}
