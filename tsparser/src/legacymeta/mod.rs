use std::collections::{HashMap, HashSet};
use std::path::Path;
use std::rc::Rc;

use swc_common::errors::HANDLER;

use crate::encore::parser::meta::v1;
use crate::legacymeta::schema::{loc_from_range, SchemaBuilder};
use crate::parser::parser::{ParseContext, ParseResult, Service};
use crate::parser::resourceparser::bind::{Bind, BindKind};
use crate::parser::resources::apis::{authhandler, gateway};
use crate::parser::resources::infra::cron::CronJobSchedule;
use crate::parser::resources::infra::{cron, objects, pubsub_subscription, pubsub_topic, sqldb};
use crate::parser::resources::Resource;
use crate::parser::types::validation;
use crate::parser::types::{Object, ObjectId};
use crate::parser::usageparser::Usage;
use crate::parser::{respath, FilePath, Range};
use litparser::{ParseResult as PResult, ToParseErr};

mod api_schema;
mod schema;

const DEFAULT_API_GATEWAY_NAME: &str = "api-gateway";

pub fn compute_meta(pc: &ParseContext, parse: &ParseResult) -> PResult<v1::Data> {
    let app_root = pc.app_root.as_path();

    let schema = SchemaBuilder::new(pc, app_root);
    MetaBuilder {
        pc,
        schema,
        parse,
        app_root,
        data: new_meta(),
    }
    .build()
}

struct MetaBuilder<'a> {
    pc: &'a ParseContext,
    schema: SchemaBuilder<'a>,
    parse: &'a ParseResult,
    app_root: &'a Path,

    data: v1::Data,
}

impl MetaBuilder<'_> {
    pub fn build(mut self) -> PResult<v1::Data> {
        // self.data.app_revision = parse_app_revision(&self.app_root)?;
        self.data.app_revision = std::env::var("ENCORE_APP_REVISION").unwrap_or_default();

        let mut svc_index = HashMap::new();
        let mut svc_to_pkg_index = HashMap::new();
        for svc in &self.parse.services {
            let Some(rel_path) = self.rel_path_string(svc.root.as_path()) else {
                HANDLER.with(|h| {
                    h.err(&format!(
                        "unable to compute relative path to service: {}",
                        svc.name
                    ))
                });
                continue;
            };

            svc_to_pkg_index.insert(svc.name.clone(), self.data.pkgs.len());
            self.data.pkgs.push(v1::Package {
                rel_path: rel_path.clone(),
                name: svc.name.clone(),
                service_name: svc.name.clone(),
                doc: svc.doc.clone().unwrap_or_default(),

                rpc_calls: vec![],   // added below
                secrets: vec![],     // added below
                trace_nodes: vec![], // TODO?
            });

            svc_index.insert(svc.name.clone(), self.data.svcs.len());
            self.data.svcs.push(v1::Service {
                name: svc.name.clone(),
                rel_path,
                rpcs: vec![],      // filled in later
                databases: vec![], // filled in later
                buckets: vec![],   // filled in later
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
                Resource::Service(_) => {}
                Resource::ServiceClient(_) => {}

                Resource::APIEndpoint(ep) => {
                    let handshake_schema = self.schema.transform_handshake(ep)?;
                    let request_schema = self.schema.transform_request(ep)?;
                    let response_schema = self
                        .schema
                        .transform_response(ep.encoding.raw_resp_schema.clone().map(|s| s.take()))
                        .map_err(|err| {
                            let sp = ep
                                .encoding
                                .raw_resp_schema
                                .as_ref()
                                .map_or(ep.range.to_span(), |s| s.span());
                            sp.parse_err(err.to_string())
                        })?;

                    let access_type: i32 = match (ep.expose, ep.require_auth) {
                        (false, _) => v1::rpc::AccessType::Private as i32,
                        (true, false) => v1::rpc::AccessType::Public as i32,
                        (true, true) => v1::rpc::AccessType::Auth as i32,
                    };

                    let static_assets = ep
                        .static_assets
                        .as_ref()
                        .map(|sa| -> PResult<v1::rpc::StaticAssets> {
                            let dir_rel_path = self.rel_path_string(&sa.dir).ok_or(
                                sa.dir.parse_err("could not resolve static asset directory"),
                            )?;
                            let not_found_rel_path = sa
                                .not_found
                                .as_ref()
                                .map(|p| {
                                    self.rel_path_string(p).ok_or(
                                        p.parse_err("could not resolve static notFound path"),
                                    )
                                })
                                .transpose()?;
                            Ok(v1::rpc::StaticAssets {
                                dir_rel_path,
                                not_found_rel_path,
                            })
                        })
                        .transpose()?;

                    let rpc = v1::Rpc {
                        name: ep.name.clone(),
                        doc: ep.doc.clone(),
                        service_name: ep.service_name.clone(),
                        access_type,
                        handshake_schema,
                        request_schema,
                        response_schema,
                        proto: if ep.raw {
                            v1::rpc::Protocol::Raw
                        } else {
                            v1::rpc::Protocol::Regular
                        } as i32,
                        path: Some(ep.encoding.path.to_meta()),
                        http_methods: ep.encoding.methods.to_vec(),
                        tags: vec![],
                        sensitive: false,
                        loc: Some(loc_from_range(self.app_root, &self.pc.file_set, ep.range)?),
                        allow_unauthenticated: !ep.require_auth,
                        body_limit: ep.body_limit,
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
                        streaming_request: ep.streaming_request,
                        streaming_response: ep.streaming_response,
                        static_assets,
                    };

                    let Some(service_idx) =
                        svc_index.get(&ep.service_name).map(|idx| idx.to_owned())
                    else {
                        return Err(ep
                            .range
                            .to_span()
                            .parse_err(format!("missing service {}", ep.service_name)));
                    };
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

                Resource::Bucket(bkt) => {
                    self.data.buckets.push(self.bucket(bkt));
                }

                Resource::PubSubTopic(topic) => {
                    let idx = self.data.pubsub_topics.len();
                    let top = self.pubsub_topic(topic)?;
                    self.data.pubsub_topics.push(top);
                    if let Some(obj) = &b.object {
                        topic_idx.insert(obj.id, idx);
                    }
                    topic_by_name.insert(topic.name.clone(), idx);
                }

                Resource::Secret(secret) => {
                    let service = self.service_for_range(&secret.range).ok_or(
                        secret
                            .range
                            .parse_err("secrets must be loaded from within services"),
                    )?;

                    let pkg_idx = svc_to_pkg_index
                        .get(&service.name)
                        .ok_or(
                            secret
                                .range
                                .parse_err(format!("missing service: {}", &service.name)),
                        )?
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

        // Keep track of things we've seen so we can report errors pointing at
        // the previous definition when we see a duplicate.
        let mut first_gateway: Option<&gateway::Gateway> = None;
        let mut first_auth_handler: Option<&Object> = None;

        // Make a second pass for resources that depend on other resources.
        for r in &dependent {
            match r {
                Dependent::PubSubSubscription((b, sub)) => {
                    let topic_idx = topic_idx
                        .get(&sub.topic.id)
                        .ok_or_else(|| sub.topic.parse_err("topic not found"))?
                        .to_owned();
                    let result = self.pubsub_subscription(b, sub)?;
                    let topic = &mut self.data.pubsub_topics[topic_idx];
                    topic.subscriptions.push(result);
                }

                Dependent::CronJob((_b, cj)) => {
                    let (svc_idx, ep_idx) = endpoint_idx
                        .get(&cj.endpoint.id)
                        .ok_or(cj.endpoint.parse_err("endpoint not found"))?
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
                        let Some(ah) = auth_handlers.get(&auth_handler.id) else {
                            gw.range.err("auth handler not found");
                            continue;
                        };

                        let service_name = self
                            .service_for_range(&ah.range)
                            .ok_or(
                                ah.range
                                    .parse_err("unable to determine service for auth handler"),
                            )?
                            .name
                            .clone();

                        let loc = loc_from_range(self.app_root, &self.pc.file_set, ah.range)?;
                        let params = self
                            .schema
                            .typ(&ah.encoding.auth_param)
                            .map_err(|err| ah.encoding.auth_param.parse_err(err.to_string()))?;
                        let auth_data = self
                            .schema
                            .typ(&ah.encoding.auth_data)
                            .map_err(|err| ah.encoding.auth_data.parse_err(err.to_string()))?;
                        Some(v1::AuthHandler {
                            name: ah.name.clone(),
                            doc: ah.doc.clone().unwrap_or_default(),
                            pkg_path: loc.pkg_path.clone(),
                            pkg_name: loc.pkg_name.clone(),
                            loc: Some(loc),
                            params: Some(params),
                            auth_data: Some(auth_data),
                            service_name,
                        })
                    } else {
                        None
                    };

                    let service_name = self
                        .service_for_range(&gw.range)
                        .ok_or(
                            gw.range
                                .parse_err("unable to determine service for gateway"),
                        )?
                        .name
                        .clone();

                    if let Some(first) = first_gateway {
                        HANDLER.with(|h| {
                            h.struct_span_err(
                                gw.range.to_span(),
                                "multiple gateways not yet supported",
                            )
                            .span_help(first.range.to_span(), "previous gateway defined here")
                            .emit();
                        });
                        continue;
                    } else {
                        first_gateway = Some(gw);
                    }

                    if let Some(ah) = &gw.auth_handler {
                        if let Some(first) = first_auth_handler {
                            HANDLER.with(|h| {
                                h.struct_span_err(
                                    ah.range.to_span(),
                                    "multiple auth handlers not yet supported",
                                )
                                .span_help(
                                    first.range.to_span(),
                                    "previous auth handler defined here",
                                )
                                .emit();
                            });
                            continue;
                        } else {
                            first_auth_handler = Some(ah);
                        }
                    }

                    self.data.auth_handler.clone_from(&auth_handler);

                    if gw.name != "api-gateway" {
                        gw.range.err("only the 'api-gateway' gateway is supported");
                        continue;
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

        let mut bucket_perms = HashMap::new();
        for u in &self.parse.usages {
            match u {
                Usage::PublishTopic(publish) => {
                    let svc =
                        self.service_for_range(&publish.range)
                            .ok_or(publish.range.parse_err(
                                "unable to determine which service this 'publish' call is within",
                            ))?;

                    // Add the publisher if it hasn't already been seen.
                    let key = (svc.name.clone(), publish.topic.name.clone());
                    if seen_publishers.insert(key) {
                        let service_name = svc.name.clone();

                        let idx = topic_by_name
                            .get(&publish.topic.name)
                            .ok_or(publish.range.parse_err("could not resolve topic"))?
                            .to_owned();
                        let topic = &mut self.data.pubsub_topics[idx];
                        topic
                            .publishers
                            .push(v1::pub_sub_topic::Publisher { service_name });
                    }
                }
                Usage::AccessDatabase(access) => {
                    let Some(svc) = self.service_for_range(&access.range) else {
                        access
                            .range
                            .parse_err("cannot determine which service is accessing this database");
                        continue;
                    };

                    let idx = svc_index.get(&svc.name).unwrap();
                    self.data.svcs[*idx].databases.push(access.db.name.clone());
                }

                Usage::Bucket(access) => {
                    let Some(svc) = self.service_for_range(&access.range) else {
                        access
                            .range
                            .err("cannot determine which service is accessing this bucket");
                        continue;
                    };

                    use objects::Operation;
                    let ops = access.ops.iter().map(|op| match op {
                        Operation::DeleteObject => v1::bucket_usage::Operation::DeleteObject,
                        Operation::ListObjects => v1::bucket_usage::Operation::ListObjects,
                        Operation::ReadObjectContents => {
                            v1::bucket_usage::Operation::ReadObjectContents
                        }
                        Operation::WriteObject => v1::bucket_usage::Operation::WriteObject,
                        Operation::UpdateObjectMetadata => {
                            v1::bucket_usage::Operation::UpdateObjectMetadata
                        }
                        Operation::GetObjectMetadata => {
                            v1::bucket_usage::Operation::GetObjectMetadata
                        }
                        Operation::GetPublicUrl => v1::bucket_usage::Operation::GetPublicUrl,
                        Operation::SignedUploadUrl => v1::bucket_usage::Operation::SignedUploadUrl,
                    } as i32);

                    let idx = svc_index.get(&svc.name).unwrap();
                    bucket_perms
                        .entry((*idx, &access.bucket.name))
                        .or_insert(vec![])
                        .extend(ops);
                }

                Usage::CallEndpoint(call) => {
                    let src_service = self
                        .service_for_range(&call.range)
                        .ok_or(call.range.parse_err("unable to determine service for call"))?
                        .name
                        .clone();
                    let dst_service = call.endpoint.0.clone();
                    let dst_endpoint = call.endpoint.1.clone();

                    let dst_idx = svc_to_pkg_index
                        .get(&dst_service)
                        .ok_or(
                            call.range
                                .parse_err("could not resolve destination service"),
                        )?
                        .to_owned();

                    let dst_pkg_rel_path = self.data.pkgs[dst_idx].rel_path.clone();

                    let src_idx = svc_to_pkg_index
                        .get(&src_service)
                        .ok_or(call.range.parse_err("could not resolve calling service"))?
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
            }
        }

        // Add the computed bucket permissions to the services.
        for ((svc_idx, bucket), mut operations) in bucket_perms {
            // Make the bucket perms sorted and unique.
            operations.sort();
            operations.dedup();
            self.data.svcs[svc_idx].buckets.push(v1::BucketUsage {
                bucket: bucket.clone(),
                operations,
            });
        }

        // Sort the packages for deterministic output.
        self.data.pkgs.sort_by(|a, b| a.name.cmp(&b.name));

        // Remove duplicate secrets.
        for pkg in &mut self.data.pkgs {
            pkg.secrets.sort();
            pkg.secrets.dedup();
        }

        for svc in &mut self.data.svcs {
            // Remove duplicate database access.
            svc.databases.sort();
            svc.databases.dedup();

            // Sort buckets by name for deterministic output.
            svc.buckets.sort_by(|a, b| a.bucket.cmp(&b.bucket));

            // Sort the endpoints for deterministic output.
            svc.rpcs.sort_by(|a, b| a.name.cmp(&b.name));
        }

        // If there is no gateway, add a default one.
        if self.data.gateways.is_empty() {
            self.data.gateways.push(v1::Gateway {
                encore_name: "api-gateway".to_string(),
                explicit: None,
            });
        }

        self.data.decls = self.schema.into_decls();
        Ok(self.data)
    }

    fn pubsub_topic(&mut self, topic: &pubsub_topic::Topic) -> PResult<v1::PubSubTopic> {
        use pubsub_topic::DeliveryGuarantee;
        let message_type = self.schema.typ(&topic.message_type).map_err(|e| {
            topic
                .message_type
                .parse_err(format!("could not resolve message type: {}", e))
        })?;
        Ok(v1::PubSubTopic {
            name: topic.name.clone(),
            doc: topic.doc.clone(),
            message_type: Some(message_type),
            delivery_guarantee: match topic.delivery_guarantee {
                DeliveryGuarantee::AtLeastOnce => v1::pub_sub_topic::DeliveryGuarantee::AtLeastOnce,
                DeliveryGuarantee::ExactlyOnce => v1::pub_sub_topic::DeliveryGuarantee::ExactlyOnce,
            } as i32,
            ordering_key: topic.ordering_attribute.clone().unwrap_or_default(),
            publishers: vec![],    // filled in below
            subscriptions: vec![], // filled in below
        })
    }

    fn pubsub_subscription(
        &self,
        bind: &Bind,
        sub: &pubsub_subscription::Subscription,
    ) -> PResult<v1::pub_sub_topic::Subscription> {
        let service_name = self
            .service_for_range(&bind.range.unwrap_or(sub.range))
            .ok_or(
                sub.range
                    .parse_err("unable to determine which service the subscription belongs to"),
            )?
            .name
            .clone();

        Ok(v1::pub_sub_topic::Subscription {
            name: sub.name.clone(),
            service_name,
            ack_deadline: sub.config.ack_deadline.as_nanos() as i64,
            message_retention: sub.config.message_retention.as_nanos() as i64,
            max_concurrency: sub.config.max_concurrency.map(|v| v as i32),
            retry_policy: Some(v1::pub_sub_topic::RetryPolicy {
                min_backoff: sub.config.min_retry_backoff.as_nanos() as i64,
                max_backoff: sub.config.max_retry_backoff.as_nanos() as i64,
                max_retries: sub.config.max_retries as i64,
            }),
        })
    }

    fn sql_database(&self, db: &sqldb::SQLDatabase) -> PResult<v1::SqlDatabase> {
        // Transform the migrations into the metadata format.
        let (migration_rel_path, migrations, allow_non_sequential_migrations) = match &db.migrations
        {
            Some(spec) => {
                let rel_path = self
                    .rel_path_string(&spec.dir)
                    .ok_or(spec.parse_err("unable to resolve migration directory"))?;

                let migrations = spec
                    .migrations
                    .iter()
                    .map(|m| v1::DbMigration {
                        filename: m.file_name.clone(),
                        description: m.description.clone(),
                        number: m.number,
                    })
                    .collect::<Vec<_>>();
                (Some(rel_path), migrations, spec.non_seq_migrations)
            }
            None => (None, vec![], false),
        };

        Ok(v1::SqlDatabase {
            name: db.name.clone(),
            doc: db.doc.clone(),
            migration_rel_path,
            migrations,
            allow_non_sequential_migrations,
        })
    }

    fn bucket(&self, bkt: &objects::Bucket) -> v1::Bucket {
        v1::Bucket {
            name: bkt.name.clone(),
            doc: bkt.doc.clone(),
            versioned: bkt.versioned,
            public: bkt.public,
        }
    }

    /// Compute the relative path from the app root.
    /// It reports an error if the path is not under the app root.
    fn rel_path<'b>(&self, path: &'b Path) -> Option<&'b Path> {
        path.strip_prefix(self.app_root).ok()
    }

    /// Compute the relative path from the app root as a String.
    fn rel_path_string(&self, path: &Path) -> Option<String> {
        let suffix = self.rel_path(path)?;
        suffix.to_str().map(|s| s.to_string())
    }

    fn service_for_range(&self, range: &Range) -> Option<&Service> {
        let path = match range.file(&self.pc.file_set) {
            FilePath::Real(path) => path,
            FilePath::Custom(_) => return None,
        };
        self.parse
            .services
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
                .map(|seg| match seg.get() {
                    Segment::Literal(lit) => v1::PathSegment {
                        r#type: SegmentType::Literal as i32,
                        value_type: ParamType::String as i32,
                        value: lit.clone(),
                        validation: None,
                    },
                    Segment::Param {
                        name,
                        value_type,
                        validation,
                    } => v1::PathSegment {
                        r#type: SegmentType::Param as i32,
                        value_type: match value_type {
                            ValueType::String => ParamType::String as i32,
                            ValueType::Int => ParamType::Int as i32,
                            ValueType::Bool => ParamType::Bool as i32,
                        },
                        value: name.clone(),
                        validation: validation.as_ref().map(validation::Expr::to_pb),
                    },
                    Segment::Wildcard { name, validation } => v1::PathSegment {
                        r#type: SegmentType::Wildcard as i32,
                        value_type: ParamType::String as i32,
                        value: name.clone(),
                        validation: validation.as_ref().map(validation::Expr::to_pb),
                    },
                    Segment::Fallback { name, validation } => v1::PathSegment {
                        r#type: SegmentType::Fallback as i32,
                        value_type: ParamType::String as i32,
                        value: name.clone(),
                        validation: validation.as_ref().map(validation::Expr::to_pb),
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
        buckets: vec![],
        gateways: vec![],
        language: v1::Lang::Typescript as i32,
    }
}

#[cfg(test)]
mod tests {
    use swc_common::errors::{Handler, HANDLER};
    use swc_common::{Globals, SourceMap, GLOBALS};
    use tempdir::TempDir;

    use crate::parser::parser::Parser;
    use crate::parser::resourceparser::PassOneParser;
    use crate::testutil::testresolve::TestResolver;
    use crate::testutil::JS_RUNTIME_PATH;

    use super::*;

    fn parse(tmp_dir: &Path, src: &str) -> anyhow::Result<v1::Data> {
        let globals = Globals::new();
        let cm: Rc<SourceMap> = Default::default();
        let errs = Rc::new(Handler::with_tty_emitter(
            swc_common::errors::ColorConfig::Auto,
            true,
            false,
            Some(cm.clone()),
        ));

        GLOBALS.set(&globals, || -> anyhow::Result<_> {
            HANDLER.set(&errs, || -> anyhow::Result<_> {
                let ar = txtar::from_str(src);
                ar.materialize(tmp_dir)?;

                let resolver = Box::new(TestResolver::new(tmp_dir.to_path_buf(), ar.clone()));
                let pc = ParseContext::with_resolver(
                    tmp_dir.to_path_buf(),
                    JS_RUNTIME_PATH.clone(),
                    resolver,
                    cm,
                    errs.clone(),
                )
                .unwrap();
                let _mods = pc.loader.load_archive(tmp_dir, &ar).unwrap();

                let pass1 = PassOneParser::new(
                    pc.file_set.clone(),
                    pc.type_checker.clone(),
                    Default::default(),
                );
                let parser = Parser::new(&pc, pass1);
                let parse = parser.parse();
                let md = compute_meta(&pc, &parse)?;
                Ok(md)
            })
        })
    }

    #[test]
    fn test_legacymeta() -> anyhow::Result<()> {
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
