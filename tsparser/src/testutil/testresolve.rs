use anyhow::{anyhow, bail, Error};
use std::path::{Path, PathBuf};
use swc_common::FileName;
use swc_ecma_loader::resolve::Resolve;

pub struct TestResolver<'a> {
    root_dir: &'a Path,
    ar: &'a txtar::Archive,
}

impl<'a> TestResolver<'a> {
    pub fn new(root_dir: &'a Path, ar: &'a txtar::Archive) -> Self {
        Self { root_dir, ar }
    }
}

impl Resolve for TestResolver<'_> {
    fn resolve(&self, base: &FileName, module_specifier: &str) -> Result<FileName, Error> {
        // Fake the existence of the runtime module for now.
        if module_specifier.starts_with("encore.dev/") {
            let mut path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .join("runtime")
                .join(
                    // Get the suffix after the "encore.dev/" part.
                    PathBuf::from(module_specifier)
                        .strip_prefix("encore.dev/")
                        .unwrap(),
                );
            path.set_extension("ts");
            return Ok(FileName::Real(path));
        }

        let base = match base {
            FileName::Real(v) => v,
            _ => bail!("TestResolver supports only files"),
        };

        let base_dir = base.parent().ok_or(anyhow!(
            "file has no parent directory: {}",
            base.to_string_lossy()
        ))?;

        let mut path = base_dir.join(module_specifier);
        path.set_extension("ts");
        path = clean_path::clean(path);

        for file in &self.ar.files {
            let cleaned = clean_path::clean(self.root_dir.join(&file.name));
            if cleaned == path {
                return Ok(FileName::Real(path));
            }
        }
        Err(anyhow!("import not found: {}", module_specifier))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_resolve() {
        let ar = txtar::from_str(
            "
-- foo.ts --
import { Bar } from './bar.ts';
-- bar.ts --
export const Bar = 5;
        ",
        );

        let root = PathBuf::from("");
        let r = TestResolver::new(&root, &ar);

        let base = FileName::Real("foo.ts".into());
        assert_eq!(
            r.resolve(&base, "./bar.ts").unwrap(),
            FileName::Real("bar.ts".into())
        );

        assert_eq!(
            r.resolve(&base, "./moo.ts").unwrap_err().to_string(),
            "import not found: ./moo.ts"
        );
    }
}
