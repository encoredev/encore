package metrics

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/idents"
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	literals "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

//go:generate stringer -type=MetricType -output=metrics_string.go

type MetricType int

const (
	Counter MetricType = iota
	Gauge
)

type Metric struct {
	AST  *ast.CallExpr
	Name string     // The unique name of the metric
	Doc  string     // The documentation on the metric
	Type MetricType // the type of metric it is

	// File is the file the metric is declared in.
	File *pkginfo.File

	// LabelType is the label type of the metric,
	// if the metric is a group.
	LabelType option.Option[schema.Type]

	// Labels is the list of parsed labels.
	Labels []Label

	ValueType schema.BuiltinType

	// The struct literal for the config. Used to inject additional configuration
	// at compile-time.
	ConfigLiteral *ast.CompositeLit
}

func (m *Metric) Kind() resource.Kind       { return resource.Metric }
func (m *Metric) Package() *pkginfo.Package { return m.File.Pkg }
func (m *Metric) ASTExpr() ast.Expr         { return m.AST }
func (m *Metric) ResourceName() string      { return m.Name }
func (m *Metric) Pos() token.Pos            { return m.AST.Pos() }
func (m *Metric) End() token.Pos            { return m.AST.End() }

type Label struct {
	Key  string
	Type schema.Type
	Doc  string
}

func (l Label) String() string {
	return fmt.Sprintf("%s %s %s", l.Key, strings.ToUpper(l.Type.String()), l.Doc)
}

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

var MetricParser = &resourceparser.Parser{
	Name: "Metric",

	InterestingImports: []paths.Pkg{"encore.dev/metrics"},
	Run: func(p *resourceparser.Pass) {
		var (
			names []pkginfo.QualifiedName
			specs = make(map[pkginfo.QualifiedName]*parseutil.ReferenceSpec)
		)
		for _, c := range metricConstructors {
			name := pkginfo.QualifiedName{PkgPath: "encore.dev/metrics", Name: c.FuncName}
			names = append(names, name)

			numTypeArgs := 1
			if c.HasLabels {
				numTypeArgs = 2
			}

			c := c // capture for closure
			parseFn := func(d parseutil.ReferenceInfo) {
				parseMetric(c, d)
			}

			spec := &parseutil.ReferenceSpec{
				AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
				MinTypeArgs: numTypeArgs,
				MaxTypeArgs: numTypeArgs,
				Parse:       parseFn,
			}
			specs[name] = spec
		}

		parseutil.FindPkgNameRefs(p.Pkg, names, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			spec := specs[name]
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseMetric(c metricConstructor, d parseutil.ReferenceInfo) {
	displayName := d.ResourceFunc.NaiveDisplayName()
	errs := d.Pass.Errs
	if len(d.Call.Args) != 2 {
		errs.Add(errInvalidArgCount(displayName, len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	// Validate the metric name.
	metricName := parseutil.ParseResourceName(errs, displayName, "metric name",
		d.Call.Args[0], parseutil.SnakeName, "e_")
	if metricName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	// Validate the metric value type.
	valueType := d.TypeArgs[0]
	if c.HasLabels {
		valueType = d.TypeArgs[1]
	}
	if valueType.Family() != schema.Builtin {
		errs.Add(errInvalidMetricType.AtGoNode(valueType.ASTExpr()))
		return
	}

	var labelType option.Option[schema.Type]
	var labelFields []Label
	if c.HasLabels {
		// Make sure it's a named struct, without pointers.
		typeArg := d.TypeArgs[0]
		declRef, ok := schemautil.ResolveNamedStruct(typeArg, false)
		if !ok {
			errs.Add(errInvalidLabelType.AtGoNode(typeArg.ASTExpr()))
			return
		} else if declRef.Pointers > 0 {
			errs.Add(errLabelNoPointer.AtGoNode(typeArg.ASTExpr()))
			return
		}

		// Make sure all the fields are builtin types.
		concrete := schemautil.ConcretizeWithTypeArgs(declRef.Decl.Type, declRef.TypeArgs).(schema.StructType)
		validKinds := append([]schema.BuiltinKind{schema.Bool, schema.String}, schemautil.Integers...)
		for _, f := range concrete.Fields {
			if f.IsAnonymous() {
				errs.Add(errLabelNoAnonymous.AtGoNode(f.AST))
			} else if !schemautil.IsBuiltinKind(f.Type, validKinds...) {
				errs.Add(errLabelInvalidType.AtGoNode(f.AST.Type, errors.AsError(fmt.Sprintf("got %s", literals.PrettyPrint(f.AST.Type)))))
			} else {
				// Validate the label
				label := idents.Convert(f.Name.MustGet(), idents.SnakeCase)
				if label == "service" {
					errs.Add(errLabelReservedName.AtGoNode(f.AST.Names[0]))
				}

				labelFields = append(labelFields, Label{
					Key:  label,
					Type: f.Type,
					Doc:  f.Doc,
				})
			}
		}
		labelType = option.Some(typeArg)
	}

	m := &Metric{
		AST:       d.Call,
		Name:      metricName,
		Doc:       d.Doc,
		Type:      c.Type,
		File:      d.File,
		ValueType: valueType.(schema.BuiltinType),
		LabelType: labelType,
		Labels:    labelFields,
	}

	// Parse and validate the metric configuration.
	cfgLit, ok := literals.ParseStruct(errs, d.File, "metrics.MetricConfig", d.Call.Args[1])
	if !ok {
		return // error reported by ParseStruct
	}
	c.ConfigParse(c, d, cfgLit, m)
	m.ConfigLiteral = cfgLit.Lit()

	d.Pass.RegisterResource(m)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, m)
	}
}

type configParseFunc func(c metricConstructor, d parseutil.ReferenceInfo, cfgLit *literals.Struct, dst *Metric)

func parseCounterConfig(c metricConstructor, d parseutil.ReferenceInfo, cfgLit *literals.Struct, dst *Metric) {
	// We don't have any actual configuration yet.
	// Parse anyway to make sure we don't have any fields we don't expect.
	type decodedConfig struct{}
	_ = literals.Decode[decodedConfig](d.Pass.Errs, cfgLit, nil)
}

func parseGaugeConfig(c metricConstructor, d parseutil.ReferenceInfo, cfgLit *literals.Struct, dst *Metric) {
	// We don't have any actual configuration yet.
	// Parse anyway to make sure we don't have any fields we don't expect.
	type decodedConfig struct{}
	_ = literals.Decode[decodedConfig](d.Pass.Errs, cfgLit, nil)
}
