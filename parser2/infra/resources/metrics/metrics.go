package metrics

import (
	"encr.dev/parser2/internal/schema"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

import (
	"go/ast"

	"encr.dev/parser2/infra/internal/literals"
	"encr.dev/parser2/infra/internal/locations"
	"encr.dev/parser2/infra/internal/parseutil"
	"encr.dev/parser2/infra/resources"
	"encr.dev/parser2/internal/pkginfo"
)

type Metric struct {
	Name string // The unique name of the pub sub topic
	Doc  string // The documentation on the pub sub topic
}

func (m *Metric) Kind() resources.Kind { return resources.Metric }

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

var MetricParser = &resources.Parser{
	Name:      "Metric",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/metrics"},
	Run: func(p *resources.Pass) {
		var (
			names []pkginfo.QualifiedName
			specs = make(map[pkginfo.QualifiedName]*parseutil.ResourceCreationSpec)
		)
		for _, c := range metricConstructors {
			name := pkginfo.QualifiedName{PkgPath: "encore.dev/metrics", Name: c.FuncName}
			names = append(names, name)

			numTypeArgs := 1
			if c.HasLabels {
				numTypeArgs = 2
			}
			spec := &parseutil.ResourceCreationSpec{
				AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
				MinTypeArgs: numTypeArgs,
				MaxTypeArgs: numTypeArgs,
				Parse: func(d parseutil.ParseData) resources.Resource {
					return parseMetric(c, d)
				},
			}
			specs[name] = spec
		}

		parseutil.FindPkgNameRefs(p.Pkg, names, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			spec := specs[name]
			parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseMetric(c metricConstructor, d parseutil.ParseData) resources.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Addf(d.Call.Pos(), "%s requires two arguments: the metric name and the metric configuration",
			displayName)
		return nil
	}

	// Validate the metric name.
	metricName := parseutil.ParseResourceName(d.Pass.Errs, displayName, "metric name",
		d.Call.Args[0], parseutil.SnakeName, "e_")
	if metricName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	// Validate the metric value type.
	if d.TypeArgs[0].Family() != schema.Builtin {
		d.Pass.Errs.Add(d.Call.Pos(), "metric value type must be a builtin type")
		return nil
	}

	m := &Metric{
		Name: metricName,
		Doc:  d.Doc,
	}

	// Parse and validate the metric configuration.
	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "metrics.MetricConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
	}
	c.ConfigParse(c, d, cfgLit, m)
	return m
}

type configParseFunc func(c metricConstructor, d parseutil.ParseData, cfgLit *literals.Struct, dst *Metric)

func parseCounterConfig(c metricConstructor, d parseutil.ParseData, cfgLit *literals.Struct, dst *Metric) {
	// We don't have any actual configuration yet.
	// Parse anyway to make sure we don't have any fields we don't expect.
	type decodedConfig struct{}
	_ = literals.Decode[decodedConfig](d.Pass.Errs, cfgLit)
}

func parseGaugeConfig(c metricConstructor, d parseutil.ParseData, cfgLit *literals.Struct, dst *Metric) {
	// We don't have any actual configuration yet.
	// Parse anyway to make sure we don't have any fields we don't expect.
	type decodedConfig struct{}
	_ = literals.Decode[decodedConfig](d.Pass.Errs, cfgLit)
}
