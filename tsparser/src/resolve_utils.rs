use std::path::{Path, PathBuf};

/// Split a bare specifier into (package_name, subpath).
/// - "foo" => ("foo", "")
/// - "foo/bar" => ("foo", "bar")
/// - "@scope/pkg" => ("@scope/pkg", "")
/// - "@scope/pkg/sub" => ("@scope/pkg", "sub")
pub fn split_package_name(target: &str) -> (&str, &str) {
    match target.find('/') {
        None => (target, ""),
        Some(idx) => {
            if target.starts_with('@') {
                let rem = &target[idx + 1..];
                match rem.find('/') {
                    None => (target, ""),
                    Some(rem_idx) => {
                        let sep = idx + rem_idx + 1;
                        (&target[..sep], &target[sep + 1..])
                    }
                }
            } else {
                (&target[..idx], &target[(idx + 1)..])
            }
        }
    }
}

/// Given a .js/.mjs/.cjs path, return the .d.ts/.d.mts/.d.cts counterpart.
pub fn dts_counterpart(path: &Path) -> Option<PathBuf> {
    let ext = path.extension()?.to_str()?;
    let new_ext = match ext {
        "js" => "d.ts",
        "mjs" => "d.mts",
        "cjs" => "d.cts",
        _ => return None,
    };
    let mut dts = path.to_path_buf();
    dts.set_extension(new_ext);
    Some(dts)
}
