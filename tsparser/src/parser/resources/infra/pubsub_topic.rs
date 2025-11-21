use std::ops::Deref;

use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::{report_and_continue, LitParser, ParseResult, Sp, ToParseErr};

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, BindName, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    extract_type_param, iter_references, resolve_object_for_bind_name, NamedClassResource,
    ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::types::{Generic, Type};
use crate::parser::usageparser::{MethodCall, ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;
use crate::span_err::ErrReporter;

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
#[allow(non_snake_case, dead_code)]
struct DecodedTopicConfig {
    deliveryGuarantee: Option<Sp<String>>,
    orderingAttribute: Option<String>,
}

impl DecodedTopicConfig {
    fn delivery_guarantee(&self) -> ParseResult<DeliveryGuarantee> {
        let Some(delivery_guarantee) = &self.deliveryGuarantee else {
            return Ok(DeliveryGuarantee::AtLeastOnce);
        };

        match delivery_guarantee.as_str() {
            "at-least-once" => Ok(DeliveryGuarantee::AtLeastOnce),
            "exactly-once" => Ok(DeliveryGuarantee::ExactlyOnce),
            _ => Err(delivery_guarantee.parse_err("invalid delivery guarantee")),
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
            let r = report_and_continue!(r);
            let object =
                resolve_object_for_bind_name(pass.type_checker, pass.module.clone(), &r.bind_name);

            let message_type = pass
                .type_checker
                .resolve_type(pass.module.clone(), &r.message_type);

            let delivery_guarantee = report_and_continue!(r.config.delivery_guarantee());
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
    },
};

#[derive(Debug)]
struct PubSubTopicDefinition {
    pub range: Range,
    pub resource_name: String,
    pub config: DecodedTopicConfig,
    pub doc_comment: Option<String>,
    pub bind_name: BindName,
    pub message_type: ast::TsType,
}

impl ReferenceParser for PubSubTopicDefinition {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        let Some(res) =
            NamedClassResource::<DecodedTopicConfig, 0, 1>::parse_resource_reference(module, path)?
        else {
            return Ok(None);
        };

        let Some(message_type) = extract_type_param(res.expr.type_args.as_deref(), 0) else {
            return Err(res.expr.parse_err("missing message type parameter"));
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
pub struct TopicUsage {
    pub range: Range,
    pub topic: Lrc<Topic>,
    pub ops: Vec<TopicOperation>,
}

pub fn resolve_topic_usage(data: &ResolveUsageData, topic: Lrc<Topic>) -> Option<Usage> {
    match &data.expr.kind {
        UsageExprKind::MethodCall(call) => {
            if call.method.as_ref() == "ref" {
                let Some(type_args) = call.call.type_args.as_deref() else {
                    call.call
                        .span
                        .err("expected a type argument in call to Topic.ref");
                    return None;
                };

                let Some(type_arg) = type_args.params.first() else {
                    call.call
                        .span
                        .err("expected a type argument in call to Topic.ref");
                    return None;
                };

                return parse_topic_ref(data, topic, call, type_arg);
            }

            if call.method.as_ref() == "publish" {
                Some(Usage::Topic(TopicUsage {
                    range: data.expr.range,
                    topic,
                    ops: vec![TopicOperation::Publish],
                }))
            } else {
                None
            }
        }
        UsageExprKind::ConstructorArg(_arg) => {
            // TODO validate: used as a subscription arg most likely
            None
        }
        _ => {
            data.expr.err("invalid topic usage");
            None
        }
    }
}

fn parse_topic_ref(
    data: &ResolveUsageData,
    topic: Lrc<Topic>,
    _call: &MethodCall,
    type_arg: &ast::TsType,
) -> Option<Usage> {
    fn process_type(
        data: &ResolveUsageData,
        sp: &swc_common::Span,
        t: &Type,
        depth: usize,
    ) -> Option<Vec<TopicOperation>> {
        if depth > 10 {
            // Prevent infinite recursion.
            return None;
        }

        match t {
            Type::Named(named) => {
                let ops = match named.obj.name.as_deref() {
                    Some("Publisher") => vec![TopicOperation::Publish],
                    _ => {
                        let underlying = data.type_checker.resolve_obj_type(&named.obj);
                        return process_type(data, sp, &underlying, depth + 1);
                    }
                };

                Some(ops)
            }

            Type::Class(cls) => {
                let ops = cls
                    .methods
                    .iter()
                    .filter_map(|method| {
                        let op = match method.as_str() {
                            "publish" => TopicOperation::Publish,
                            _ => {
                                // Ignore other methods.
                                return None;
                            }
                        };

                        Some(op)
                    })
                    .collect();
                Some(ops)
            }

            Type::Generic(Generic::Intersection(int)) => {
                let mut result = Vec::new();
                for t in &[&int.x, &int.y] {
                    if let Some(ops) = process_type(data, sp, t, depth + 1) {
                        result.extend(ops);
                    }
                }

                if result.is_empty() {
                    None
                } else {
                    Some(result)
                }
            }

            _ => {
                sp.err(&format!("unsupported topic permission type {t:#?}"));
                None
            }
        }
    }

    let typ = data
        .type_checker
        .resolve_type(data.module.clone(), type_arg);

    if let Some(ops) = process_type(data, &typ.span(), typ.deref(), 0) {
        Some(Usage::Topic(TopicUsage {
            range: data.expr.range,
            topic,
            ops,
        }))
    } else {
        typ.err("no topic permissions found in type argument");
        None
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TopicOperation {
    /// Publishing messages to the topic.
    Publish,
}
