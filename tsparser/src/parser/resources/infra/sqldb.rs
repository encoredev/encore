use std::path::{Path, PathBuf};
use std::str::FromStr;

use itertools::Either;
use litparser_derive::LitParser;
use once_cell::sync::Lazy;
use regex::Regex;
use swc_common::sync::Lrc;
use swc_common::{Span, Spanned};
use swc_ecma_ast as ast;

use litparser::{report_and_continue, LitParser, Sp, ToParseErr};
use litparser::{LocalRelPath, ParseResult};

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
use crate::span_err::{ErrReporter, ErrorWithSpanExt};

#[derive(Debug, Clone)]
pub struct SQLDatabase {
    pub span: Span,
    pub name: String,
    pub doc: Option<String>,
    pub migrations: Option<Sp<DBMigrations>>,
}

#[derive(Clone, Debug)]
pub enum MigrationFileSource {
    Prisma,
    Drizzle,
}

#[derive(Debug, thiserror::Error)]
pub enum MigrationFileSourceParseError {
    #[error("unexpected value for migration file source: {0}")]
    UnexpectedValue(String),
}

impl FromStr for MigrationFileSource {
    type Err = MigrationFileSourceParseError;

    fn from_str(input: &str) -> Result<MigrationFileSource, Self::Err> {
        match input {
            "prisma" => Ok(MigrationFileSource::Prisma),
            "drizzle" => Ok(MigrationFileSource::Drizzle),
            _ => Err(MigrationFileSourceParseError::UnexpectedValue(
                input.to_string(),
            )),
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
    source: Option<String>,
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
                let r = report_and_continue!(r);
                let cfg = r.config.unwrap_or_default();

                let migrations = match (cfg.migrations, &pass.module.file_path) {
                    (None, _) => None,
                    (_, FilePath::Custom(_)) => {
                        pass.module
                            .ast
                            .span()
                            .shrink_to_lo()
                            .err("cannot use custom file path for db migrations");
                        continue;
                    }
                    (Some(Either::Left(rel)), FilePath::Real(path)) => {
                        let dir = path.parent().unwrap().join(rel.buf);
                        let migrations =
                            report_and_continue!(parse_migrations(rel.span, &dir, None));
                        Some(Sp::new(
                            rel.span,
                            DBMigrations {
                                dir,
                                migrations,
                                non_seq_migrations: false,
                            },
                        ))
                    }
                    (Some(Either::Right(cfg)), FilePath::Real(path)) => {
                        let dir = path.parent().unwrap().join(cfg.path.buf);
                        let source = if let Some(ref string) = cfg.source {
                            match MigrationFileSource::from_str(string) {
                                Ok(source) => Some(source),
                                Err(e) => {
                                    e.with_span(r.range.into()).report();
                                    continue;
                                }
                            }
                        } else {
                            None
                        };

                        let migrations = report_and_continue!(parse_migrations(
                            cfg.path.span,
                            &dir,
                            source.as_ref()
                        ));
                        let non_seq_migrations =
                            matches!(source, Some(MigrationFileSource::Prisma));
                        Some(Sp::new(
                            cfg.path.span,
                            DBMigrations {
                                dir,
                                migrations,
                                non_seq_migrations,
                            },
                        ))
                    }
                };

                let object = match &r.bind_name {
                    None => None,
                    Some(id) => pass
                        .type_checker
                        .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
                };

                let resource = Resource::SQLDatabase(Lrc::new(SQLDatabase {
                    span: r.range.to_span(),
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
                let r = report_and_continue!(r);
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
    },
};

fn visit_dirs(
    span: Span,
    dir: &Path,
    depth: i8,
    max_depth: i8,
    cb: &mut dyn FnMut(&std::fs::DirEntry) -> ParseResult<()>,
) -> ParseResult<()> {
    let entries = std::fs::read_dir(dir).map_err(|err| span.parse_err(err.to_string()))?;
    for entry in entries {
        let entry = entry.map_err(|err| span.parse_err(err.to_string()))?;
        let path = entry.path();
        if path.is_dir() && depth < max_depth {
            visit_dirs(span, &path, depth + 1, max_depth, cb)?;
        } else {
            cb(&entry)?;
        }
    }
    Ok(())
}

fn parse_default(span: Span, dir: &Path) -> ParseResult<Vec<DBMigration>> {
    let mut migrations = vec![];
    static FILENAME_RE: Lazy<Regex> =
        Lazy::new(|| Regex::new(r"^(\d+)_([^.]+)\.(up|down).sql$").unwrap());

    visit_dirs(span, dir, 0, 0, &mut |entry| -> ParseResult<()> {
        let path = entry.path();
        let name = entry.file_name();
        let name = name.to_str().ok_or(span.parse_err(format!(
            "invalid migration filename: {}",
            name.to_string_lossy()
        )))?;

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
            .ok_or(span.parse_err(format!("invalid migration filename: {}", name)))?;
        if captures[3].eq("up") {
            migrations.push(DBMigration {
                file_name: name.to_string(),
                description: captures[2].to_string(),
                number: captures[1]
                    .parse::<u64>()
                    .map_err(|err| span.parse_err(err.to_string()))?,
            });
        }
        Ok(())
    })?;

    migrations.sort_by_key(|m| m.number);
    Ok(migrations)
}

fn parse_drizzle(span: Span, dir: &Path) -> ParseResult<Vec<DBMigration>> {
    let mut migrations = vec![];

    static FILENAME_RE: Lazy<Regex> = Lazy::new(|| Regex::new(r"^(\d+)_([^.]+)\.sql$").unwrap());

    visit_dirs(span, dir, 0, 0, &mut |entry| -> ParseResult<()> {
        let path = entry.path();
        let name = entry.file_name();
        let name = name.to_str().ok_or(span.parse_err(format!(
            "invalid migration filename: {}",
            name.to_string_lossy()
        )))?;

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
            .ok_or(span.parse_err(format!("invalid migration filename: {}", name)))?;
        migrations.push(DBMigration {
            file_name: name.to_string(),
            description: captures[2].to_string(),
            number: captures[1]
                .parse::<u64>()
                .map_err(|err| span.parse_err(err.to_string()))?,
        });

        Ok(())
    })?;
    Ok(migrations)
}

fn parse_prisma(span: Span, dir: &Path) -> ParseResult<Vec<DBMigration>> {
    let mut migrations = vec![];

    static FILENAME_RE: Lazy<Regex> = Lazy::new(|| Regex::new(r"^(\d+)_(.*)$").unwrap());

    visit_dirs(span, dir, 0, 1, &mut |entry| -> ParseResult<()> {
        let path = entry.path();
        let name = entry.file_name();
        let name = name.to_str().ok_or(span.parse_err(format!(
            "invalid migration filename: {}",
            name.to_string_lossy()
        )))?;
        if name != "migration.sql" {
            return Ok(());
        }
        let dir_name = path
            .parent()
            .ok_or(span.parse_err("migration directory has no parent"))?
            .file_name()
            .ok_or(span.parse_err("migration directory has no name"))?
            .to_str()
            .ok_or(span.parse_err("migration directory has invalid name"))?;

        // Ensure the file name matches the regex.
        let captures = FILENAME_RE
            .captures(dir_name)
            .ok_or(span.parse_err(format!("invalid migration directory name: {}", dir_name)))?;
        migrations.push(DBMigration {
            file_name: path
                .strip_prefix(dir)
                .map_err(|_| {
                    span.parse_err(format!(
                        "migration directory is not a subdirectory of {}",
                        dir.display()
                    ))
                })?
                .to_string_lossy()
                .to_string(),
            description: captures[2].to_string(),
            number: captures[1]
                .parse::<u64>()
                .map_err(|err| span.parse_err(err.to_string()))?,
        });
        Ok(())
    })?;
    Ok(migrations)
}

fn parse_migrations(
    span: Span,
    dir: &Path,
    source: Option<&MigrationFileSource>,
) -> ParseResult<Vec<DBMigration>> {
    if !dir.exists() {
        return Err(span.parse_err("migrations directory does not exist"));
    } else if !dir.is_dir() {
        return Err(span.parse_err("migrations path is not a directory"));
    }

    let mut migrations = match source {
        Some(MigrationFileSource::Drizzle) => parse_drizzle(span, dir),
        Some(MigrationFileSource::Prisma) => parse_prisma(span, dir),
        _ => parse_default(span, dir),
    }?;
    migrations.sort_by_key(|m| m.number);
    Ok(migrations)
}

pub fn resolve_database_usage(data: &ResolveUsageData, db: Lrc<SQLDatabase>) -> Option<Usage> {
    match &data.expr.kind {
        UsageExprKind::MethodCall(_)
        | UsageExprKind::FieldAccess(_)
        | UsageExprKind::CallArg(_)
        | UsageExprKind::ConstructorArg(_) => Some(Usage::AccessDatabase(AccessDatabaseUsage {
            range: data.expr.range,
            db,
        })),

        _ => {
            data.expr.err("invalid use of database resource");
            None
        }
    }
}

#[derive(Debug)]
pub struct AccessDatabaseUsage {
    pub range: Range,
    pub db: Lrc<SQLDatabase>,
}
