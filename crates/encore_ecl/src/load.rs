use std::collections::{HashSet, VecDeque};
use std::rc::Rc;

use crate::ast::{File, Import};
use crate::diagnostic::{Diagnostic, ErrorList};
use crate::eval::RuleSet;
use crate::parser::parse_file;
use crate::position::{Position, SourceFile};
use crate::value::go_quote;

/// A read-only filesystem abstraction, mirroring Go's `io/fs.FS`. Paths use
/// forward slashes and are interpreted relative to the filesystem root.
pub trait FileSystem {
    fn read_file(&self, path: &str) -> std::io::Result<Vec<u8>>;
    fn is_file(&self, path: &str) -> bool;
}

/// An in-memory filesystem, useful for tests (mirrors Go's `fstest.MapFS`).
#[derive(Default)]
pub struct MapFs {
    files: std::collections::HashMap<String, Vec<u8>>,
}

impl MapFs {
    pub fn new() -> MapFs {
        MapFs::default()
    }

    pub fn insert(&mut self, path: impl Into<String>, content: impl Into<Vec<u8>>) -> &mut Self {
        self.files.insert(path.into(), content.into());
        self
    }
}

impl FileSystem for MapFs {
    fn read_file(&self, path: &str) -> std::io::Result<Vec<u8>> {
        match self.files.get(path) {
            Some(d) => Ok(d.clone()),
            None => Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                format!("open {path}: file does not exist"),
            )),
        }
    }

    fn is_file(&self, path: &str) -> bool {
        self.files.contains_key(path)
    }
}

/// A filesystem backed by a directory on disk.
pub struct DiskFs {
    root: std::path::PathBuf,
}

impl DiskFs {
    pub fn new(root: impl Into<std::path::PathBuf>) -> DiskFs {
        DiskFs { root: root.into() }
    }
}

impl FileSystem for DiskFs {
    fn read_file(&self, path: &str) -> std::io::Result<Vec<u8>> {
        std::fs::read(self.root.join(path))
    }

    fn is_file(&self, path: &str) -> bool {
        self.root.join(path).is_file()
    }
}

struct Pending {
    path: String,
    imp: Option<Import>,
    src: Option<Rc<SourceFile>>,
}

/// Parses the given entrypoint files from `fsys` and recursively follows their
/// imports, returning all files combined into a `RuleSet`.
///
/// Import paths are resolved relative to the importing file's directory first,
/// then relative to the root of `fsys`. A file imported multiple times is
/// included once; import cycles are therefore harmless.
pub fn load(fsys: &dyn FileSystem, entrypoints: &[&str]) -> Result<RuleSet, ErrorList> {
    let mut diags = ErrorList::new();
    let mut files: Vec<File> = Vec::new();
    let mut visited: HashSet<String> = HashSet::new();
    let mut queue: VecDeque<Pending> = VecDeque::new();
    for p in entrypoints {
        queue.push_back(Pending {
            path: path_clean(p),
            imp: None,
            src: None,
        });
    }

    while let Some(item) = queue.pop_front() {
        if visited.contains(&item.path) {
            continue;
        }
        visited.insert(item.path.clone());

        let data = match fsys.read_file(&item.path) {
            Ok(d) => d,
            Err(e) => {
                if let Some(imp) = &item.imp {
                    let d = diags.add(
                        item.src.clone(),
                        imp.path_pos.clone(),
                        imp.path_end.clone(),
                        format!("cannot read imported file {}: {}", go_quote(&item.path), e),
                    );
                    let _ = d;
                } else {
                    diags.push(Diagnostic::new(
                        None,
                        Position {
                            file: item.path.clone(),
                            ..Default::default()
                        },
                        Position::default(),
                        format!("cannot read file {}: {}", go_quote(&item.path), e),
                    ));
                }
                continue;
            }
        };

        let pr = parse_file(item.path.clone(), data);
        if !pr.errors.is_empty() {
            diags.extend(pr.errors);
        }
        let file = pr.file;
        let dir = path_dir(&item.path);
        let file_src = file.src.clone();
        let imports = file.imports.clone();
        files.push(file);

        for imp in &imports {
            let target = match resolve_import(fsys, &dir, &imp.path) {
                Some(t) => t,
                None => {
                    let rel = path_clean(&path_join(&dir, &imp.path));
                    let cleaned = path_clean(&imp.path);
                    let mut d = Diagnostic::new(
                        Some(file_src.clone()),
                        imp.path_pos.clone(),
                        imp.path_end.clone(),
                        format!("cannot find imported file {}", go_quote(&imp.path)),
                    );
                    if rel != cleaned {
                        d.detail = vec![format!(
                            "looked for {} and {}",
                            go_quote(&rel),
                            go_quote(&cleaned)
                        )];
                    }
                    diags.push(d);
                    continue;
                }
            };
            queue.push_back(Pending {
                path: target,
                imp: Some(imp.clone()),
                src: Some(file_src.clone()),
            });
        }
    }

    if !diags.is_empty() {
        diags.sort();
        return Err(diags);
    }
    Ok(RuleSet::new(files))
}

/// Resolves an import path relative to the importing file's directory, falling
/// back to the filesystem root.
fn resolve_import(fsys: &dyn FileSystem, dir: &str, imp_path: &str) -> Option<String> {
    let candidates = [path_clean(&path_join(dir, imp_path)), path_clean(imp_path)];
    candidates.into_iter().find(|c| fsys.is_file(c))
}

// --- forward-slash path helpers, mirroring Go's `path` package ---

fn path_clean(path: &str) -> String {
    if path.is_empty() {
        return ".".to_string();
    }
    let rooted = path.starts_with('/');
    let mut comps: Vec<&str> = Vec::new();
    for part in path.split('/') {
        match part {
            "" | "." => continue,
            ".." => {
                if rooted {
                    comps.pop();
                } else if comps.last().map(|s| *s == "..").unwrap_or(true) {
                    comps.push("..");
                } else {
                    comps.pop();
                }
            }
            _ => comps.push(part),
        }
    }
    let mut s = String::new();
    if rooted {
        s.push('/');
    }
    s.push_str(&comps.join("/"));
    if s.is_empty() {
        ".".to_string()
    } else {
        s
    }
}

fn path_dir(p: &str) -> String {
    match p.rfind('/') {
        Some(i) => path_clean(&p[..=i]),
        None => ".".to_string(),
    }
}

fn path_join(a: &str, b: &str) -> String {
    let joined = match (a.is_empty(), b.is_empty()) {
        (true, true) => return String::new(),
        (true, false) => b.to_string(),
        (false, true) => a.to_string(),
        (false, false) => format!("{a}/{b}"),
    };
    path_clean(&joined)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::eval::Resource;
    use crate::testutil::{assert_err_contains, assert_value, eval_ok};
    use crate::value::{boolean, number};

    fn res(kind: &str, name: &str) -> Resource {
        Resource {
            kind: kind.into(),
            name: name.into(),
            ..Default::default()
        }
    }

    fn load_err(fsys: &dyn FileSystem, entrypoints: &[&str]) -> ErrorList {
        match load(fsys, entrypoints) {
            Ok(_) => panic!("expected load error, got ok"),
            Err(e) => e,
        }
    }

    #[test]
    fn load_imports() {
        let mut fsys = MapFs::new();
        fsys.insert(
            "main.encore",
            "import \"policies/services.encore\"\nimport \"policies/storage.encore\"\n\nservice \"api\" {\n    cpu: default 2\n}\n",
        );
        fsys.insert(
            "policies/services.encore",
            "for service {\n    cpu: >= 0.25 & <= 8 | default 0.5\n}\n",
        );
        fsys.insert(
            "policies/storage.encore",
            "import \"buckets.encore\"\n\nfor sql_database {\n    deletion_protection: true\n}\n",
        );
        fsys.insert(
            "policies/buckets.encore",
            "for bucket {\n    public_access: false\n}\n",
        );

        let rs = load(&fsys, &["main.encore"]).unwrap();
        assert_eq!(rs.files.len(), 4);

        let result = eval_ok(&rs, &res("service", "api"));
        assert_eq!(result.matched.len(), 2);
        assert_value(&result.properties.get("cpu").unwrap().value, &number(2.0));

        let result = eval_ok(&rs, &res("bucket", "uploads"));
        assert_value(
            &result.properties.get("public_access").unwrap().value,
            &boolean(false),
        );
    }

    #[test]
    fn load_import_cycle() {
        let mut fsys = MapFs::new();
        fsys.insert(
            "a.encore",
            "import \"b.encore\"\nfor service { cpu: default 1 }\n",
        );
        fsys.insert(
            "b.encore",
            "import \"a.encore\"\nfor bucket { versioning: true }\n",
        );
        let rs = load(&fsys, &["a.encore"]).unwrap();
        assert_eq!(rs.files.len(), 2);
    }

    #[test]
    fn load_duplicate_imports_included_once() {
        let mut fsys = MapFs::new();
        fsys.insert(
            "a.encore",
            "import \"common.encore\"\nimport \"b.encore\"\n",
        );
        fsys.insert("b.encore", "import \"common.encore\"\n");
        fsys.insert("common.encore", "for service { cpu: default 1 }\n");
        let rs = load(&fsys, &["a.encore"]).unwrap();
        assert_eq!(rs.files.len(), 3);

        let result = eval_ok(
            &rs,
            &Resource {
                kind: "service".into(),
                ..Default::default()
            },
        );
        assert_eq!(result.matched.len(), 1);
    }

    #[test]
    fn load_missing_import() {
        let mut fsys = MapFs::new();
        fsys.insert("main.encore", "import \"nope.encore\"\n");
        assert_err_contains(
            &load_err(&fsys, &["main.encore"]),
            &[
                "main.encore:1:8: error: cannot find imported file \"nope.encore\"",
                "import \"nope.encore\"",
            ],
        );
    }

    #[test]
    fn load_missing_entrypoint() {
        let fsys = MapFs::new();
        assert_err_contains(
            &load_err(&fsys, &["main.encore"]),
            &["cannot read file \"main.encore\""],
        );
    }

    #[test]
    fn load_parse_errors_across_files() {
        let mut fsys = MapFs::new();
        fsys.insert(
            "main.encore",
            "import \"other.encore\"\nfor service { cpu = 1 }\n",
        );
        fsys.insert("other.encore", "for bucket { versioning == }\n");
        assert_err_contains(
            &load_err(&fsys, &["main.encore"]),
            &["main.encore:2:19", "other.encore:1:25"],
        );
    }
}
