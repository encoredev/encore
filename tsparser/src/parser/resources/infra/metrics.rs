use convert_case::{Case, Casing};
use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_common::Span;
use swc_ecma_ast as ast;

use litparser::{report_and_continue, ParseResult, Sp};

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, BindName, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    extract_type_param, iter_references, resolve_object_for_bind_name, validate_snake_case_name,
    NamedClassResourceOptionalConfig, ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::types::{FieldName, Type};
use crate::parser::usageparser::{ResolveUsageData, Usage, UsageExprKind};
use crate::parser::Range;

#[derive(Debug, Clone)]
pub struct Metric {
    pub name: String,
    pub doc: Option<String>,
    pub metric_type: MetricType,
    /// The type parameter for labels (for CounterGroup/GaugeGroup)
    pub label_type: Option<Sp<Type>>,
    /// The source location where this metric was defined.
    pub span: Span,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MetricType {
    Counter,
    CounterGroup,
    Gauge,
    GaugeGroup,
}

#[derive(Debug, LitParser, Default)]
#[allow(non_snake_case, dead_code)]
struct DecodedMetricConfig {
    // Reserved for future configuration options
}

pub const METRIC_PARSER: ResourceParser = ResourceParser {
    name: "metrics",
    interesting_pkgs: &[PkgPath("encore.dev/metrics")],

    run: |pass| {
        let names = TrackedNames::new(&[
            ("encore.dev/metrics", "Counter"),
            ("encore.dev/metrics", "CounterGroup"),
            ("encore.dev/metrics", "Gauge"),
            ("encore.dev/metrics", "GaugeGroup"),
        ]);
        let module = pass.module.clone();

        for r in iter_references::<MetricDefinition>(&module, &names) {
            let r = report_and_continue!(r);

            // Validate metric name is snake_case and doesn't start with "e_"
            if let Err(err_msg) = validate_snake_case_name(&r.resource_name, Some("e_")) {
                r.range.err(&format!(
                    "invalid metric name '{}': {}.",
                    r.resource_name, err_msg
                ));
                continue;
            }

            // Validate label names if this is a group metric
            if let Some(ref label_type) = r.label_type {
                // Resolve the interface to get the fields
                use crate::parser::resources::parseutil::resolve_interface;
                let label_type_sp = Sp::new(
                    r.range.to_span(),
                    pass.type_checker
                        .resolve_type(pass.module.clone(), label_type),
                );

                if let Some(iface) = resolve_interface(pass.type_checker, &label_type_sp) {
                    for field in &iface.fields {
                        if let FieldName::String(key) = &field.name {
                            let label_key = key.to_case(Case::Snake);

                            // Check if the label is named "service" (reserved)
                            if label_key == "service" {
                                r.range.err(&format!(
                                    "invalid label name '{}': the label name 'service' is reserved and automatically added by the Encore runtime",
                                    key
                                ));
                                continue;
                            }
                        }
                    }
                }
            }

            let object =
                resolve_object_for_bind_name(pass.type_checker, pass.module.clone(), &r.bind_name);

            let label_type = r
                .label_type
                .as_ref()
                .map(|lt| pass.type_checker.resolve_type(pass.module.clone(), lt));

            let resource = Resource::Metric(Lrc::new(Metric {
                name: r.resource_name.to_owned(),
                doc: r.doc_comment,
                metric_type: r.metric_type,
                label_type,
                span: r.range.to_span(),
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
struct MetricDefinition {
    pub range: Range,
    pub resource_name: String,
    /// Reserved for future configuration validation
    #[allow(dead_code)]
    pub config: Option<DecodedMetricConfig>,
    pub doc_comment: Option<String>,
    pub bind_name: BindName,
    pub metric_type: MetricType,
    /// Type parameter for labels (for *Group types)
    pub label_type: Option<ast::TsType>,
}

impl ReferenceParser for MetricDefinition {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        // Constructor: new Counter(name) or new Counter(name, config)
        // NAME_IDX=0 (name), CONFIG_IDX=1 (optional config)
        let Some(res) =
            NamedClassResourceOptionalConfig::<DecodedMetricConfig, 0, 1>::parse_resource_reference(module, path)?
        else {
            return Ok(None);
        };

        // Get the class name from the new expression callee to determine if it's Counter/Gauge
        let class_name = match res.expr.callee.as_ref() {
            ast::Expr::Ident(ident) => ident.sym.as_ref(),
            _ => return Ok(None),
        };

        // Determine if it's a counter or gauge
        let is_counter = class_name == "Counter" || class_name == "CounterGroup";

        // Check if it has a label type parameter (Group variants)
        let label_type = extract_type_param(res.expr.type_args.as_deref(), 0);
        let has_labels = label_type.is_some();

        // Determine the metric type
        let metric_type = match (is_counter, has_labels) {
            (true, false) => MetricType::Counter,
            (true, true) => MetricType::CounterGroup,
            (false, false) => MetricType::Gauge,
            (false, true) => MetricType::GaugeGroup,
        };

        Ok(Some(Self {
            range: res.expr.span.into(),
            resource_name: res.resource_name,
            config: res.config,
            doc_comment: res.doc_comment,
            bind_name: res.bind_name,
            metric_type,
            label_type: label_type.map(|lt| lt.to_owned()),
        }))
    }
}

#[derive(Debug)]
pub struct MetricUsage {
    pub range: Range,
    pub metric: Lrc<Metric>,
    pub ops: Vec<MetricOperation>,
}

pub fn resolve_metric_usage(data: &ResolveUsageData, metric: Lrc<Metric>) -> Option<Usage> {
    match &data.expr.kind {
        UsageExprKind::MethodCall(call) => {
            if call.method.as_ref() == "ref" || call.method.as_ref() == "with" {
                let ops = determine_metric_operations(&metric);
                return Some(Usage::Metric(MetricUsage {
                    range: data.expr.range,
                    metric,
                    ops,
                }));
            }

            // Determine the operation based on method name and metric type
            let operation = match call.method.as_ref() {
                "increment" => Some(MetricOperation::Increment),
                "set" => Some(MetricOperation::Set),
                _ => None,
            };

            operation.map(|op| {
                Usage::Metric(MetricUsage {
                    range: data.expr.range,
                    metric,
                    ops: vec![op],
                })
            })
        }
        UsageExprKind::ConstructorArg(_arg) => {
            // Metrics used as constructor args (similar to topics in subscriptions)
            None
        }
        _ => {
            data.expr.range.err("invalid metric usage");
            None
        }
    }
}

/// Determine what operations are available for a given metric type
fn determine_metric_operations(metric: &Metric) -> Vec<MetricOperation> {
    match metric.metric_type {
        MetricType::Counter | MetricType::CounterGroup => vec![MetricOperation::Increment],
        MetricType::Gauge | MetricType::GaugeGroup => vec![MetricOperation::Set],
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MetricOperation {
    /// Incrementing a counter.
    Increment,
    /// Setting a gauge value.
    Set,
}
