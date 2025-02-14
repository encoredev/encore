use std::rc::Rc;

use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_common::Spanned;
use swc_ecma_ast as ast;

use litparser::{report_and_continue, LitParser, Sp};

use crate::parser::resourceparser::bind::{BindData, BindKind, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{iter_references, NamedClassResource, TrackedNames};
use crate::parser::resources::Resource;
use crate::parser::types::Object;
use crate::parser::Range;
use crate::span_err::ErrReporter;

#[derive(Debug, Clone)]
pub struct Subscription {
    pub range: Range,
    pub topic: Sp<Rc<Object>>,
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
    pub max_concurrency: Option<u32>,
}

#[allow(non_snake_case)]
#[derive(Debug, LitParser)]
struct DecodedSubscriptionConfig {
    #[allow(dead_code)]
    handler: ast::Expr,
    #[allow(dead_code)]
    maxConcurrency: Option<u32>,
    ackDeadline: Option<std::time::Duration>,
    messageRetention: Option<std::time::Duration>,
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
            let r = report_and_continue!(r);
            let topic_expr = r.constructor_args[0].clone();
            if let Some(spread) = topic_expr.spread.as_ref() {
                spread.err("cannot use ... for PubSub topic reference");
                continue;
            }
            let object = match &r.bind_name {
                None => None,
                Some(id) => pass
                    .type_checker
                    .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
            };

            let Some(topic) = pass
                .type_checker
                .resolve_obj(pass.module.clone(), &topic_expr.expr)
            else {
                topic_expr.expr.err("cannot resolve topic reference");
                continue;
            };

            let resource = Resource::PubSubSubscription(Lrc::new(Subscription {
                range: r.range,
                topic: Sp::new(topic_expr.expr.span(), topic),
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
                    max_concurrency: r.config.maxConcurrency,
                },
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
    },
};
