use std::env;
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};

fn main() {
    let encore_dev_dir = PathBuf::from("../../runtimes/js/encore.dev");
    println!(
        "cargo:rerun-if-changed={}",
        encore_dev_dir.to_string_lossy()
    );

    let out_dir = env::var("OUT_DIR").unwrap();
    let dest_path = Path::new(&out_dir).join("encore_dev_files.rs");
    let mut out = fs::File::create(&dest_path).unwrap();

    // Collect all .ts files and package.json
    let mut files: Vec<(String, PathBuf)> = Vec::new();

    // Add package.json
    let pkg_json = encore_dev_dir.join("package.json");
    if pkg_json.exists() {
        files.push(("node_modules/encore.dev/package.json".into(), pkg_json));
    }

    // Walk for .ts files
    collect_ts_files(&encore_dev_dir, &encore_dev_dir, &mut files);

    writeln!(out, "static ENCORE_DEV_FILES: &[(&str, &str)] = &[").unwrap();

    for (rel_path, abs_path) in &files {
        let abs = fs::canonicalize(abs_path).unwrap();
        writeln!(
            out,
            "    (\"{rel_path}\", include_str!(\"{}\")),",
            abs.to_string_lossy().replace('\\', "/")
        )
        .unwrap();
    }

    writeln!(out, "];").unwrap();
}

fn collect_ts_files(base: &Path, dir: &Path, files: &mut Vec<(String, PathBuf)>) {
    let Ok(entries) = fs::read_dir(dir) else {
        return;
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            // Skip dist/, node_modules/, and hidden dirs
            let name = entry.file_name();
            let name = name.to_string_lossy();
            if name == "dist" || name == "node_modules" || name.starts_with('.') {
                continue;
            }
            collect_ts_files(base, &path, files);
        } else if let Some(ext) = path.extension() {
            if ext == "ts" {
                let rel = path.strip_prefix(base).unwrap();
                let nm_path = format!("node_modules/encore.dev/{}", rel.to_string_lossy());
                files.push((nm_path, path));
            }
        }
    }
}
