package parser

import (
	"go/ast"

	"encr.dev/parser/encoding"
	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/idents"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// metricConstructor describes a particular metric constructor function.
type metricConstructor struct {
	FuncName   string
	HasLabels  bool
	MetricKind meta.Metric_MetricKind
}

var metricConstructors = []metricConstructor{
	{"NewCounter", false, meta.Metric_COUNTER},
	{"NewCounterGroup", true, meta.Metric_COUNTER},
	{"NewGauge", false, meta.Metric_GAUGE},
	{"NewGaugeGroup", true, meta.Metric_GAUGE},
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

		var (
			metricValueTypeArg ast.Expr
			labelTypeArg       ast.Expr
		)
		typeArgs := getTypeArguments(callExpr.Fun)
		switch len(typeArgs) {
		case 0:
			// Metric constructors should always have one type argument to define the metric
			// value type.
			if con.HasLabels {
				p.errf(callExpr.Pos(), "metrics.%s requires two type arguments", con.FuncName)
			} else {
				p.errf(callExpr.Pos(), "metrics.%s requires type argument", con.FuncName)
			}
			return nil
		case 1:
			// Error if the metric constructor needs labels.
			if con.HasLabels {
				p.errf(callExpr.Pos(), "metrics.%s requires two type arguments (got one type argument only)", con.FuncName)
				return nil
			}
			metricValueTypeArg = typeArgs[0]
		default:
			labelTypeArg = typeArgs[0]
			metricValueTypeArg = typeArgs[1]
		}

		metricValueType := p.resolveType(file.Pkg, file, metricValueTypeArg, nil).GetBuiltin()
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
			Name:      metricName,
			ValueType: metricValueType,
			Kind:      con.MetricKind,
			Doc:       cursor.DocComment(),
			Svc:       file.Pkg.Service, // nil means global metric
			DeclFile:  file,
			DeclCall:  callExpr,
			IdentAST:  ident,
			ConfigLit: cfg.Lit(),
		}

		// Resolve labels
		if con.HasLabels {
			if _, isPtr := labelTypeArg.(*ast.StarExpr); isPtr {
				p.errInSrc(srcerrors.MetricLabelsIsPointer(p.fset, typeArgs[0], "metrics."+con.FuncName))
				return nil
			}
			metric.LabelsType = p.resolveType(file.Pkg, file, typeArgs[0], nil)
			metric.LabelsAST = typeArgs[0]
			metric.Labels = p.validateMetricLabels(metric, con)
		}

		p.metrics = append(p.metrics, metric)

		return metric
	}
}

func (p *parser) validateMetricLabels(m *est.Metric, con metricConstructor) []est.Label {
	labels := m.LabelsType
	named := labels.GetNamed()
	if named == nil {
		p.errInSrc(srcerrors.MetricLabelsNotNamedStruct(p.fset, m.LabelsAST, "metrics."+con.FuncName, labels))
		return nil
	}

	decl := p.decls[named.Id]
	if decl.Type.GetStruct() == nil {
		p.errInSrc(srcerrors.MetricLabelsNotNamedStruct(p.fset, m.LabelsAST, "metrics."+con.FuncName, decl.Type))
		return nil
	}

	st, err := encoding.GetConcreteStructType(p.decls, decl.Type, named.TypeArguments)
	if err != nil {
		p.errf(m.Ident().Pos(), "unable to resolve concrete type: %v", err)
		return nil
	}

	// Validate struct fields
	var parsed []est.Label
	for _, f := range st.Fields {
		label := idents.Convert(f.Name, idents.SnakeCase)
		if label == "service" {
			p.errInSrc(srcerrors.MetricLabelReservedName(p.fset, m.LabelsAST, f.Name, label))
			continue
		}

		// Allow strings, bools, int and uint types.
		switch f.Typ.GetBuiltin() {
		case schema.Builtin_STRING, schema.Builtin_BOOL,
			schema.Builtin_INT, schema.Builtin_INT8, schema.Builtin_INT16, schema.Builtin_INT32, schema.Builtin_INT64,
			schema.Builtin_UINT, schema.Builtin_UINT8, schema.Builtin_UINT16, schema.Builtin_UINT32, schema.Builtin_UINT64:
			// OK
		default:
			p.errInSrc(srcerrors.MetricLabelsFieldInvalidType(p.fset, m.LabelsAST, "metrics."+con.FuncName, f.Name, f.Typ))
			continue
		}

		parsed = append(parsed, est.Label{
			Key:  label,
			Type: f.Typ.GetBuiltin(),
			Doc:  f.Doc,
		})
	}
	return parsed
}

func (p *parser) parseMetricReference(file *est.File, resource est.Resource, cursor *walker.Cursor) {
	metric := resource.(*est.Metric)
	if metric.Svc != nil && file.Pkg.Service != metric.Svc {
		p.errInSrc(srcerrors.MetricReferencedInOtherService(p.fset, cursor.Node(), metric.IdentAST))
		return
	}
}
