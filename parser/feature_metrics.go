package parser

import (
	"go/ast"

	"encr.dev/parser/encoding"
	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	"encr.dev/pkg/errinsrc/srcerrors"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// metricConstructor describes a particular metric constructor function.
type metricConstructor struct {
	FuncName  string
	HasLabels bool
}

var metricConstructors = []metricConstructor{
	{"NewCounter", false},
	{"NewCounterGroup", true},
	{"NewGauge", false},
	{"NewGaugeGroup", true},
}

func init() {
	registerResource(
		est.MetricResource,
		"metric",
		"https://encore.dev/docs/observability/metrics",
		"metrics",
		"encore.dev/metrics",
	)

	registerResourceReferenceParser(
		est.MetricResource,
		(*parser).parseMetricReference,
	)

	for _, constructor := range metricConstructors {
		numTypeArgs := 1
		if constructor.HasLabels {
			numTypeArgs = 2
		}

		registerResourceCreationParser(
			est.MetricResource,
			constructor.FuncName, numTypeArgs,
			createMetricParser(constructor),
			locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
		)
	}
}

func createMetricParser(con metricConstructor) func(*parser, *est.File, *walker.Cursor, *ast.Ident, *ast.CallExpr) est.Resource {
	return func(p *parser, file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
		if len(callExpr.Args) != 2 {
			p.errf(
				callExpr.Pos(),
				"metrics.%s requires two arguments: the metric name and the metric configuration",
				con.FuncName,
			)
			return nil
		}

		metricName := p.parseResourceName("metrics."+con.FuncName, "metric name", callExpr.Args[0], snakeName, "e_")
		if metricName == "" {
			// we already reported the error inside parseResourceName
			return nil
		}

		cfg, ok := p.parseStructLit(file, "metrics.MetricConfig", callExpr.Args[1])
		if !ok {
			return nil
		}

		if !cfg.FullyConstant() {
			dynamic := cfg.DynamicFields()
			failed := false
			for fieldName, expr := range dynamic {
				failed = true
				p.errf(expr.Pos(), "The %s field in metrics.MetricConfig must be a constant literal, got %v", fieldName, prettyPrint(expr))
			}
			if failed {
				return nil
			}
		}

		metric := &est.Metric{
			Doc:       cursor.DocComment(),
			Svc:       file.Pkg.Service, // nil means global metric
			DeclFile:  file,
			DeclCall:  callExpr,
			IdentAST:  ident,
			ConfigLit: cfg.Lit(),
		}

		// Resolve labels
		if con.HasLabels {
			typeArgs := getTypeArguments(callExpr.Fun)
			if _, isPtr := typeArgs[0].(*ast.StarExpr); isPtr {
				p.errInSrc(srcerrors.MetricLabelsIsPointer(p.fset, typeArgs[0], "metrics."+con.FuncName))
				return nil
			}
			metric.LabelsType = p.resolveType(file.Pkg, file, typeArgs[0], nil)
			metric.LabelsAST = typeArgs[0]
			p.validateMetricLabels(metric, con)
		}

		p.metrics = append(p.metrics, metric)

		return metric
	}
}

// validateMetricLabels validates a parsed cache keyspace.
func (p *parser) validateMetricLabels(m *est.Metric, con metricConstructor) {
	labels := m.LabelsType
	named := labels.GetNamed()
	if named == nil {
		p.errInSrc(srcerrors.MetricLabelsNotNamedStruct(p.fset, m.LabelsAST, "metrics."+con.FuncName, labels))
		return
	}

	decl := p.decls[named.Id]
	if decl.Type.GetStruct() == nil {
		p.errInSrc(srcerrors.MetricLabelsNotNamedStruct(p.fset, m.LabelsAST, "metrics."+con.FuncName, decl.Type))
		return
	}

	st, err := encoding.GetConcreteStructType(p.decls, decl.Type, named.TypeArguments)
	if err != nil {
		p.errf(m.Ident().Pos(), "unable to resolve concrete type: %v", err)
		return
	}

	// Validate struct fields
	for _, f := range st.Fields {
		// Allow strings, bools, int and uint types.
		switch f.Typ.GetBuiltin() {
		case schema.Builtin_STRING, schema.Builtin_BOOL,
			schema.Builtin_INT, schema.Builtin_INT8, schema.Builtin_INT16, schema.Builtin_INT32, schema.Builtin_INT64,
			schema.Builtin_UINT, schema.Builtin_UINT8, schema.Builtin_UINT16, schema.Builtin_UINT32, schema.Builtin_UINT64:
		// OK
		default:
			p.errInSrc(srcerrors.MetricLabelsFieldInvalidType(p.fset, m.LabelsAST, "metrics."+con.FuncName, f.Name, f.Typ))
		}
	}
}

func (p *parser) parseMetricReference(file *est.File, resource est.Resource, cursor *walker.Cursor) {
	metric := resource.(*est.Metric)
	if metric.Svc != nil && file.Pkg.Service != metric.Svc {
		p.errInSrc(srcerrors.MetricReferencedInOtherService(p.fset, cursor.Node(), metric.IdentAST))
		return
	}
}
