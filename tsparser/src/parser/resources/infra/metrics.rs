use litparser_derive::LitParser;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use litparser::{report_and_continue, ParseResult, Sp, ToParseErr};

use crate::parser::module_loader::Module;
use crate::parser::resourceparser::bind::{BindData, BindKind, BindName, ResourceOrPath};
use crate::parser::resourceparser::paths::PkgPath;
use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::parseutil::{
    extract_type_param, iter_references, resolve_object_for_bind_name, NamedClassResource,
    ReferenceParser, TrackedNames,
};
use crate::parser::resources::Resource;
use crate::parser::types::Type;
use crate::parser::Range;

#[derive(Debug, Clone)]
pub struct Metric {
    pub name: String,
    pub doc: Option<String>,
    pub metric_type: MetricType,
    /// The type parameter for labels (for CounterGroup/GaugeGroup)
    pub label_type: Option<Sp<Type>>,
    /// The type parameter for values (number/bigint)
    pub value_type: Sp<Type>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MetricType {
    Counter,
    CounterGroup,
    Gauge,
    GaugeGroup,
}

impl MetricType {
    fn has_labels(&self) -> bool {
        matches!(self, MetricType::CounterGroup | MetricType::GaugeGroup)
    }

    fn is_counter(&self) -> bool {
        matches!(self, MetricType::Counter | MetricType::CounterGroup)
    }
}

#[derive(Debug, LitParser)]
#[allow(non_snake_case, dead_code)]
struct DecodedMetricConfig {
    // Reserved for future configuration options
}

pub const COUNTER_PARSER: ResourceParser = ResourceParser {
    name: "metric_counter",
    interesting_pkgs: &[PkgPath("encore.dev/metrics")],

    run: |pass| {
        let names = TrackedNames::new(&[
            ("encore.dev/metrics", "Counter"),
            ("encore.dev/metrics", "CounterGroup"),
        ]);
        let module = pass.module.clone();

        for r in iter_references::<MetricDefinition>(&module, &names) {
            let r = report_and_continue!(r);
            let object =
                resolve_object_for_bind_name(pass.type_checker, pass.module.clone(), &r.bind_name);

            let label_type = r.label_type.as_ref().map(|lt| {
                pass.type_checker
                    .resolve_type(pass.module.clone(), lt)
            });

            let value_type = pass
                .type_checker
                .resolve_type(pass.module.clone(), &r.value_type);

            let resource = Resource::Metric(Lrc::new(Metric {
                name: r.resource_name.to_owned(),
                doc: r.doc_comment,
                metric_type: r.metric_type,
                label_type,
                value_type,
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

pub const GAUGE_PARSER: ResourceParser = ResourceParser {
    name: "metric_gauge",
    interesting_pkgs: &[PkgPath("encore.dev/metrics")],

    run: |pass| {
        let names = TrackedNames::new(&[
            ("encore.dev/metrics", "Gauge"),
            ("encore.dev/metrics", "GaugeGroup"),
        ]);
        let module = pass.module.clone();

        for r in iter_references::<MetricDefinition>(&module, &names) {
            let r = report_and_continue!(r);
            let object =
                resolve_object_for_bind_name(pass.type_checker, pass.module.clone(), &r.bind_name);

            let label_type = r.label_type.as_ref().map(|lt| {
                pass.type_checker
                    .resolve_type(pass.module.clone(), lt)
            });

            let value_type = pass
                .type_checker
                .resolve_type(pass.module.clone(), &r.value_type);

            let resource = Resource::Metric(Lrc::new(Metric {
                name: r.resource_name.to_owned(),
                doc: r.doc_comment,
                metric_type: r.metric_type,
                label_type,
                value_type,
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
    pub config: DecodedMetricConfig,
    pub doc_comment: Option<String>,
    pub bind_name: BindName,
    pub metric_type: MetricType,
    /// Type parameter for labels (for *Group types)
    pub label_type: Option<ast::TsType>,
    /// Type parameter for value (number/bigint)
    pub value_type: ast::TsType,
}

impl ReferenceParser for MetricDefinition {
    fn parse_resource_reference(
        module: &Module,
        path: &swc_ecma_visit::AstNodePath,
    ) -> ParseResult<Option<Self>> {
        // Determine which class we're looking for based on the node
        // We need to check the actual new expression to determine the type

        // Try to parse as Counter or Gauge (no labels, 1 type param)
        // Constructor: new Counter<T>(name, config)
        // NAME_IDX=0 (name), CONFIG_IDX=1 (config)
        if let Some(res) =
            NamedClassResource::<DecodedMetricConfig, 0, 1>::parse_resource_reference(module, path)?
        {
            // Check if it has 1 type param (Counter/Gauge) or 2 (CounterGroup/GaugeGroup)
            let type_params = res.expr.type_args.as_ref().map(|ta| ta.params.len()).unwrap_or(0);

            if type_params == 1 {
                // Counter or Gauge: new Counter<number>(name, config)
                let Some(value_type) = extract_type_param(res.expr.type_args.as_deref(), 0) else {
                    return Err(res.expr.parse_err("missing value type parameter"));
                };

                return Ok(Some(Self {
                    range: res.expr.span.into(),
                    resource_name: res.resource_name,
                    config: res.config,
                    doc_comment: res.doc_comment,
                    bind_name: res.bind_name,
                    metric_type: MetricType::Counter, // Will be overridden by parser context
                    label_type: None,
                    value_type: value_type.to_owned(),
                }));
            } else if type_params == 2 {
                // CounterGroup or GaugeGroup: new CounterGroup<Labels, number>(name, config)
                let Some(label_type) = extract_type_param(res.expr.type_args.as_deref(), 0) else {
                    return Err(res.expr.parse_err("missing label type parameter"));
                };
                let Some(value_type) = extract_type_param(res.expr.type_args.as_deref(), 1) else {
                    return Err(res.expr.parse_err("missing value type parameter"));
                };

                return Ok(Some(Self {
                    range: res.expr.span.into(),
                    resource_name: res.resource_name,
                    config: res.config,
                    doc_comment: res.doc_comment,
                    bind_name: res.bind_name,
                    metric_type: MetricType::CounterGroup, // Will be overridden by parser context
                    label_type: Some(label_type.to_owned()),
                    value_type: value_type.to_owned(),
                }));
            }
        }

        Ok(None)
    }
}
