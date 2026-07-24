package ecl

import (
	"sort"
	"strings"

	eclv1 "encr.dev/proto/encore/ecl/v1"
)

// ToProto converts an environment evaluation result into the wire schema in
// encore.ecl.v1. The RuleSet supplies whether each resource kind is managed.
func (rs *RuleSet) ToProto(er *EnvResult) *eclv1.EvaluationResult {
	out := &eclv1.EvaluationResult{}
	for _, r := range er.Results {
		out.Resources = append(out.Resources, rs.resourceToProto(r))
	}
	return out
}

func (rs *RuleSet) resourceToProto(r *Result) *eclv1.Resource {
	res := &eclv1.Resource{
		Kind:    r.Resource.Kind,
		Name:    r.Resource.Name,
		Managed: rs.isManaged(r.Resource.Kind),
	}

	// Scalar properties (reference-valued ones are emitted under references).
	var paths []string
	for path, rp := range r.Properties {
		if rp.Ref == nil {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		rp := r.Properties[path]
		res.Properties = append(res.Properties, &eclv1.Property{
			Path:       path,
			Value:      valueToProto(rp.Value),
			Source:     sourceToProto(rp.Source),
			Constraint: mergeConstraints(propConstraints(path, r.Matched)),
		})
	}

	// Reference-valued properties, grouped by path.
	byPath := map[string]*eclv1.Reference{}
	var refPaths []string
	for _, rr := range r.References {
		ref, ok := byPath[rr.Path]
		if !ok {
			ref = &eclv1.Reference{
				Path:       rr.Path,
				TargetKind: rr.TargetKind,
				TargetName: rr.TargetName,
				Unresolved: rr.unresolved,
			}
			byPath[rr.Path] = ref
			refPaths = append(refPaths, rr.Path)
		}
		if rr.Object != nil {
			for _, p := range rr.Object.Props {
				if sv := p.scalar(); sv != nil && sv.Constraint != nil {
					ref.Object = append(ref.Object, &eclv1.PropertyConstraint{
						Path:       p.Path,
						Constraint: mergeConstraints([]Constraint{sv.Constraint}),
					})
				}
			}
		}
	}
	sort.Strings(refPaths)
	for _, p := range refPaths {
		res.References = append(res.References, byPath[p])
	}
	return res
}

// propConstraints collects the scalar constraint expressions for a property
// path from every rule that matched the resource.
func propConstraints(path string, matched []*Rule) []Constraint {
	var cs []Constraint
	for _, r := range matched {
		for _, p := range r.Props {
			if p.Path != path {
				continue
			}
			if sv := p.scalar(); sv != nil && sv.Constraint != nil {
				cs = append(cs, sv.Constraint)
			}
		}
	}
	return cs
}

// mergeConstraints normalizes a set of constraints (all conjoined) into the
// wire Constraint: required flag, min/max bounds, allowed/excluded value sets,
// plus the effective constraint rendered into expr. Returns nil if empty.
func mergeConstraints(cs []Constraint) *eclv1.Constraint {
	if len(cs) == 0 {
		return nil
	}

	var (
		exprParts    []string
		seen         = map[string]bool{}
		required     bool
		lower, upper *Comparison
		exact        *Comparison
		excluded     []Value
		sets         [][]Value
	)
	for _, c := range cs {
		if s := c.String(); !seen[s] {
			seen[s] = true
			exprParts = append(exprParts, s)
		}
		for _, term := range conjuncts(c) {
			switch t := term.(type) {
			case *RequiredConstraint:
				required = true
			case *Comparison:
				switch t.Op {
				case OpEq:
					exact = t
				case OpNeq:
					excluded = append(excluded, t.Value)
				case OpGe, OpGt:
					if lower == nil || tighterLower(t, lower) {
						lower = t
					}
				case OpLe, OpLt:
					if upper == nil || tighterUpper(t, upper) {
						upper = t
					}
				}
			case *OrConstraint:
				if vs, ok := exactAlternatives(t); ok {
					sets = append(sets, vs)
				}
			}
		}
	}

	out := &eclv1.Constraint{
		Required: required,
		Expr:     strings.Join(exprParts, " & "),
	}
	if lower != nil {
		out.Min = &eclv1.Bound{Value: valueToProto(lower.Value), Inclusive: lower.Op == OpGe}
	}
	if upper != nil {
		out.Max = &eclv1.Bound{Value: valueToProto(upper.Value), Inclusive: upper.Op == OpLe}
	}

	var allowed []Value
	switch {
	case exact != nil:
		allowed = []Value{exact.Value}
	case len(sets) > 0:
		allowed = sets[0]
		for _, s := range sets[1:] {
			allowed = intersectValues(allowed, s)
		}
	}
	for _, v := range allowed {
		out.Allowed = append(out.Allowed, valueToProto(v))
	}
	for _, v := range excluded {
		out.Excluded = append(out.Excluded, valueToProto(v))
	}
	return out
}

func intersectValues(a, b []Value) []Value {
	var out []Value
	for _, v := range a {
		if valueInSet(v, b) {
			out = append(out, v)
		}
	}
	return out
}

func valueToProto(v Value) *eclv1.Value {
	out := &eclv1.Value{}
	switch v.Kind {
	case NumberKind:
		out.Kind = &eclv1.Value_NumberValue{NumberValue: v.Num}
	case BoolKind:
		out.Kind = &eclv1.Value_BoolValue{BoolValue: v.Bool}
	case StringKind:
		out.Kind = &eclv1.Value_StringValue{StringValue: v.Str}
	case SizeKind:
		out.Kind = &eclv1.Value_SizeBytes{SizeBytes: v.Num}
		out.Unit = v.unit
	case DurationKind:
		out.Kind = &eclv1.Value_DurationMs{DurationMs: v.Num}
		out.Unit = v.unit
	}
	return out
}

func sourceToProto(s ValueSource) eclv1.ValueSource {
	switch s {
	case SourceExplicit:
		return eclv1.ValueSource_VALUE_SOURCE_EXPLICIT
	case SourceDefault:
		return eclv1.ValueSource_VALUE_SOURCE_DEFAULT
	default:
		return eclv1.ValueSource_VALUE_SOURCE_UNSPECIFIED
	}
}
