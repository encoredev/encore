package ecl

import (
	"fmt"
	"slices"
	"strings"
)

// Validate statically analyzes the rule set without reference to any
// concrete resource. It reports:
//
//   - unknown resource kinds (see RuleSet.Schema)
//   - reference properties pointed at the wrong kind, and scalar/reference
//     property mismatches
//   - duplicate property rules within a rule or object constraint
//   - selectors that can never match
//   - mixed value types within a property rule
//   - defaults that violate their own rule's constraint
//   - 'required' used inside a '|' alternative
//   - pairs of rules that can match the same resource but produce
//     impossible constraints when merged
//   - defaults that violate the constraints of another rule that can
//     match the same resource
//
// Ambiguous defaults are not flagged statically: whether two defaults
// compete depends on which rules match a concrete resource, so that
// check is performed by Evaluate. The cross-rule checks here are
// conservative; Evaluate always performs the authoritative checks for
// concrete resources.
func (rs *RuleSet) Validate() error {
	v := &validator{rs: rs, norms: make(map[*Rule][]normCond)}
	v.run()
	v.diags.sort()
	return v.diags.Err()
}

type validator struct {
	rs    *RuleSet
	diags ErrorList
	norms map[*Rule][]normCond

	// badProps marks (rule, path) pairs whose property rule already
	// failed a local check, to avoid piling on cross-rule diagnostics.
	badProps map[ruleProp]bool
	reported map[string]bool // dedupe of cross-rule diagnostics
}

func (v *validator) run() {
	v.badProps = make(map[ruleProp]bool)
	v.reported = make(map[string]bool)

	var all []*Rule
	v.rs.rules(func(r *Rule) { all = append(all, r) })

	for _, r := range all {
		v.checkKindName(r)
		v.checkSelector(r)
		v.checkEnvScope(r)
		v.checkProperties(r)
	}
	v.checkRulePairs(all)
}

// checkEnvScope flags `if` block conditions that reference a non-environment
// attribute. Environment scope is the only thing in scope at the top level, so
// resource conditions (team, tags, ...) belong on the resource rule's `if`
// clause instead.
func (v *validator) checkEnvScope(r *Rule) {
	scope := v.rs.envScope()
	if len(scope) == 0 {
		return // explicitly empty scope disables the check
	}
	for _, c := range r.Where {
		if !c.EnvScoped || v.rs.isEnvScoped(c.Field) {
			continue
		}
		// `if` conditions are shared across the rules they wrap; report each
		// offending condition once via the position+message dedupe in report.
		d := &Diagnostic{
			Pos: c.Pos, End: c.FieldEnd, src: ruleSrc(r),
			Message: fmt.Sprintf("'%s' is not an environment attribute and cannot be tested in an 'if' block", c.Field),
		}
		d.Hint = fmt.Sprintf("'if' tests environment attributes (%s); move resource conditions to each rule's 'if' clause", strings.Join(scope, ", "))
		v.report(d)
	}
}

func (v *validator) norm(r *Rule) []normCond {
	if n, ok := v.norms[r]; ok {
		return n
	}
	n := normalizeRule(r)
	v.norms[r] = n
	return n
}

func (v *validator) checkKindName(r *Rule) {
	schema := v.rs.schema()
	if len(schema) == 0 {
		return // explicitly empty schema disables the check
	}
	if _, ok := schema[r.Kind]; ok {
		return
	}
	kinds := sortedKinds(schema)
	d := v.diags.addf(ruleSrc(r), r.KindPos, r.KindEnd, "unknown resource kind '%s'", r.Kind)
	if s := suggest(r.Kind, kinds); s != "" {
		d.Hint = fmt.Sprintf("did you mean '%s'?", s)
	} else {
		d.Hint = "known kinds: " + strings.Join(kinds, ", ")
	}
}

func sortedKinds(schema map[string]Kind) []string {
	kinds := make([]string, 0, len(schema))
	for k := range schema {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	return kinds
}

// checkSelector flags selectors that can never match, such as
// `env.type == production && env.type == staging`.
func (v *validator) checkSelector(r *Rule) {
	conds := v.norm(r)
	// conds[0] is the name condition from the rule header when the rule
	// is named; the rest map to r.Where in order.
	offset := 0
	if r.Name != "" {
		offset = 1
	}
	for i := range conds {
		for j := i + 1; j < len(conds); j++ {
			if contradicts(conds[i], conds[j]) {
				// Point at the second of the two contradicting
				// conditions (the name condition from the header has no
				// source node of its own, so fall back to the rule).
				pos, end := r.Pos, Position{}
				if j >= offset {
					cond := r.Where[j-offset]
					pos, end = cond.Pos, cond.End
				}
				d := v.diags.addf(ruleSrc(r), pos, end,
					"this rule can never match: '%s' contradicts '%s'", conds[j], conds[i])
				d.Detail = []string{"in rule: " + r.Header()}
				return
			}
		}
	}
}

func (v *validator) checkProperties(r *Rule) {
	seen := make(map[string]*Property)
	for _, p := range r.Props {
		if prev, ok := seen[p.Path]; ok {
			d := v.diags.addf(ruleSrc(r), p.Pos, p.PathEnd,
				"duplicate property rule for '%s' in the same rule", p.Path)
			d.Related = append(d.Related, RelatedInfo{
				Pos:     prev.Pos,
				Message: fmt.Sprintf("'%s' was first defined at %s", p.Path, prev.Pos),
			})
			d.Hint = "combine the constraints with '&' in a single property rule"
			v.badProps[ruleProp{rule: r, prop: p}] = true
			continue
		}
		seen[p.Path] = p
		v.checkProperty(r, p, r.Kind)
	}
}

func (v *validator) checkProperty(r *Rule, p *Property, kind string) {
	switch p.Value.(type) {
	case *RefValue:
		v.checkRefProperty(r, p, kind)
	default:
		v.checkScalarProperty(r, p, kind)
	}
}

// checkRefProperty validates a reference-valued property: the path must be a
// declared reference of its kind, the reference must point at the right
// target kind, and any nested object constraints are validated against the
// target kind.
func (v *validator) checkRefProperty(r *Rule, p *Property, kind string) {
	rc := ruleProp{rule: r, prop: p}
	rv := p.ref()

	want, isRef := v.rs.refTarget(kind, p.Path)
	if !isRef {
		// Only flag when the kind is known; unknown kinds are reported by
		// checkKindName.
		if _, known := v.rs.kindSchema(kind); known {
			d := v.diags.addf(ruleSrc(r), p.Pos, p.PathEnd,
				"property '%s' of %s is not a reference property", p.Path, kind)
			d.Hint = refPropertyHint(v.rs, kind)
			v.badProps[rc] = true
		}
	} else {
		for _, ref := range []*Reference{rv.Ref, refOf(rv.Default)} {
			if ref == nil {
				continue
			}
			if ref.Kind != want {
				d := v.diags.addf(ruleSrc(r), ref.KindPos, ref.End,
					"property '%s' references %s, but it must reference a %s", p.Path, ref.Kind, want)
				d.Detail = []string{"in rule: " + r.Header()}
				v.badProps[rc] = true
			}
		}
	}

	// Validate the nested object constraint against the target kind.
	if rv.Object != nil {
		v.checkObjectConstraint(r, rv.Object, want)
	}
}

// checkObjectConstraint validates the properties of an object constraint as
// if they belonged to the target kind.
func (v *validator) checkObjectConstraint(r *Rule, obj *ObjectConstraint, targetKind string) {
	seen := make(map[string]*Property)
	for _, p := range obj.Props {
		if prev, ok := seen[p.Path]; ok {
			d := v.diags.addf(ruleSrc(r), p.Pos, p.PathEnd,
				"duplicate property '%s' in the same object constraint", p.Path)
			d.Related = append(d.Related, RelatedInfo{
				Pos:     prev.Pos,
				Message: fmt.Sprintf("'%s' was first defined at %s", p.Path, prev.Pos),
			})
			d.Hint = "combine the constraints with '&' in a single line"
			continue
		}
		seen[p.Path] = p
		switch p.Value.(type) {
		case *RefValue:
			v.checkRefProperty(r, p, targetKind)
		default:
			if !v.checkPropertyTypes(r, p) {
				continue
			}
			v.checkPropertySat(r, p)
		}
	}
}

func (v *validator) checkScalarProperty(r *Rule, p *Property, kind string) {
	rc := ruleProp{rule: r, prop: p}
	bad := func() { v.badProps[rc] = true }
	sv := p.scalar()

	// A scalar constraint on a declared reference property is a type error.
	if _, isRef := v.rs.refTarget(kind, p.Path); isRef {
		d := v.diags.addf(ruleSrc(r), p.Pos, p.PathEnd,
			"property '%s' is a reference and cannot take a scalar constraint", p.Path)
		want, _ := v.rs.refTarget(kind, p.Path)
		d.Hint = fmt.Sprintf("write a reference, e.g.: %s: %s.<name>", p.Path, want)
		bad()
		return
	}

	// 'required' must not appear inside a '|' alternative.
	walkConstraint(sv.Constraint, func(c Constraint) {
		or, ok := c.(*OrConstraint)
		if !ok {
			return
		}
		for _, alt := range or.Alts {
			walkConstraint(alt, func(inner Constraint) {
				if req, ok := inner.(*RequiredConstraint); ok {
					d := v.diags.addf(ruleSrc(r), req.Pos, req.End,
						"'required' cannot be part of a '|' alternative")
					d.Hint = fmt.Sprintf("combine it with '&' instead: %s: required & <constraint>", p.Path)
					bad()
				}
			})
		}
	})

	if !v.checkPropertyTypes(r, p) {
		bad()
		return
	}

	// The default must satisfy the rule's own constraint.
	if sv.Default != nil && sv.Constraint != nil {
		if ok, fail, _ := checkValue(sv.Default.Value, sv.Constraint); !ok {
			d := v.diags.addf(ruleSrc(r), sv.Default.ValuePos, sv.Default.ValueEnd,
				"default value %s violates the constraint '%s' in the same property rule",
				sv.Default.Value, fail)
			d.Detail = []string{"in rule: " + r.Header()}
			bad()
			return
		}
	}

	if !v.checkPropertySat(r, p) {
		bad()
	}
}

func refOf(d *RefDefault) *Reference {
	if d == nil {
		return nil
	}
	return d.Ref
}

// refPropertyHint lists the reference properties of a kind for diagnostics.
func refPropertyHint(rs *RuleSet, kind string) string {
	k, ok := rs.kindSchema(kind)
	if !ok || len(k.References) == 0 {
		return fmt.Sprintf("%s has no reference properties", kind)
	}
	names := make([]string, 0, len(k.References))
	for name := range k.References {
		names = append(names, name)
	}
	slices.Sort(names)
	return "reference properties of " + kind + ": " + strings.Join(names, ", ")
}

// checkPropertyTypes verifies that all values in a property rule
// (constraint values and the default) have the same type.
func (v *validator) checkPropertyTypes(r *Rule, p *Property) bool {
	type typedValue struct {
		kind ValueKind
		desc string
		span Span
	}
	var first *typedValue
	checkType := func(tv typedValue) bool {
		if first == nil {
			first = &tv
			return true
		}
		if first.kind != tv.kind {
			d := v.diags.addf(ruleSrc(r), tv.span.Start, tv.span.End,
				"mixed value types for property '%s': %s is a %s, but %s is a %s",
				p.Path, first.desc, first.kind, tv.desc, tv.kind)
			d.Detail = []string{"in rule: " + r.Header()}
			return false
		}
		return true
	}
	sv := p.scalar()
	if sv == nil {
		return true
	}
	ok := true
	walkConstraint(sv.Constraint, func(c Constraint) {
		if cmp, isCmp := c.(*Comparison); isCmp {
			ok = checkType(typedValue{cmp.Value.Kind, "'" + cmp.String() + "'", cmp.span()}) && ok
		}
	})
	if def := sv.Default; def != nil {
		span := Span{Start: def.ValuePos, End: def.ValueEnd}
		ok = checkType(typedValue{def.Value.Kind, "the default " + def.Value.String(), span}) && ok
	}
	return ok
}

// checkPropertySat verifies that a property rule's own constraints are
// satisfiable, e.g. `cpu: >= 4 & <= 2` is impossible.
func (v *validator) checkPropertySat(r *Rule, p *Property) bool {
	var diags ErrorList
	sc := &satChecker{path: p.Path, diags: &diags}
	sc.feed(ruleProp{rule: r, prop: p})
	sc.check()
	v.diags = append(v.diags, diags...)
	return len(diags) == 0
}

// checkRulePairs runs cross-rule checks on every pair of rules of the
// same kind whose selectors could match the same resource.
func (v *validator) checkRulePairs(all []*Rule) {
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			a, b := all[i], all[j]
			if a.Kind != b.Kind || !selectorsCanCoMatch(v.norm(a), v.norm(b)) {
				continue
			}
			v.checkPairConstraints(a, b)
		}
	}
}

func (v *validator) checkPairConstraints(a, b *Rule) {
	for _, pa := range a.Props {
		for _, pb := range b.Props {
			if pa.Path != pb.Path {
				continue
			}
			// Cross-rule constraint checks only apply to scalar properties.
			if pa.scalar() == nil || pb.scalar() == nil {
				continue
			}
			rcA, rcB := ruleProp{rule: a, prop: pa}, ruleProp{rule: b, prop: pb}
			if v.badProps[rcA] || v.badProps[rcB] {
				continue
			}
			var diags ErrorList
			sc := &satChecker{path: pa.Path, diags: &diags}
			sc.feed(rcA)
			sc.feed(rcB)
			sc.check()
			for _, d := range diags {
				d.Detail = append(d.Detail,
					"the rules can match the same resource, and constraints from all matching rules merge by intersection")
				v.report(d)
			}
			v.checkDefaultAgainst(rcA, rcB)
			v.checkDefaultAgainst(rcB, rcA)
		}
	}
}

// checkDefaultAgainst flags an explicit default in one rule that violates
// the constraint of another rule that can match the same resource.
// Implicit defaults (exact value constraints) are skipped: those
// conflicts are already reported as constraint conflicts.
//
// To avoid false positives, the default is only checked when it could
// actually be selected for a resource matching both rules: either the
// other rule provides no default of its own (so this one can win
// specificity), or this rule is strictly more specific (so this default
// always wins over the other's).
func (v *validator) checkDefaultAgainst(da, other ruleProp) {
	daSV, otherSV := da.prop.scalar(), other.prop.scalar()
	if daSV == nil || otherSV == nil {
		return
	}
	if daSV.Default == nil || otherSV.Constraint == nil {
		return
	}
	otherHasDefault := otherSV.Default != nil
	if !otherHasDefault {
		_, otherHasDefault = implicitDefault(otherSV.Constraint)
	}
	if otherHasDefault && !strictlyImplies(v.norm(da.rule), v.norm(other.rule)) {
		return
	}
	def := daSV.Default
	ok, fail, mismatch := checkValue(def.Value, otherSV.Constraint)
	if ok || mismatch != nil { // type conflicts are reported by the satisfiability check
		return
	}
	d := &Diagnostic{
		Pos: def.ValuePos,
		End: def.ValueEnd,
		Message: fmt.Sprintf("default value %s for property '%s' violates the constraint '%s' of another rule that can match the same resource",
			def.Value, da.prop.Path, fail),
		src: ruleSrc(da.rule),
		Detail: []string{
			fmt.Sprintf("  the default is defined at %s in rule: %s", def.ValuePos, da.rule.Header()),
			fmt.Sprintf("  the constraint is defined at %s in rule: %s", fail.span().Start, other.rule.Header()),
		},
	}
	v.report(d)
}

// report appends a diagnostic, deduplicating identical position+message
// pairs that can arise from overlapping pairwise checks.
func (v *validator) report(d *Diagnostic) {
	key := d.Pos.String() + "\x00" + d.Message
	if v.reported[key] {
		return
	}
	v.reported[key] = true
	v.diags = append(v.diags, d)
}
