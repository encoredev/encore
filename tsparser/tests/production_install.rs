#![cfg(unix)]

use std::io::Write;
use std::os::unix::fs::PermissionsExt;
use std::process::{Command, Stdio};

// Exercise the actual prepare protocol, manager resolution and child execution.
// Each parser subprocess gets an isolated PATH, so tests never mutate global env
// or invoke the user's package managers. No registry or VM is needed.
#[test]
fn production_install_commands() {
    for (manager, yarn_version, expected) in [
        ("npm", "", "install\n--omit=dev\n"),
        ("pnpm", "", "install\n--prod\n"),
        ("bun", "", "install\n--backend\ncopyfile\n--omit\ndev\n"),
        ("yarn", "1.22.22", "install\n--production=true\n"),
        ("yarn", "4.6.0", "workspaces\nfocus\n--all\n--production\n"),
    ] {
        for existing_tree in [false, true] {
            let case = PrepareCase::new(manager, yarn_version, existing_tree);
            assert!(case.prepare(Some("production"), false), "{manager}");
            assert_eq!(case.args(), expected, "{manager}, existing={existing_tree}");
            assert_eq!(
                std::fs::read_to_string(case.root.path().join("cwd"))
                    .unwrap()
                    .trim(),
                case.root.path().canonicalize().unwrap().to_str().unwrap()
            );
        }
    }
}

#[test]
fn default_install_keeps_development_behavior() {
    for manager in ["npm", "pnpm", "bun", "yarn"] {
        for mode in [None, Some("all")] {
            for existing in [false, true] {
                let case = PrepareCase::new(manager, "4.6.0", existing);
                assert!(case.prepare(mode, false));
                let expected = if existing {
                    ""
                } else if manager == "bun" {
                    "install\n--backend\ncopyfile\n"
                } else {
                    "install\n"
                };
                assert_eq!(case.args(), expected, "{manager}");
            }
        }
    }
}

#[test]
fn production_install_inherits_workspace_manager() {
    let case = PrepareCase::new("yarn", "4.6.0", false);
    let app = case.root.path().join("nested-app");
    std::fs::create_dir(&app).unwrap();
    std::fs::write(
        app.join("package.json"),
        r#"{"dependencies":{"encore.dev":"1.2.3"}}"#,
    )
    .unwrap();
    assert!(case.prepare_in(&app, Some("production"), false));
    assert_eq!(
        std::fs::read_to_string(app.join("args")).unwrap(),
        "workspaces\nfocus\n--all\n--production\n"
    );
    assert_eq!(case.args(), "");
}

#[test]
fn production_install_failure_is_reported() {
    let case = PrepareCase::new("npm", "", false);
    assert!(!case.prepare(Some("production"), true));
    let case = PrepareCase::new("yarn", "invalid-version", false);
    assert!(!case.prepare(Some("production"), false));
    assert_eq!(case.args(), "");
}

struct PrepareCase {
    root: tempdir::TempDir,
    bin: tempdir::TempDir,
    yarn_version: String,
}
impl PrepareCase {
    fn new(manager: &str, yarn_version: &str, existing: bool) -> Self {
        let root = tempdir::TempDir::new("prepare-app").unwrap();
        let bin = tempdir::TempDir::new("prepare-bin").unwrap();
        std::fs::write(
            root.path().join("package.json"),
            serde_json::json!({
                "packageManager": format!("{manager}@1.0.0"),
                "dependencies": {"encore.dev": "1.2.3"}
            })
            .to_string(),
        )
        .unwrap();
        if existing {
            std::fs::create_dir(root.path().join("node_modules")).unwrap();
        }
        let program = bin.path().join(manager);
        std::fs::write(&program, "#!/bin/sh\nif [ \"$1\" = --version ]; then printf '%s\\n' \"$TEST_YARN_VERSION\"; exit; fi\nprintf '%s\\n' \"$@\" > args\npwd > cwd\nexit \"$TEST_INSTALL_EXIT\"\n").unwrap();
        std::fs::set_permissions(&program, std::fs::Permissions::from_mode(0o755)).unwrap();
        Self {
            root,
            bin,
            yarn_version: yarn_version.to_owned(),
        }
    }
    // `mode` is the wire value of builder.InstallMode; None omits the field.
    fn prepare(&self, mode: Option<&str>, fail: bool) -> bool {
        self.prepare_in(self.root.path(), mode, fail)
    }
    fn prepare_in(&self, app: &std::path::Path, mode: Option<&str>, fail: bool) -> bool {
        let mut input = serde_json::json!({"app_root": app, "runtime_version": "v1.2.3"});
        if let Some(mode) = mode {
            input["install_mode"] = mode.into();
        }
        let mut child = Command::new(env!("CARGO_BIN_EXE_tsparser-encore"))
            .env("PATH", self.bin.path())
            .env("BUN_INSTALL_BACKEND", "copyfile")
            .env("TEST_YARN_VERSION", &self.yarn_version)
            .env("TEST_INSTALL_EXIT", if fail { "1" } else { "0" })
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .unwrap();
        writeln!(child.stdin.take().unwrap(), "prepare\n{input}").unwrap();
        let output = child.wait_with_output().unwrap();
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        assert!(
            output.stdout.len() >= 5,
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        output.stdout[4] == 0
    }
    fn args(&self) -> String {
        std::fs::read_to_string(self.root.path().join("args")).unwrap_or_default()
    }
}
