package metrics

import (
	"go/ast"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	literals "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

type MetricType int

const (
	Counter MetricType = iota
	Gauge
)

type Metric struct {
	Name string     // The unique name of the metric
	Doc  string     // The documentation on the metric
	Type MetricType // the type of metric it is

	// File is the file the metric is declared in.
	File *pkginfo.File

	// LabelType is the label type of the metric,
	// if the metric is a group.
	LabelType option.Option[schema.Type]

	ValueType schema.BuiltinType

	// The struct literal for the config. Used to inject additional configuration
	// at compile-time.
	ConfigLiteral *ast.CompositeLit
}

func (m *Metric) Kind() resource.Kind       { return resource.Metric }
func (m *Metric) DeclaredIn() *pkginfo.File { return m.File }

// metricConstructor describes a particular metric constructor function.
type metricConstructor struct {
	FuncName    string
	ConfigName  string
	ConfigParse configParseFunc
	HasLabels   bool
	Type        MetricType
}

var metricConstructors = []metricConstructor{
	{"NewCounter", "CounterConfig", parseCounterConfig, false, Counter},
	{"NewCounterGroup", "CounterConfig", parseCounterConfig, true, Counter},
	{"NewGauge", "GaugeConfig", parseGaugeConfig, false, Gauge},
	{"NewGaugeGroup", "GaugeConfig", parseGaugeConfig, true, Gauge},
}

var MetricParser = &resource.Parser{
	Name:      "Metric",
	DependsOn: nil,

	RequiredImports: []paths.Pkg{"encore.dev/metrics"},
	Run: func(p *resource.Pass) []resource.Resource {
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

			c := c // capture for closure
			parseFn := func(d parseutil.ParseData) resource.Resource {
				return parseMetric(c, d)
			}

			spec := &parseutil.ResourceCreationSpec{
				AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
				MinTypeArgs: numTypeArgs,
				MaxTypeArgs: numTypeArgs,
				Parse:       parseFn,
			}
			specs[name] = spec
		}

		var resources []resource.Resource
		parseutil.FindPkgNameRefs(p.Pkg, names, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			spec := specs[name]
			r := parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
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

func parseMetric(c metricConstructor, d parseutil.ParseData) resource.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	errs := d.Pass.Errs
	if len(d.Call.Args) != 2 {
		errs.Addf(d.Call.Pos(), "%s requires two arguments: the metric name and the metric configuration",
			displayName)
		return nil
	}

	// Validate the metric name.
	metricName := parseutil.ParseResourceName(errs, displayName, "metric name",
		d.Call.Args[0], parseutil.SnakeName, "e_")
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
		errs.Add(d.Call.Pos(), "metric value type must be a builtin type")
		return nil
	}

	var labelType option.Option[schema.Type]
	if c.HasLabels {
		// Make sure it's a named struct, without pointers.
		typeArg := d.TypeArgs[0]
		declRef, ok := schemautil.ResolveNamedStruct(typeArg, false)
		if !ok {
			errs.Add(typeArg.ASTExpr().Pos(), "invalid metric label type: must be a named struct")
			return nil
		} else if declRef.Pointers > 0 {
			errs.Add(typeArg.ASTExpr().Pos(), "invalid metric label type: must not be a pointer type")
			return nil
		}

		// Make sure all the fields are builtin types.
		concrete := schemautil.ConcretizeWithTypeArgs(declRef.Decl.Type, declRef.TypeArgs).(schema.StructType)
		validKinds := append([]schema.BuiltinKind{schema.Bool, schema.String}, schemautil.Integers...)
		for _, f := range concrete.Fields {
			if f.IsAnonymous() {
				errs.Add(f.AST.Pos(), "anonymous fields are not supported in metric labels")
			} else if !schemautil.IsBuiltinKind(f.Type, validKinds...) {
				errs.Addf(f.AST.Pos(), "invalid metric label field %s: must be string, bool, or integer type",
					f.Name.MustGet())
			}
		}
		labelType = option.Some(typeArg)
	}

	m := &Metric{
		Name:      metricName,
		Doc:       d.Doc,
		Type:      c.Type,
		File:      d.File,
		ValueType: valueType.(schema.BuiltinType),
		LabelType: labelType,
	}

	// Parse and validate the metric configuration.
	cfgLit, ok := literals.ParseStruct(errs, d.File, "metrics.MetricConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
	}
	c.ConfigParse(c, d, cfgLit, m)
	m.ConfigLiteral = cfgLit.Lit()
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
