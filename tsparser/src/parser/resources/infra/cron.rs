use std::rc::Rc;

use anyhow::Result;
use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::LitParser;

use crate::parser::resourceparser::bind::{BindData, BindKind};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{iter_references, NamedClassResource, TrackedNames};
use crate::parser::resources::Resource;
use crate::parser::types::Object;

#[derive(Debug, Clone)]
pub struct CronJob {
    pub name: String,
    pub title: Option<String>,
    pub doc: Option<String>,
    pub schedule: CronJobSchedule,
    pub endpoint: Rc<Object>,
}

#[derive(Debug, Clone)]
pub enum CronJobSchedule {
    Every(u32), // every N minutes
    Cron(CronExpr),
}

#[derive(Debug, Clone)]
pub struct CronExpr(pub String);

#[derive(Debug, LitParser)]
struct DecodedCronJobConfig {
    endpoint: ast::Expr,
    title: Option<String>,
    every: Option<std::time::Duration>,
    schedule: Option<CronExpr>,
}

pub const CRON_PARSER: ResourceParser = ResourceParser {
    name: "cron",
    interesting_pkgs: &[PkgPath("encore.dev/cron")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/cron", "CronJob")]);

        let module = pass.module.clone();
        type Res = NamedClassResource<DecodedCronJobConfig>;
        for r in iter_references::<Res>(&module, &names) {
            let r = r?;
            let object = match &r.bind_name {
                None => None,
                Some(id) => pass
                    .type_checker
                    .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone()))?,
            };

            let endpoint = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &r.config.endpoint)?
                .ok_or(anyhow::anyhow!("can't resolve endpoint"))?;

            let schedule = r.config.schedule()?;
            let resource = Resource::CronJob(Lrc::new(CronJob {
                name: r.resource_name.to_owned(),
                doc: r.doc_comment,
                title: r.config.title,
                endpoint,
                schedule,
            }));
            pass.add_resource(resource.clone());
            pass.add_bind(BindData {
                range: r.range,
                resource,
                object,
                kind: BindKind::Create,
                ident: r.bind_name,
            });
        }
        Ok(())
    },
};

impl LitParser for CronExpr {
    fn parse_lit(input: &ast::Expr) -> anyhow::Result<Self> {
        match input {
            ast::Expr::Lit(ast::Lit::Str(str)) => {
                // Ensure the cron expression is valid
                let expr = str.value.as_ref();
                cron_parser::parse(expr, &chrono::Utc::now())?;
                Ok(CronExpr(expr.to_string()))
            }
            _ => anyhow::bail!("expected cron expression, got {:?}", input),
        }
    }
}

impl DecodedCronJobConfig {
    fn schedule(&self) -> Result<CronJobSchedule> {
        match (self.every, self.schedule.as_ref()) {
            (None, Some(schedule)) => Ok(CronJobSchedule::Cron(schedule.clone())),
            (Some(every), None) => {
                // TODO introduce more robust validation and error reporting here.
                let secs = every.as_secs();
                if secs % 60 != 0 {
                    anyhow::bail!("`every` must be a multiple of 60 seconds");
                }
                let mins = secs / 60;
                if mins > (24 * 60) {
                    anyhow::bail!("`every` must be at most 24 hours");
                }
                Ok(CronJobSchedule::Every(mins as u32))
            }
            (None, None) => {
                anyhow::bail!("expected either `every` or `schedule` to be set");
            }
            (Some(_), Some(_)) => {
                anyhow::bail!("expected either `every` or `schedule` to be set, not both");
            }
        }
    }
}
