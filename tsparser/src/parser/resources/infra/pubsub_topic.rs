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
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;

#[derive(Debug, Clone)]
pub struct Topic {
    pub name: String,
    pub doc: Option<String>,
    pub delivery_guarantee: DeliveryGuarantee,
    pub ordering_attribute: Option<String>,
}

#[derive(Debug, Clone, Copy)]
pub enum DeliveryGuarantee {
    AtLeastOnce,
    ExactlyOnce,
}

#[derive(Debug, LitParser)]
struct DecodedTopicConfig {
    #[allow(non_snake_case)]
    deliveryGuarantee: Option<String>,
    #[allow(non_snake_case)]
    orderingAttribute: Option<String>,
}

impl DecodedTopicConfig {
    fn delivery_guarantee(&self) -> Result<DeliveryGuarantee> {
        let Some(delivery_guarantee) = &self.deliveryGuarantee else {
            return Ok(DeliveryGuarantee::AtLeastOnce);
        };

        match delivery_guarantee.as_str() {
            "at-least-once" => Ok(DeliveryGuarantee::AtLeastOnce),
            "exactly-once" => Ok(DeliveryGuarantee::ExactlyOnce),
            _ => anyhow::bail!("invalid delivery guarantee"),
        }
    }
}

pub const TOPIC_PARSER: ResourceParser = ResourceParser {
    name: "pubsub_topic",
    interesting_pkgs: &[PkgPath("encore.dev/pubsub")],

    run: |pass| {
        let names = TrackedNames::new(&[("encore.dev/pubsub", "Topic")]);
        let module = pass.module.clone();

        for r in iter_references::<NamedClassResource<DecodedTopicConfig>>(&module, &names) {
            let r = r?;
            let object = match &r.bind_name {
                None => None,
                Some(id) => pass
                    .type_checker
                    .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone()))?,
            };

            let delivery_guarantee = r.config.delivery_guarantee()?;
            let resource = Resource::PubSubTopic(Lrc::new(Topic {
                name: r.resource_name.to_owned(),
                doc: r.doc_comment,
                delivery_guarantee,
                ordering_attribute: r.config.orderingAttribute,
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

#[derive(Debug)]
pub struct PublishUsage {
    pub range: Range,
    pub topic: Lrc<Topic>,
}

pub fn resolve_topic_usage(data: &ResolveUsageData, topic: Lrc<Topic>) -> Result<Option<Usage>> {
    Ok(match &data.expr.kind {
        UsageExprKind::MethodCall(method) => {
            if method.method.as_ref() == "publish" {
                Some(Usage::PublishTopic(PublishUsage {
                    range: data.expr.range,
                    topic,
                }))
            } else {
                None
            }
        }
        UsageExprKind::ConstructorArg(_arg) => {
            // TODO validate: used as a subscription arg most likely
            None
        }
        _ => anyhow::bail!("invalid topic usage"),
    })
}
