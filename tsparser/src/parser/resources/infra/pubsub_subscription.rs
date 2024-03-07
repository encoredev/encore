use std::rc::Rc;

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
pub struct Subscription {
    pub topic: Rc<Object>,
    pub name: String,
    pub doc: Option<String>,
    pub config: SubscriptionConfig,
}

#[derive(Debug, Clone)]
pub struct SubscriptionConfig {
    pub ack_deadline: std::time::Duration,
    pub message_retention: std::time::Duration,
    pub min_retry_backoff: std::time::Duration,
    pub max_retry_backoff: std::time::Duration,
    pub max_retries: u32,
}

#[allow(non_snake_case)]
#[derive(Debug, LitParser)]
struct DecodedSubscriptionConfig {
    #[allow(dead_code)]
    handler: ast::Expr,
    #[allow(dead_code, non_snake_case)]
    maxConcurrency: Option<u32>,
    #[allow(non_snake_case)]
    ackDeadline: Option<std::time::Duration>,
    #[allow(non_snake_case)]
    messageRetention: Option<std::time::Duration>,
    #[allow(non_snake_case)]
    retryPolicy: Option<DecodedRetryPolicy>,
}

#[allow(non_snake_case)]
#[derive(Debug, LitParser)]
struct DecodedRetryPolicy {
    minBackoff: Option<std::time::Duration>,
    maxBackoff: Option<std::time::Duration>,
    maxRetries: Option<u32>,
}

pub const SUBSCRIPTION_PARSER: ResourceParser = ResourceParser {
    name: "pubsub_subscription",
    interesting_pkgs: &[PkgPath("encore.dev/pubsub")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/pubsub", "Subscription")]);
        let module = pass.module.clone();

        type Res = NamedClassResource<DecodedSubscriptionConfig, 1, 2>;
        for r in iter_references::<Res>(&module, &names) {
            let r = r?;
            let topic_expr = r.constructor_args[0].clone();
            if topic_expr.spread.is_some() {
                anyhow::bail!("can't use ... for PubSub topic reference");
            }
            let object = match &r.bind_name {
                None => None,
                Some(id) => pass
                    .type_checker
                    .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone()))?,
            };

            let topic = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &topic_expr.expr)?
                .ok_or(anyhow::anyhow!("can't resolve topic"))?;

            let resource = Resource::PubSubSubscription(Lrc::new(Subscription {
                topic,
                name: r.resource_name.to_owned(),
                doc: r.doc_comment,
                config: SubscriptionConfig {
                    ack_deadline: r
                        .config
                        .ackDeadline
                        .unwrap_or(std::time::Duration::from_secs(30)),
                    message_retention: r
                        .config
                        .messageRetention
                        .unwrap_or(std::time::Duration::from_secs(7 * 24 * 60 * 60)),
                    min_retry_backoff: r
                        .config
                        .retryPolicy
                        .as_ref()
                        .and_then(|p| p.minBackoff)
                        .unwrap_or(std::time::Duration::from_secs(10)),
                    max_retry_backoff: r
                        .config
                        .retryPolicy
                        .as_ref()
                        .and_then(|p| p.maxBackoff)
                        .unwrap_or(std::time::Duration::from_secs(10 * 60)),
                    max_retries: r
                        .config
                        .retryPolicy
                        .as_ref()
                        .and_then(|p| p.maxRetries)
                        .unwrap_or(100),
                },
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
