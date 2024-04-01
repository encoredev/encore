use duct::cmd;
use once_cell::sync::Lazy;
use std::path::PathBuf;

pub mod testparse;
pub mod testresolve;
pub mod typeparse;

pub static JS_RUNTIME_PATH: Lazy<PathBuf> = Lazy::new(js_runtime_path);

fn js_runtime_path() -> PathBuf {
    let repo_root = cmd!("git", "rev-parse", "--show-toplevel")
        .stdout_capture()
        .read()
        .unwrap();
    PathBuf::from(repo_root.trim()).join("runtimes").join("js")
}
