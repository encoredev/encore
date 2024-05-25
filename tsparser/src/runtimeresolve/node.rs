use std::collections::HashSet;
use std::{
    fs::File,
    io::BufReader,
    path::{Component, Path, PathBuf},
};

use anyhow::{bail, Context, Error};
use clean_path::Clean;
use serde::Deserialize;
use swc_common::FileName;
use swc_ecma_loader::resolve::Resolve;

use crate::runtimeresolve::exports::Exports;
use crate::runtimeresolve::tsconfig::TsConfigPathResolver;

static PACKAGE: &str = "package.json";

#[derive(Deserialize)]
struct PackageJson {
    #[allow(dead_code)]
    #[serde(default)]
    main: Option<String>,
    #[allow(dead_code)]
    #[serde(default)]
    module: Option<String>,
    #[serde(default)]
    exports: Option<Exports>,
}

#[derive(Debug)]
pub struct EncoreRuntimeResolver<R> {
    inner: R,
    js_runtime_path: PathBuf,
    extra_export_conditions: Vec<String>,
    tsconfig_resolver: Option<TsConfigPathResolver>,
}

static DEFAULT_CONDITIONS: &[&str] = &["node-addons", "node", "import", "require", "default"];

impl<R> EncoreRuntimeResolver<R> {
    pub fn new(inner: R, js_runtime_path: PathBuf, extra_export_conditions: Vec<String>) -> Self {
        Self {
            inner,
            js_runtime_path,
            extra_export_conditions,
            tsconfig_resolver: None,
        }
    }

    pub fn with_tsconfig_resolver(self, resolver: TsConfigPathResolver) -> Self {
        Self {
            tsconfig_resolver: Some(resolver),
            ..self
        }
    }

    /// Resolve a path from the "exports" directive in the package.json file, if present.
    fn resolve_export(&self, pkg_dir: &Path, rel_target: &str) -> Result<Option<PathBuf>, Error> {
        let package_json_path = pkg_dir.join(PACKAGE);
        if !package_json_path.is_file() {
            bail!("package.json not found: {}", package_json_path.display());
        }

        let file = File::open(&package_json_path)?;
        let reader = BufReader::new(file);
        let pkg: PackageJson = serde_json::from_reader(reader).context(format!(
            "failed to deserialize {}",
            package_json_path.display()
        ))?;

        let Some(exports) = &pkg.exports else {
            bail!("no exports field in {}", package_json_path.display());
        };

        let mut conditions =
            HashSet::from_iter(self.extra_export_conditions.iter().map(|s| s.as_str()));
        conditions.extend(DEFAULT_CONDITIONS);

        // The result is relative to the package directory, whereas we want to return an absolute path.
        let result = exports
            .resolve_import_path(rel_target, &conditions)
            .map(|p| p.to_path_buf());
        Ok(result.map(|p| pkg_dir.join(p)))
    }

    /// Resolve the package name from a target import path, e.g.:
    /// - "foo" => ("foo", "")
    /// - "foo/bar" => ("foo", "bar")
    /// - "@foo/bar" => ("@foo/bar", "")
    /// - "@foo/bar/baz" => ("@foo/bar", "baz")
    fn pkg_name_from_target<'b>(&self, target: &'b str) -> (&'b str, &'b str) {
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

    fn resolve_encore_module(&self, target: &str) -> Result<Option<PathBuf>, Error> {
        let target_path = Path::new(target);
        let mut components = target_path.components();

        if let Some(Component::Normal(_)) = components.next() {
            // It's a normal import, not an absolute or relative path.
            let (pkg_name, pkg_path) = self.pkg_name_from_target(target);
            let pkg_dir = self.js_runtime_path.join(pkg_name);

            if pkg_dir.exists() {
                return self.resolve_export(&pkg_dir, pkg_path);
            }
        }

        Ok(None)
    }
}

impl<R> Resolve for EncoreRuntimeResolver<R>
where
    R: Resolve,
{
    fn resolve(&self, base: &FileName, target: &str) -> Result<FileName, Error> {
        if let Some(tsconfig) = &self.tsconfig_resolver {
            if let Some(buf) = tsconfig.resolve(target) {
                return self.inner.resolve(tsconfig.base(), buf.as_ref());
            }
        }

        match self.resolve_encore_module(target)? {
            Some(buf) => Ok(FileName::Real(buf.clean())),
            None => self.inner.resolve(base, target),
        }
    }
}
