use anyhow::Result;
use litparser_derive::LitParser;
use swc_common::errors::HANDLER;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::{LitParser, Sp};

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    extract_type_param, iter_references, NamedClassResource, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::types::Type;
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;

#[derive(Debug, Clone)]
pub struct Topic {
    pub name: String,
    pub doc: Option<String>,
    pub delivery_guarantee: DeliveryGuarantee,
    pub ordering_attribute: Option<String>,
    pub message_type: Sp<Type>,
}

#[derive(Debug, Clone, Copy)]
pub enum DeliveryGuarantee {
    AtLeastOnce,
    ExactlyOnce,
}

#[derive(Debug, LitParser)]
#[allow(non_snake_case)]
struct DecodedTopicConfig {
    deliveryGuarantee: Option<String>,
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

        for r in iter_references::<PubSubTopicDefinition>(&module, &names) {
            let r = r?;
            let object = match &r.bind_name {
                None => None,
                Some(id) => pass
                    .type_checker
                    .resolve_obj(pass.module.clone(), &ast::Expr::Ident(id.clone())),
            };

            let message_type = pass
                .type_checker
                .resolve_type(pass.module.clone(), &r.message_type);

            let delivery_guarantee = r.config.delivery_guarantee()?;
            let resource = Resource::PubSubTopic(Lrc::new(Topic {
                name: r.resource_name.to_owned(),
                doc: r.doc_comment,
                delivery_guarantee,
                message_type,
                ordering_attribute: r.config.orderingAttribute,
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
        Ok(())
    },
};

#[derive(Debug)]
struct PubSubTopicDefinition {
    pub range: Range,
    pub resource_name: String,
    pub config: DecodedTopicConfig,
    pub doc_comment: Option<String>,
    pub bind_name: Option<ast::Ident>,
    pub message_type: ast::TsType,
}

impl ReferenceParser for PubSubTopicDefinition {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> Result<Option<Self>> {
        let Some(res) =
            NamedClassResource::<DecodedTopicConfig, 0, 1>::parse_resource_reference(module, path)?
        else {
            return Ok(None);
        };

        let Some(message_type) = extract_type_param(res.expr.type_args.as_deref(), 0) else {
            HANDLER.with(|h| h.span_err(res.expr.span, "missing message type parameter"));
            return Ok(None);
        };

        Ok(Some(Self {
            range: res.expr.span.into(),
            resource_name: res.resource_name,
            config: res.config,
            doc_comment: res.doc_comment,
            bind_name: res.bind_name,
            message_type: message_type.to_owned(),
        }))
    }
}

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
