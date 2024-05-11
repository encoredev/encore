use std::path::{Path, PathBuf};

use anyhow::{anyhow, Result};
use litparser_derive::LitParser;
use once_cell::sync::Lazy;
use regex::Regex;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::LitParser;
use litparser::LocalRelPath;

use crate::parser::resourceparser::bind::ResourceOrPath;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::NamedStaticMethod;
use crate::parser::resources::parseutil::{iter_references, NamedClassResource, TrackedNames};
use crate::parser::resources::Resource;
use crate::parser::resources::ResourcePath;
use crate::parser::FilePath;

#[derive(Debug, Clone)]
pub struct SQLDatabase {
    pub name: String,
    pub doc: Option<String>,
    pub migrations: Option<DBMigrations>,
}

#[derive(Debug, Clone)]
pub struct DBMigrations {
    pub dir: PathBuf,
    pub migrations: Vec<DBMigration>,
}

#[derive(Debug, Clone)]
pub struct DBMigration {
    pub file_name: String,
    pub description: String,
    pub number: u64,
}

#[derive(LitParser)]
struct DecodedDatabaseConfig {
    migrations: Option<LocalRelPath>,
}

pub const SQLDB_PARSER: ResourceParser = ResourceParser {
    name: "sqldb",
    interesting_pkgs: &[PkgPath("encore.dev/storage/sqldb")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/storage/sqldb", "SQLDatabase")]);

        let module = pass.module.clone();
        {
            type Res = NamedClassResource<DecodedDatabaseConfig>;
            for r in iter_references::<Res>(&module, &names) {
                let r = r?;

                let migrations = match (r.config.migrations, &pass.module.file_path) {
                    (None, _) => None,
                    (_, FilePath::Custom(_)) => {
                        anyhow::bail!("cannot use custom file path for db migrations")
                    }
                    (Some(rel), FilePath::Real(path)) => {
                        let dir = path.parent().unwrap().join(rel.0);
                        let migrations = parse_migrations(&dir)?;
                        Some(DBMigrations { dir, migrations })
                    }
                };

                let object = match &r.bind_name {
                    None => None,
                    Some(id) => pass
                        .type_checker
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone()))?,
                };

                let resource = Resource::SQLDatabase(Lrc::new(SQLDatabase {
                    name: r.resource_name,
                    doc: r.doc_comment,
                    migrations,
                }));
                pass.add_resource(resource.clone());
                pass.add_bind(BindData {
                    range: r.range,
                    resource: ResourceOrPath::Resource(resource),
                    object,
                    kind: BindKind::Create,
                    ident: r.bind_name,
                });
            }
        }

        {
            for r in iter_references::<NamedStaticMethod>(&module, &names) {
                let r = r?;
                let object = match &r.bind_name {
                    None => None,
                    Some(id) => pass
                        .type_checker
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone()))?,
                };

                pass.add_bind(BindData {
                    range: r.range,
                    resource: ResourceOrPath::Path(ResourcePath::SQLDatabase {
                        name: r.resource_name,
                    }),
                    object,
                    kind: BindKind::Create,
                    ident: r.bind_name,
                });
            }
        }

        Ok(())
    },
};

fn parse_migrations(dir: &Path) -> Result<Vec<DBMigration>> {
    let mut migrations = vec![];

    static FILENAME_RE: Lazy<Regex> =
        Lazy::new(|| Regex::new(r"^(\d+)_([^.]+)\.(up|down).sql$").unwrap());

    let paths = std::fs::read_dir(dir)?;
    for entry in paths {
        let entry = entry?;
        if !entry.file_type()?.is_file() {
            continue;
        }
        let path = entry.path();
        let name = entry.file_name();
        let name = name.to_str().ok_or(anyhow!(
            "invalid migration filename: {}",
            name.to_string_lossy()
        ))?;

        // If the file is not an SQL file ignore it, to allow for other files to be present
        // in the migration directory. For SQL files we want to ensure they're properly named
        // so that we complain loudly about potential typos. (It's theoretically possible to
        // typo the filename extension as well, but it's less likely due to syntax highlighting).
        let ext = path.extension().and_then(|ext| ext.to_str());
        if ext != Some("sql") {
            continue;
        }

        // Ensure the file name matches the regex.
        let captures = FILENAME_RE
            .captures(name)
            .ok_or(anyhow!("invalid migration filename: {}", name))?;
        if captures[3].eq("up") {
            migrations.push(DBMigration {
                file_name: name.to_string(),
                description: captures[2].to_string(),
                number: captures[1].parse()?,
            });
        }
    }

    Ok(migrations)
}
