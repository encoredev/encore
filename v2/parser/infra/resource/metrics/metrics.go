package metrics

import (
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	literals2 "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil2 "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

import (
	"go/ast"
)

type Metric struct {
	Name string // The unique name of the pub sub topic
	Doc  string // The documentation on the pub sub topic
}

func (m *Metric) Kind() resource.Kind { return resource.Metric }

// metricConstructor describes a particular metric constructor function.
type metricConstructor struct {
	FuncName    string
	ConfigName  string
	ConfigParse configParseFunc
	HasLabels   bool
	MetricKind  meta.Metric_MetricKind
}

var metricConstructors = []metricConstructor{
	{"NewCounter", "CounterConfig", parseCounterConfig, false, meta.Metric_COUNTER},
	{"NewCounterGroup", "CounterConfig", parseCounterConfig, true, meta.Metric_COUNTER},
	{"NewGauge", "GaugeConfig", parseGaugeConfig, false, meta.Metric_GAUGE},
	{"NewGaugeGroup", "GaugeConfig", parseGaugeConfig, true, meta.Metric_GAUGE},
}

var MetricParser = &resource.Parser{
	Name:      "Metric",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/metrics"},
	Run: func(p *resource.Pass) []resource.Resource {
		var (
			names []pkginfo.QualifiedName
			specs = make(map[pkginfo.QualifiedName]*parseutil2.ResourceCreationSpec)
		)
		for _, c := range metricConstructors {
			name := pkginfo.QualifiedName{PkgPath: "encore.dev/metrics", Name: c.FuncName}
			names = append(names, name)

			numTypeArgs := 1
			if c.HasLabels {
				numTypeArgs = 2
			}

			c := c // capture for closure
			parseFn := func(d parseutil2.ParseData) resource.Resource {
				return parseMetric(c, d)
			}

			spec := &parseutil2.ResourceCreationSpec{
				AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
				MinTypeArgs: numTypeArgs,
				MaxTypeArgs: numTypeArgs,
				Parse:       parseFn,
			}
			specs[name] = spec
		}

		var resources []resource.Resource
		parseutil2.FindPkgNameRefs(p.Pkg, names, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			spec := specs[name]
			r := parseutil2.ParseResourceCreation(p, spec, parseutil2.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
			if r != nil {
				resources = append(resources, r)
			}
		})
		return resources
	},
}

func parseMetric(c metricConstructor, d parseutil2.ParseData) resource.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Addf(d.Call.Pos(), "%s requires two arguments: the metric name and the metric configuration",
			displayName)
		return nil
	}

	// Validate the metric name.
	metricName := parseutil2.ParseResourceName(d.Pass.Errs, displayName, "metric name",
		d.Call.Args[0], parseutil2.SnakeName, "e_")
	if metricName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	// Validate the metric value type.
	valueType := d.TypeArgs[0]
	if c.HasLabels {
		valueType = d.TypeArgs[1]
	}
	if valueType.Family() != schema.Builtin {
		d.Pass.Errs.Add(d.Call.Pos(), "metric value type must be a builtin type")
		return nil
	}

	m := &Metric{
		Name: metricName,
		Doc:  d.Doc,
	}

	// TODO(andre) Validate the label type

	// Parse and validate the metric configuration.
	cfgLit, ok := literals2.ParseStruct(d.Pass.Errs, d.File, "metrics.MetricConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
	}
	c.ConfigParse(c, d, cfgLit, m)
	return m
}

type configParseFunc func(c metricConstructor, d parseutil2.ParseData, cfgLit *literals2.Struct, dst *Metric)

func parseCounterConfig(c metricConstructor, d parseutil2.ParseData, cfgLit *literals2.Struct, dst *Metric) {
	// We don't have any actual configuration yet.
	// Parse anyway to make sure we don't have any fields we don't expect.
	type decodedConfig struct{}
	_ = literals2.Decode[decodedConfig](d.Pass.Errs, cfgLit)
}

func parseGaugeConfig(c metricConstructor, d parseutil2.ParseData, cfgLit *literals2.Struct, dst *Metric) {
	// We don't have any actual configuration yet.
	// Parse anyway to make sure we don't have any fields we don't expect.
	type decodedConfig struct{}
	_ = literals2.Decode[decodedConfig](d.Pass.Errs, cfgLit)
}
