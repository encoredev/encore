use std::collections::{HashMap, HashSet};
use std::path::Path;
use std::rc::Rc;

use anyhow::{Context, Result};

use crate::app::Service;
use crate::encore::parser::meta::v1;
use crate::legacymeta::schema::{loc_from_range, SchemaBuilder};
use crate::parser::parser::{ParseContext, ParseResult};
use crate::parser::resourceparser::bind::{Bind, BindKind};
use crate::parser::resources::apis::{authhandler, gateway};
use crate::parser::resources::infra::cron::CronJobSchedule;
use crate::parser::resources::infra::{cron, pubsub_subscription, pubsub_topic, sqldb};
use crate::parser::resources::Resource;
use crate::parser::types::ObjectId;
use crate::parser::usageparser::Usage;
use crate::parser::{respath, FilePath, Range};

mod api_schema;
mod schema;

const DEFAULT_API_GATEWAY_NAME: &'static str = "api-gateway";

pub fn compute_meta(
    pc: &ParseContext,
    parse: &ParseResult,
    services: &[Service],
) -> Result<v1::Data> {
    // The metadata assumes there's a single app root since it uses
    // relative paths to refer to files. Make this assumption for now.
    if pc.dir_roots.len() != 1 {
        anyhow::bail!("multiple app roots not supported");
    }
    let app_root = pc.dir_roots[0].as_path();

    let schema = SchemaBuilder::new(pc, app_root);
    MetaBuilder {
        pc,
        schema,
        parse,
        services,
        app_root,
        data: new_meta(),
    }
    .build()
}

struct MetaBuilder<'a> {
    pc: &'a ParseContext<'a>,
    schema: SchemaBuilder<'a>,
    parse: &'a ParseResult,
    services: &'a [Service],
    app_root: &'a Path,

    data: v1::Data,
}

impl<'a> MetaBuilder<'a> {
    pub fn build(mut self) -> Result<v1::Data> {
        // self.data.app_revision = parse_app_revision(&self.app_root)?;
        self.data.app_revision = std::env::var("ENCORE_APP_REVISION").unwrap_or_default();

        let mut svc_index = HashMap::new();
        let mut svc_to_pkg_index = HashMap::new();
        for svc in self.services {
            let rel_path = self.rel_path_string(svc.root.as_path())?;
            svc_to_pkg_index.insert(svc.name.clone(), self.data.pkgs.len());
            self.data.pkgs.push(v1::Package {
                rel_path: rel_path.clone(),
                name: svc.name.clone(),
                service_name: svc.name.clone(),
                rpc_calls: vec![], // added below
                secrets: vec![],   // added below

                doc: "".into(),      // TODO
                trace_nodes: vec![], // TODO?
            });

            svc_index.insert(svc.name.clone(), self.data.svcs.len());
            self.data.svcs.push(v1::Service {
                name: svc.name.clone(),
                rel_path,
                rpcs: vec![], // filled in later
                databases: self.svc_databases_names(svc),
                has_config: false, // TODO change when config is supported

                // We no longer care about migrations in a service, so just set
                // this to the empty array. The field is required for backwards compatibility.
                migrations: vec![],
            });
        }

        // Store resources that are dependent on other resources
        // so we can do another pass to resolve them.
        enum Dependent<'a> {
            // Depends on topic objects
            PubSubSubscription((&'a Bind, &'a pubsub_subscription::Subscription)),

            // Depends on endpoint objects
            CronJob((&'a Bind, &'a cron::CronJob)),

            // Depends on auth handler objects
            Gateway((&'a Bind, &'a gateway::Gateway)),
        }

        let mut dependent: Vec<Dependent> = Vec::new();
        let mut topic_idx: HashMap<ObjectId, usize> = HashMap::new();
        let mut endpoint_idx: HashMap<ObjectId, (usize, usize)> = HashMap::new();
        let mut topic_by_name: HashMap<String, usize> = HashMap::new();

        let mut auth_handlers: HashMap<ObjectId, Rc<authhandler::AuthHandler>> = HashMap::new();

        for b in &self.parse.binds {
            if b.kind != BindKind::Create {
                continue;
            }
            match &b.resource {
                // Do nothing for these resources:
                Resource::ServiceClient(_) => {}

                Resource::APIEndpoint(ep) => {
                    let request_schema = self
                        .schema
                        .transform_request(ep.encoding.raw_req_schema.clone())?;
                    let response_schema = self
                        .schema
                        .transform_response(ep.encoding.raw_resp_schema.clone())?;

                    let access_type: i32 = match (ep.expose, ep.require_auth) {
                        (false, _) => v1::rpc::AccessType::Private as i32,
                        (true, false) => v1::rpc::AccessType::Public as i32,
                        (true, true) => v1::rpc::AccessType::Auth as i32,
                    };

                    let rpc = v1::Rpc {
                        name: ep.name.clone(),
                        doc: ep.doc.clone(),
                        service_name: ep.service_name.clone(),
                        access_type,
                        request_schema,
                        response_schema,
                        proto: v1::rpc::Protocol::Regular as i32,
                        path: Some(ep.encoding.path.to_meta()),
                        http_methods: ep.encoding.methods.to_vec(),
                        tags: vec![],
                        sensitive: false,
                        loc: Some(loc_from_range(&self.app_root, &self.pc.file_set, ep.range)?),
                        allow_unauthenticated: !ep.require_auth,
                        expose: {
                            let mut map = HashMap::new();
                            if ep.expose {
                                map.insert(
                                    DEFAULT_API_GATEWAY_NAME.to_string(),
                                    v1::rpc::ExposeOptions {},
                                );
                            }
                            map
                        },
                    };

                    let service_idx = svc_index
                        .get(&ep.service_name)
                        .ok_or(anyhow::anyhow!("missing service: {}", ep.service_name))?
                        .to_owned();
                    let service = &mut self.data.svcs[service_idx];

                    if let Some(obj) = &b.object {
                        let ep_idx = service.rpcs.len();
                        endpoint_idx.insert(obj.id, (service_idx, ep_idx));
                    }
                    service.rpcs.push(rpc);
                }

                Resource::AuthHandler(ah) => {
                    if let Some(obj) = &b.object {
                        auth_handlers.insert(obj.id, ah.clone());
                    }
                }

                Resource::SQLDatabase(db) => {
                    self.data.sql_databases.push(self.sql_database(db)?);
                }

                Resource::PubSubTopic(topic) => {
                    let idx = self.data.pubsub_topics.len();
                    self.data.pubsub_topics.push(self.pubsub_topic(topic));
                    if let Some(obj) = &b.object {
                        topic_idx.insert(obj.id, idx);
                    }
                    topic_by_name.insert(topic.name.clone(), idx);
                }

                Resource::Secret(secret) => {
                    let service = self
                        .service_for_range(&secret.range)
                        .ok_or(anyhow::anyhow!(
                            "secrets must be loaded from within services"
                        ))?;
                    let pkg_idx = svc_to_pkg_index
                        .get(&service.name)
                        .ok_or(anyhow::anyhow!("missing service: {}", &service.name))?
                        .to_owned();
                    let pkg = &mut self.data.pkgs[pkg_idx];
                    pkg.secrets.push(secret.name.clone());
                }

                // Dependent resources
                // TODO: Include Cache Keyspace here too.
                Resource::PubSubSubscription(sub) => {
                    dependent.push(Dependent::PubSubSubscription((b, sub)));
                }
                Resource::CronJob(cj) => {
                    dependent.push(Dependent::CronJob((b, cj)));
                }
                Resource::Gateway(gw) => {
                    dependent.push(Dependent::Gateway((b, gw)));
                }
            }
        }

        // Make a second pass for resources that depend on other resources.
        for r in &dependent {
            match r {
                Dependent::PubSubSubscription((b, sub)) => {
                    let topic_idx = topic_idx
                        .get(&sub.topic.id)
                        .ok_or(anyhow::anyhow!("missing topic"))?
                        .to_owned();
                    let result = self.pubsub_subscription(b, sub)?;
                    let topic = &mut self.data.pubsub_topics[topic_idx];
                    topic.subscriptions.push(result);
                }

                Dependent::CronJob((_b, cj)) => {
                    let (svc_idx, ep_idx) = endpoint_idx
                        .get(&cj.endpoint.id)
                        .ok_or(anyhow::anyhow!("missing endpoint"))?
                        .to_owned();
                    let svc = &self.data.svcs[svc_idx];
                    let ep = &svc.rpcs[ep_idx];

                    let title = cj.title.clone().unwrap_or(cj.name.clone());
                    let result = v1::CronJob {
                        id: cj.name.clone(),
                        doc: cj.doc.to_owned(),
                        title,
                        endpoint: Some(v1::QualifiedName {
                            pkg: svc.rel_path.clone(),
                            name: ep.name.clone(),
                        }),
                        schedule: match &cj.schedule {
                            CronJobSchedule::Cron(expr) => format!("schedule:{}", expr.0),
                            CronJobSchedule::Every(mins) => format!("every:{}", mins),
                        },
                    };
                    self.data.cron_jobs.push(result);
                }

                Dependent::Gateway((_b, gw)) => {
                    let auth_handler = if let Some(auth_handler) = &gw.auth_handler {
                        let ah = auth_handlers
                            .get(&auth_handler.id)
                            .ok_or(anyhow::anyhow!("auth handler not found"))?;

                        let service_name = self
                            .service_for_range(&ah.range)
                            .ok_or(anyhow::anyhow!(
                                "unable to determine service for auth handler"
                            ))?
                            .name
                            .clone();

                        let loc = loc_from_range(&self.app_root, &self.pc.file_set, ah.range)?;
                        Some(v1::AuthHandler {
                            name: ah.name.clone(),
                            doc: ah.doc.clone().unwrap_or_default(),
                            pkg_path: loc.pkg_path.clone(),
                            pkg_name: loc.pkg_name.clone(),
                            loc: Some(loc),
                            params: Some(self.schema.typ(&ah.encoding.auth_param)?),
                            auth_data: Some(self.schema.typ(&ah.encoding.auth_param)?),
                            service_name,
                        })
                    } else {
                        None
                    };

                    let service_name = self
                        .service_for_range(&gw.range)
                        .ok_or(anyhow::anyhow!("unable to determine service for gateway"))?
                        .name
                        .clone();

                    if self.data.auth_handler.is_some() {
                        anyhow::bail!("multiple auth handlers not yet supported");
                    } else if self.data.gateways.len() > 0 {
                        anyhow::bail!("multiple gateways not yet supported");
                    }
                    self.data.auth_handler = auth_handler.clone();

                    if gw.name != "api-gateway" {
                        anyhow::bail!("only the 'api-gateway' gateway is supported");
                    }
                    let encore_name = DEFAULT_API_GATEWAY_NAME.to_string();

                    self.data.gateways.push(v1::Gateway {
                        encore_name,
                        explicit: Some(v1::gateway::Explicit {
                            service_name,
                            auth_handler,
                        }),
                    });
                }
            }
        }

        let mut seen_publishers = HashSet::new();
        let mut seen_calls = HashSet::new();
        for u in &self.parse.usages {
            match u {
                Usage::PublishTopic(publish) => {
                    let svc = self
                        .service_for_range(&publish.range)
                        .ok_or(anyhow::anyhow!("unable to determine service for publish"))?;

                    // Add the publisher if it hasn't already been seen.
                    let key = (svc.name.clone(), publish.topic.name.clone());
                    if seen_publishers.insert(key) {
                        let service_name = svc.name.clone();

                        let idx = topic_by_name
                            .get(&publish.topic.name)
                            .ok_or(anyhow::anyhow!("missing topic: {}", publish.topic.name))?
                            .to_owned();
                        let topic = &mut self.data.pubsub_topics[idx];
                        topic
                            .publishers
                            .push(v1::pub_sub_topic::Publisher { service_name });
                    }
                }
                Usage::CallEndpoint(call) => {
                    let src_service = self
                        .service_for_range(&call.range)
                        .ok_or(anyhow::anyhow!("unable to determine service for call"))?
                        .name
                        .clone();
                    let dst_service = call.endpoint.service_name.clone();
                    let dst_endpoint = call.endpoint.name.clone();

                    let dst_idx = svc_to_pkg_index
                        .get(&dst_service)
                        .ok_or(anyhow::anyhow!("missing service: {}", &dst_service))?
                        .to_owned();

                    let dst_pkg_rel_path = self.data.pkgs[dst_idx].rel_path.clone();

                    let src_idx = svc_to_pkg_index
                        .get(&src_service)
                        .ok_or(anyhow::anyhow!("missing service: {}", src_service))?
                        .to_owned();
                    let src_pkg = &mut self.data.pkgs[src_idx];

                    let call_key = (src_service, dst_service, dst_endpoint.clone());
                    if seen_calls.insert(call_key) {
                        src_pkg.rpc_calls.push(v1::QualifiedName {
                            pkg: dst_pkg_rel_path.clone(),
                            name: dst_endpoint,
                        });
                    }
                }
                Usage::ReferenceEndpoint(_) => {}
            }
        }

        // Sort the endpoints for deterministic output.
        for svc in &mut self.data.svcs {
            svc.rpcs.sort_by(|a, b| a.name.cmp(&b.name));
        }

        // Sort the packages for deterministic output.
        self.data.pkgs.sort_by(|a, b| a.name.cmp(&b.name));

        // Remove duplicate secrets.
        for pkg in &mut self.data.pkgs {
            pkg.secrets.sort();
            pkg.secrets.dedup();
        }

        // If there is no gateway, add a default one.
        if self.data.gateways.len() == 0 {
            self.data.gateways.push(v1::Gateway {
                encore_name: "api-gateway".to_string(),
                explicit: None,
            });
        }

        self.data.decls = self.schema.into_decls();
        Ok(self.data)
    }

    fn svc_databases_names(&self, svc: &Service) -> Vec<String> {
        let mut dbs: Vec<String> = svc
            .binds
            .iter()
            .filter_map(|b| match &b.resource {
                Resource::SQLDatabase(db) => Some(db.name.clone()),
                _ => None,
            })
            .collect();

        // Sort the result for deterministic output.
        dbs.sort();
        dbs
    }

    fn pubsub_topic(&self, topic: &pubsub_topic::Topic) -> v1::PubSubTopic {
        let mut topic = v1::PubSubTopic {
            name: topic.name.clone(),
            doc: topic.doc.clone(),
            message_type: None,           // TODO
            delivery_guarantee: 0,        // TODO
            ordering_key: "".to_string(), // TODO
            publishers: vec![],           // TODO
            subscriptions: vec![],        // TODO
        };

        let mut seen_publishers = HashSet::new();
        let _add_publisher = |svc_name: &str| {
            if !seen_publishers.contains(svc_name) {
                topic.publishers.push(v1::pub_sub_topic::Publisher {
                    service_name: svc_name.to_string(),
                });
                seen_publishers.insert(svc_name.to_string());
            }
        };

        // Sort the publishers for deterministic output.
        topic
            .publishers
            .sort_by(|a, b| a.service_name.cmp(&b.service_name));

        // TODO: Usage parsing not yet implemented

        topic
    }

    fn pubsub_subscription(
        &self,
        bind: &Bind,
        sub: &pubsub_subscription::Subscription,
    ) -> Result<v1::pub_sub_topic::Subscription> {
        let service_name = self
            .service_for_range(&bind.range.unwrap())
            .ok_or(anyhow::anyhow!(
                "unable to determine service for subscription"
            ))?
            .name
            .clone();

        Ok(v1::pub_sub_topic::Subscription {
            name: sub.name.clone(),
            service_name,
            ack_deadline: sub.config.ack_deadline.as_nanos() as i64,
            message_retention: sub.config.message_retention.as_nanos() as i64,
            retry_policy: Some(v1::pub_sub_topic::RetryPolicy {
                min_backoff: sub.config.min_retry_backoff.as_nanos() as i64,
                max_backoff: sub.config.max_retry_backoff.as_nanos() as i64,
                max_retries: sub.config.max_retries as i64,
            }),
        })
    }

    fn sql_database(&self, db: &sqldb::SQLDatabase) -> Result<v1::SqlDatabase> {
        // Transform the migrations into the metadata format.
        let (migration_rel_path, migrations) = match &db.migrations {
            Some(spec) => {
                let rel_path = self.rel_path_string(&spec.dir)?;
                let migrations = spec
                    .migrations
                    .iter()
                    .map(|m| v1::DbMigration {
                        filename: m.file_name.clone(),
                        description: m.description.clone(),
                        number: m.number,
                    })
                    .collect::<Vec<_>>();
                (Some(rel_path), migrations)
            }
            None => (None, vec![]),
        };

        Ok(v1::SqlDatabase {
            name: db.name.clone(),
            doc: db.doc.clone(),
            migration_rel_path,
            migrations,
        })
    }

    /// Compute the relative path from the app root.
    /// It reports an error if the path is not under the app root.
    fn rel_path<'b>(&self, path: &'b Path) -> Result<&'b Path> {
        let suffix = path.strip_prefix(self.app_root)?;
        Ok(suffix)
    }

    /// Compute the relative path from the app root as a String.
    fn rel_path_string(&self, path: &Path) -> Result<String> {
        let suffix = self.rel_path(path)?;
        let s = suffix
            .to_str()
            .ok_or(anyhow::anyhow!("invalid path: {:?}", path))?;
        Ok(s.to_string())
    }

    fn service_for_range(&self, range: &Range) -> Option<&Service> {
        let path = match range.file(&self.pc.file_set) {
            FilePath::Real(path) => path,
            FilePath::Custom(_) => return None,
        };
        self.services
            .iter()
            .find(|svc| path.starts_with(svc.root.as_path()))
    }
}

impl respath::Path {
    fn to_meta(&self) -> v1::Path {
        use respath::{Segment, ValueType};
        use v1::path_segment::{ParamType, SegmentType};
        v1::Path {
            r#type: v1::path::Type::Url as i32,
            segments: self
                .segments
                .iter()
                .map(|seg| match seg {
                    Segment::Literal(lit) => v1::PathSegment {
                        r#type: SegmentType::Literal as i32,
                        value_type: ParamType::String as i32,
                        value: lit.clone(),
                    },
                    Segment::Param { name, value_type } => v1::PathSegment {
                        r#type: SegmentType::Param as i32,
                        value_type: match value_type {
                            ValueType::String => ParamType::String as i32,
                            ValueType::Int => ParamType::Int as i32,
                            ValueType::Bool => ParamType::Bool as i32,
                        },
                        value: name.clone(),
                    },
                    Segment::Wildcard { name } => v1::PathSegment {
                        r#type: SegmentType::Wildcard as i32,
                        value_type: ParamType::String as i32,
                        value: name.clone(),
                    },
                    Segment::Fallback { name } => v1::PathSegment {
                        r#type: SegmentType::Fallback as i32,
                        value_type: ParamType::String as i32,
                        value: name.clone(),
                    },
                })
                .collect(),
        }
    }
}

fn new_meta() -> v1::Data {
    v1::Data {
        module_path: "app".to_string(),
        app_revision: "".to_string(),
        uncommitted_changes: false,
        decls: vec![],
        pkgs: vec![],
        svcs: vec![],
        auth_handler: None,
        cron_jobs: vec![],
        pubsub_topics: vec![],
        middleware: vec![],
        cache_clusters: vec![],
        experiments: vec![],
        metrics: vec![],
        sql_databases: vec![],
        gateways: vec![],
    }
}

#[cfg(test)]
mod tests {
    use swc_common::{Globals, GLOBALS};
    use tempdir::TempDir;

    use crate::app::collect_services;
    use crate::app::service_discovery::discover_services;
    use crate::parser::parser::Parser;
    use crate::parser::resourceparser::PassOneParser;
    use crate::testutil::testresolve::TestResolver;
    use crate::testutil::JS_RUNTIME_PATH;

    use super::*;

    fn parse(tmp_dir: &Path, src: &str) -> Result<v1::Data> {
        let globals = Globals::new();
        GLOBALS.set(&globals, || {
            let ar = txtar::from_str(src);
            ar.materialize(tmp_dir)?;

            let resolver = Box::new(TestResolver::new(tmp_dir, &ar));
            let pc = ParseContext::with_resolver(tmp_dir.to_path_buf(), &JS_RUNTIME_PATH, resolver)
                .unwrap();
            let _mods = pc.loader.load_archive(&tmp_dir, &ar).unwrap();

            let pass1 = PassOneParser::new(
                pc.file_set.clone(),
                pc.type_checker.clone(),
                Default::default(),
            );
            let parser = Parser::new(&pc, pass1);
            let parse = parser.parse()?;

            let discovered = discover_services(&pc.file_set, &parse.binds)?;
            let services = collect_services(&pc.file_set, &parse, discovered)?;
            compute_meta(&pc, &parse, &services)
        })
    }

    #[test]
    fn test_legacymeta() -> Result<()> {
        let src = r#"
-- foo.ts --
import { Bar } from './bar.ts';
-- bar.ts --
export const Bar = 5;
        "#;
        let tmp_dir = TempDir::new("tsparser-test")?;
        let meta = parse(tmp_dir.path(), src)?;
        assert_eq!(meta.svcs.len(), 0);
        Ok(())
    }
}

fn _parse_app_revision(dir: &Path) -> anyhow::Result<String> {
    duct::cmd!(
        "git",
        "-c",
        "log.showsignature=false",
        "show",
        "-s",
        "--format=%H:%ct"
    )
    .dir(dir)
    .read()
    .map_err(|e| anyhow::anyhow!("failed to run git: {}", e))
    .and_then(|s| {
        let (hash, _) = s.trim().split_once(":").context("invalid git output")?;
        Ok(hash.to_string())
    })
}
