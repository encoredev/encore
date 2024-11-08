use std::path::{Path, PathBuf};
use std::str::FromStr;

use anyhow::Context;
use anyhow::{anyhow, Result};
use itertools::Either;
use litparser_derive::LitParser;
use once_cell::sync::Lazy;
use regex::Regex;
use swc_common::errors::HANDLER;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::LitParser;
use litparser::LocalRelPath;

use crate::parser::resourceparser::bind::ResourceOrPath;
use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{iter_references, TrackedNames};
use crate::parser::resources::parseutil::{NamedClassResourceOptionalConfig, NamedStaticMethod};
use crate::parser::resources::Resource;
use crate::parser::resources::ResourcePath;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::{FilePath, Range};
use crate::span_err::ErrorWithSpanExt;

#[derive(Debug, Clone)]
pub struct SQLDatabase {
    pub name: String,
    pub doc: Option<String>,
    pub migrations: Option<DBMigrations>,
}

#[derive(Clone, Debug)]
pub enum Orm {
    Prisma,
    Drizzle,
}

impl FromStr for Orm {
    type Err = anyhow::Error;

    fn from_str(input: &str) -> Result<Orm, Self::Err> {
        match input {
            "prisma" => Ok(Orm::Prisma),
            "drizzle" => Ok(Orm::Drizzle),
            _ => Err(anyhow!("unexpected value for orm: {input}")),
        }
    }
}

#[derive(Debug, Clone)]
pub struct DBMigrations {
    pub dir: PathBuf,
    pub migrations: Vec<DBMigration>,
    pub non_seq_migrations: bool,
}

#[derive(Debug, Clone)]
pub struct DBMigration {
    pub file_name: String,
    pub description: String,
    pub number: u64,
}

#[derive(LitParser)]
struct MigrationsConfig {
    path: LocalRelPath,
    orm: Option<String>,
}

#[derive(LitParser, Default)]
struct DecodedDatabaseConfig {
    migrations: Option<Either<LocalRelPath, MigrationsConfig>>,
}

pub const SQLDB_PARSER: ResourceParser = ResourceParser {
    name: "sqldb",
    interesting_pkgs: &[PkgPath("encore.dev/storage/sqldb")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/storage/sqldb", "SQLDatabase")]);

        let module = pass.module.clone();
        {
            type Res = NamedClassResourceOptionalConfig<DecodedDatabaseConfig>;
            for r in iter_references::<Res>(&module, &names) {
                let r = r?;
                let cfg = r.config.unwrap_or_default();

                let migrations = match (cfg.migrations, &pass.module.file_path) {
                    (None, _) => None,
                    (_, FilePath::Custom(_)) => {
                        anyhow::bail!("cannot use custom file path for db migrations")
                    }
                    (Some(Either::Left(rel)), FilePath::Real(path)) => {
                        let dir = path.parent().unwrap().join(rel.0);
                        let migrations = parse_migrations(&dir, None)?;
                        Some(DBMigrations {
                            dir,
                            migrations,
                            non_seq_migrations: false,
                        })
                    }
                    (Some(Either::Right(cfg)), FilePath::Real(path)) => {
                        let dir = path.parent().unwrap().join(cfg.path.0);
                        let orm = if let Some(ref string) = cfg.orm {
                            match Orm::from_str(string) {
                                Ok(orm) => Some(orm),
                                Err(e) => {
                                    e.with_span(r.range.into()).report();
                                    continue;
                                }
                            }
                        } else {
                            None
                        };

                        let migrations = parse_migrations(&dir, orm.as_ref())?;
                        let non_seq_migrations = match orm {
                            Some(Orm::Prisma) => true,
                            _ => false,
                        };
                        Some(DBMigrations {
                            dir,
                            migrations,
                            non_seq_migrations,
                        })
                    }
                };

                let object = match &r.bind_name {
                    None => None,
                    Some(id) => pass
                        .type_checker
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
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
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
                };

                pass.add_bind(BindData {
                    range: r.range,
                    resource: ResourceOrPath::Path(ResourcePath::SQLDatabase {
                        name: r.resource_name,
                    }),
                    object,
                    kind: BindKind::Reference,
                    ident: r.bind_name,
                });
            }
        }

        Ok(())
    },
};

fn visit_dirs(
    dir: &Path,
    depth: i8,
    max_depth: i8,
    cb: &mut dyn FnMut(&std::fs::DirEntry) -> Result<()>,
) -> Result<()> {
    if dir.is_dir() {
        for entry in std::fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();
            if path.is_dir() && depth < max_depth {
                visit_dirs(&path, depth + 1, max_depth, cb)?;
            } else {
                cb(&entry)?;
            }
        }
    }
    Ok(())
}

fn parse_default(dir: &Path) -> Result<Vec<DBMigration>> {
    let mut migrations = vec![];
    static FILENAME_RE: Lazy<Regex> =
        Lazy::new(|| Regex::new(r"^(\d+)_([^.]+)\.(up|down).sql$").unwrap());

    visit_dirs(dir, 0, 0, &mut |entry| -> Result<()> {
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
            return Ok(());
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
        Ok(())
    })?;

    migrations.sort_by_key(|m| m.number);
    Ok(migrations)
}

fn parse_drizzle(dir: &Path) -> Result<Vec<DBMigration>> {
    let mut migrations = vec![];

    static FILENAME_RE: Lazy<Regex> = Lazy::new(|| Regex::new(r"^(\d+)_([^.]+)\.sql$").unwrap());

    visit_dirs(dir, 0, 0, &mut |entry| -> Result<()> {
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
            return Ok(());
        }

        // Ensure the file name matches the regex.
        let captures = FILENAME_RE
            .captures(name)
            .ok_or(anyhow!("invalid migration filename: {}", name))?;
        migrations.push(DBMigration {
            file_name: name.to_string(),
            description: captures[2].to_string(),
            number: captures[1].parse()?,
        });

        Ok(())
    })?;
    Ok(migrations)
}

fn parse_prisma(dir: &Path) -> Result<Vec<DBMigration>> {
    let mut migrations = vec![];

    static FILENAME_RE: Lazy<Regex> = Lazy::new(|| Regex::new(r"^(\d+)-(.+)$").unwrap());

    visit_dirs(dir, 0, 1, &mut |entry| -> Result<()> {
        let path = entry.path();
        let name = entry.file_name();
        let name = name.to_str().ok_or(anyhow!(
            "invalid migration filename: {}",
            name.to_string_lossy()
        ))?;
        if name != "migration.sql" {
            return Ok(());
        }
        let dir_name = path
            .parent()
            .context(anyhow!("migration directory has no parent"))?
            .file_name()
            .context(anyhow!("migration directory has no file name"))?
            .to_str()
            .context(anyhow!("migration directory has invalid file name"))?;

        // Ensure the file name matches the regex.
        let captures = FILENAME_RE
            .captures(dir_name)
            .ok_or(anyhow!("invalid migration directory name: {}", dir_name))?;
        migrations.push(DBMigration {
            file_name: path
                .strip_prefix(dir)
                .context(anyhow!(
                    "migration directory is not a subdirectory of {}",
                    dir.display()
                ))?
                .to_string_lossy()
                .to_string(),
            description: captures[2].to_string(),
            number: captures[1].parse()?,
        });
        Ok(())
    })?;
    Ok(migrations)
}

fn parse_migrations(dir: &Path, orm: Option<&Orm>) -> Result<Vec<DBMigration>> {
    let mut migrations = match orm {
        Some(Orm::Drizzle) => parse_drizzle(dir),
        Some(Orm::Prisma) => parse_prisma(dir),
        _ => parse_default(dir),
    }
    .context("failed to parse migrations")?;
    migrations.sort_by_key(|m| m.number);
    Ok(migrations)
}

pub fn resolve_database_usage(
    data: &ResolveUsageData,
    db: Lrc<SQLDatabase>,
) -> Result<Option<Usage>> {
    Ok(match &data.expr.kind {
        UsageExprKind::MethodCall(_)
        | UsageExprKind::FieldAccess(_)
        | UsageExprKind::CallArg(_)
        | UsageExprKind::ConstructorArg(_) => Some(Usage::AccessDatabase(AccessDatabaseUsage {
            range: data.expr.range,
            db,
        })),

        _ => {
            HANDLER.with(|h| {
                h.span_err(
                    data.expr.range.to_span(),
                    "invalid use of database resource",
                )
            });
            None
        }
    })
}

#[derive(Debug)]
pub struct AccessDatabaseUsage {
    pub range: Range,
    pub db: Lrc<SQLDatabase>,
}
