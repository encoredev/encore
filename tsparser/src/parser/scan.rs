use crate::parser::module_loader::{Module, ModuleLoader};
use anyhow::Result;
use std::ffi::OsStr;
use swc_common::sync::Lrc;
use walkdir::WalkDir;

/// Walk the filesystem starting at root, parsing all .ts files and calling process_module on each.
pub fn scan<F>(loader: &ModuleLoader, root: &std::path::Path, mut process_module: F) -> Result<()>
where
    F: FnMut(Lrc<Module>) -> Result<()>,
{
    let walker = WalkDir::new(root).into_iter();
    for entry in walker.filter_entry(|e| !ignored(e)) {
        let entry = entry?;
        if !entry.file_type().is_file() {
            continue;
        }

        // If it's a .ts file, parse it.
        let ext = entry.path().extension().and_then(OsStr::to_str);
        if let Some("ts") = ext {
            let module = loader.load_fs_file(entry.path(), None)?;
            process_module(module)?;
        }
    }
    Ok(())
}

fn ignored(entry: &walkdir::DirEntry) -> bool {
    match entry.file_name().to_str().unwrap_or_default() {
        "node_modules" | ".git" | "encore.gen" => true,
        _ => false,
    }
}
