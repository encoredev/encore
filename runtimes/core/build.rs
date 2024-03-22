use std::path::{Path, PathBuf};

fn main() -> std::io::Result<()> {
    prost_build::compile_protos(
        &[
            "../../proto/encore/runtime/v1/runtime.proto",
            "../../proto/encore/parser/meta/v1/meta.proto",
        ],
        &["../../proto/"],
    )?;

    // We add an extra compile time environment variable which allows our error module
    // to know where the root of the workspace that we are compiling is - thus in stack traces
    // we can show the relative path to the file that caused the error.
    println!(
        "cargo:rustc-env=ENCORE_BINARY_SRC_PATH={}",
        workspace_dir().to_string_lossy()
    );

    println!("cargo:rustc-env=ENCORE_BINARY_GIT_HASH={}", get_git_hash());

    Ok(())
}

fn workspace_dir() -> PathBuf {
    let output = std::process::Command::new(env!("CARGO"))
        .arg("locate-project")
        .arg("--workspace")
        .arg("--message-format=plain")
        .output()
        .unwrap()
        .stdout;

    let cargo_toml_file = Path::new(std::str::from_utf8(&output).unwrap().trim());
    cargo_toml_file.parent().unwrap().to_path_buf()
}

use std::env;

fn get_git_hash() -> String {
    use std::process::Command;

    let commit = Command::new("git")
        .arg("rev-parse")
        .arg("--verify")
        .arg("HEAD")
        .output();
    if let Ok(commit_output) = commit {
        let commit_string = String::from_utf8_lossy(&commit_output.stdout);

        commit_string
            .lines()
            .next()
            .unwrap_or("unknown")
            .to_string()
    } else {
        "unknown".to_string()
    }
}
