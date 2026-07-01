package ecl

import (
	"fmt"
	"strconv"
	"strings"
)

// RuleSet is a collection of parsed files evaluated together as a single
// policy set.
type RuleSet struct {
	Files []*File

	// Schema describes the known resource kinds. If nil, DefaultSchema is
	// used. An explicitly empty (non-nil) map disables kind checking.
	Schema map[string]Kind

	// EnvScope lists the environment-scoped attribute names that an `if` block
	// may test, e.g. "env" (covering env.type, env.name, ...) or "provider". A
	// field is environment-scoped if it equals one of these names or has one as
	// a dotted prefix. If nil, DefaultEnvScope is used; an explicitly empty
	// (non-nil) slice disables the environment-scope check.
	EnvScope []string
}

// Kind is the schema of a resource kind.
type Kind struct {
	// Managed reports whether the kind is infrastructure managed by Encore:
	// a named or dynamic block of a managed kind instantiates the resource.
	// A block of an app-discovered kind (Managed false) only configures a
	// resource discovered from application code.
	Managed bool

	// References maps reference-valued property names of this kind to the
	// resource kind they refer to, e.g. {"cluster": "sql_cluster"}.
	References map[string]string
}

// DefaultSchema is the default resource-kind schema used by Validate and
// EvaluateEnv when RuleSet.Schema is nil.
var DefaultSchema = map[string]Kind{
	"service":          {References: map[string]string{"instance": "service_instance"}},
	"service_instance": {Managed: true},
	"sql_database":     {References: map[string]string{"cluster": "sql_cluster"}},
	"sql_cluster":      {Managed: true},
	"bucket":           {},
	"pubsub_topic":     {},
	"cache":            {},
	"secret":           {},
	"cron_job":         {},
}

// DefaultEnvScope is the default set of environment-scoped attribute names
// used by Validate when RuleSet.EnvScope is nil. "env" covers env.type,
// env.name, and other env.* attributes; "provider" is environment-wide.
var DefaultEnvScope = []string{"env", "provider"}

func (rs *RuleSet) schema() map[string]Kind {
	if rs.Schema != nil {
		return rs.Schema
	}
	return DefaultSchema
}

func (rs *RuleSet) envScope() []string {
	if rs.EnvScope != nil {
		return rs.EnvScope
	}
	return DefaultEnvScope
}

// isEnvScoped reports whether a field path is an environment attribute: it
// equals a declared environment-scope name or has one as a dotted prefix.
func (rs *RuleSet) isEnvScoped(field string) bool {
	for _, e := range rs.envScope() {
		if field == e || strings.HasPrefix(field, e+".") {
			return true
		}
	}
	return false
}

func (rs *RuleSet) kindSchema(kind string) (Kind, bool) {
	k, ok := rs.schema()[kind]
	return k, ok
}

func (rs *RuleSet) isManaged(kind string) bool {
	k, ok := rs.kindSchema(kind)
	return ok && k.Managed
}

// refTarget returns the kind that the named property of kind refers to,
// if it is a declared reference property.
func (rs *RuleSet) refTarget(kind, prop string) (string, bool) {
	k, ok := rs.kindSchema(kind)
	if !ok {
		return "", false
	}
	t, ok := k.References[prop]
	return t, ok
}

// NewRuleSet combines parsed files into a rule set.
func NewRuleSet(files ...*File) *RuleSet {
	return &RuleSet{Files: files}
}

func (rs *RuleSet) rules(fn func(*Rule)) {
	for _, f := range rs.Files {
		for _, r := range f.Rules {
			fn(r)
		}
	}
}

func ruleSrc(r *Rule) *sourceFile {
	if r.file == nil {
		return nil
	}
	return r.file.src
}

// Resource describes a concrete resource to evaluate rules against.
type Resource struct {
	Kind string // resource kind, e.g. "service"
	Name string // resource name, e.g. "api"

	// Attrs holds the resource's selector attributes keyed by dotted
	// field path, e.g. "env.type", "team", "tags.data".
	// The "name" field is implied by Name and need not be included.
	Attrs map[string]Value

	// Config holds the resource's explicitly configured properties,
	// keyed by dotted property path, e.g. "cpu" or "instances.min".
	Config map[string]Value
}

func (r *Resource) describe() string {
	if r.Name != "" {
		return r.Kind + " " + strconv.Quote(r.Name)
	}
	return r.Kind
}

// ValueSource says where a resolved property value came from.
type ValueSource int

const (
	// SourceExplicit means the resource configured the value itself.
	SourceExplicit ValueSource = iota
	// SourceDefault means the value came from a rule's default
	// (explicit `default v`, or an exact value constraint acting as one).
	SourceDefault
)

// ResolvedProperty is the final value of a single property.
type ResolvedProperty struct {
	Path        string
	Value       Value             // scalar value; zero for reference properties
	Ref         *ResolvedRefValue // non-nil for reference properties
	Source      ValueSource
	DefaultRule *Rule // the rule that provided the default; nil if explicit
}

// ResolvedRefValue is the resolved target of a reference-valued property.
type ResolvedRefValue struct {
	Kind string
	Name string
}

// Result is the outcome of evaluating a rule set against a resource.
type Result struct {
	// Resource is the resource that was evaluated.
	Resource *Resource
	// Properties holds the final resolved configuration: all explicitly
	// configured properties plus any applied defaults.
	Properties map[string]ResolvedProperty
	// Matched lists the rules that matched the resource, in source order.
	Matched []*Rule
	// References lists the reference-valued property constraints of all
	// matching rules, with the target resource resolved from the resource's
	// own configuration. They are checked against the target resource by
	// EvaluateEnv (the target must exist; object constraints must hold).
	References []ResolvedRef
}

// ResolvedRef is a reference-valued property of a resource, with its target
// resolved to a concrete (kind, name). An identity entry (Object nil)
// asserts the target exists; an object entry carries nested constraints to
// check against the target's resolved configuration.
type ResolvedRef struct {
	Path       string            // the property path holding the reference
	TargetKind string            // the referenced resource kind
	TargetName string            // resolved target name; "" if unresolved
	Object     *ObjectConstraint // nested constraints; nil for identity entries
	Rule       *Rule
	Prop       *Property
	unresolved string // reason the reference could not resolve; "" if resolved
}

// ruleProp pairs a rule with one of its property rules.
type ruleProp struct {
	rule *Rule
	prop *Property
}

type evaluator struct {
	rs    *RuleSet
	res   *Resource
	diags ErrorList
	norms map[*Rule][]normCond

	reportedConds map[*Condition]bool
}

// Evaluate applies the rule set to a resource: it finds all matching
// rules (named blocks merge like ordinary rules for the resource they
// configure), applies defaults to unset properties, and validates the
// final configuration against all matching constraints.
//
// Reference-valued properties are resolved into Result.References but not
// checked here, since checking that the target exists and satisfies any
// object constraints needs the whole environment; use EvaluateEnv for that.
//
// On failure it returns an ErrorList describing every problem found.
func (rs *RuleSet) Evaluate(res *Resource) (*Result, error) {
	result, diags := rs.evaluate(res, nil)
	if len(diags) > 0 {
		diags.sort()
		return nil, diags
	}
	return result, nil
}

// evaluate is the internal form of Evaluate, returning a best-effort
// result together with any diagnostics. extra holds synthesized rules
// (e.g. from dynamic blocks fired by EvaluateEnv) to match in addition to
// the rule set's own rules.
func (rs *RuleSet) evaluate(res *Resource, extra []*Rule) (*Result, ErrorList) {
	ev := &evaluator{
		rs:            rs,
		res:           res,
		norms:         make(map[*Rule][]normCond),
		reportedConds: make(map[*Condition]bool),
	}
	if res.Kind == "" {
		ev.diags.addf(nil, Position{}, Position{}, "resource kind must not be empty")
		return nil, ev.diags
	}

	var matched []*Rule
	collect := func(r *Rule) {
		if ev.matches(r) {
			matched = append(matched, r)
		}
	}
	rs.rules(collect)
	for _, r := range extra {
		collect(r)
	}

	// Group property rules across matching rules by property path.
	props := make(map[string][]ruleProp)
	var order []string
	for _, r := range matched {
		for _, p := range r.Props {
			if _, ok := props[p.Path]; !ok {
				order = append(order, p.Path)
			}
			props[p.Path] = append(props[p.Path], ruleProp{rule: r, prop: p})
		}
	}

	result := &Result{
		Resource:   res,
		Properties: make(map[string]ResolvedProperty, len(res.Config)+len(order)),
		Matched:    matched,
	}
	for path, v := range res.Config {
		result.Properties[path] = ResolvedProperty{Path: path, Value: v, Source: SourceExplicit}
	}

	for _, path := range order {
		rcs := props[path]
		if isRefPath(rcs) {
			ev.resolveRefProperty(path, rcs, result)
			continue
		}
		ev.resolveScalarProperty(path, rcs, result)
	}

	return result, ev.diags
}

// isRefPath reports whether the property at a path is reference-valued.
func isRefPath(rcs []ruleProp) bool {
	for _, rc := range rcs {
		if rc.prop.ref() != nil {
			return true
		}
	}
	return false
}

func (ev *evaluator) resolveScalarProperty(path string, rcs []ruleProp, result *Result) {
	res := ev.res
	final, have := res.Config[path]
	var defCand *defaultCandidate

	if !have {
		if cands := ev.collectDefaults(rcs); len(cands) > 0 {
			win, ambiguous := ev.resolveDefault(path, cands)
			if ambiguous != nil {
				// If the constraints themselves are impossible to satisfy
				// (e.g. conflicting exact values), report that as the root
				// cause instead of the ambiguity.
				before := len(ev.diags)
				ev.checkSatisfiable(path, rcs)
				if len(ev.diags) == before {
					ev.diags = append(ev.diags, ambiguous)
				}
				return
			}
			final, have, defCand = win.value, true, &win
			result.Properties[path] = ResolvedProperty{
				Path: path, Value: win.value, Source: SourceDefault, DefaultRule: win.rule,
			}
		}
	}

	if have {
		for _, rc := range rcs {
			sv := rc.prop.scalar()
			if sv == nil || sv.Constraint == nil {
				continue
			}
			ok, fail, mismatch := checkValue(final, sv.Constraint)
			switch {
			case mismatch != nil:
				ev.typeMismatch(path, final, rc, mismatch)
			case !ok:
				ev.violation(path, final, defCand, rc, fail)
			}
		}
	} else {
		for _, rc := range rcs {
			if sv := rc.prop.scalar(); sv != nil {
				if req := topLevelRequired(sv.Constraint); req != nil {
					ev.requiredMissing(path, rc, req)
				}
			}
		}
		ev.checkSatisfiable(path, rcs)
	}
}

// resolveRefProperty resolves a reference-valued property: it picks the
// most specific identity, records the resolved target, and emits the
// identity assertion and any object constraints into result.References for
// EvaluateEnv to check.
func (ev *evaluator) resolveRefProperty(path string, rcs []ruleProp, result *Result) {
	rs, res := ev.rs, ev.res

	targetKind, _ := rs.refTarget(res.Kind, path)
	if targetKind == "" {
		for _, rc := range rcs {
			if rv := rc.prop.ref(); rv != nil && rv.Ref != nil {
				targetKind = rv.Ref.Kind
				break
			}
		}
	}

	var (
		name       string
		haveID     bool
		idRule     *Rule
		idProp     *Property
		idSource   = SourceDefault
		unresolved string
	)
	if cv, ok := res.Config[path]; ok && cv.Kind == StringKind {
		name, haveID, idSource = cv.Str, true, SourceExplicit
	} else if cands := ev.collectRefs(rcs); len(cands) > 0 {
		win, ambiguous := ev.resolveRef(path, cands)
		if ambiguous != nil {
			ev.diags = append(ev.diags, ambiguous)
		} else {
			name, haveID = win.name, true
			idRule, idProp, unresolved = win.rule, win.prop, win.unresolved
			if targetKind == "" {
				targetKind = win.kind
			}
		}
	}

	if haveID {
		result.Properties[path] = ResolvedProperty{
			Path: path, Ref: &ResolvedRefValue{Kind: targetKind, Name: name},
			Source: idSource, DefaultRule: idRule,
		}
		result.References = append(result.References, ResolvedRef{
			Path: path, TargetKind: targetKind, TargetName: name,
			Rule: idRule, Prop: idProp, unresolved: unresolved,
		})
	}

	// Object constraints from every matching rule apply to the target.
	for _, rc := range rcs {
		rv := rc.prop.ref()
		if rv == nil || rv.Object == nil {
			continue
		}
		result.References = append(result.References, ResolvedRef{
			Path: path, TargetKind: targetKind, TargetName: name,
			Object: rv.Object, Rule: rc.rule, Prop: rc.prop, unresolved: unresolved,
		})
	}
}

// --- rule matching ---

func (ev *evaluator) matches(r *Rule) bool {
	// A dynamic block matches no resource directly; it fires via EvaluateEnv,
	// which synthesizes named rules for the resources it instantiates.
	if r.DynExpr != "" {
		return false
	}
	if r.Kind != ev.res.Kind {
		return false
	}
	if r.Name != "" && r.Name != ev.res.Name {
		return false
	}
	for _, c := range r.Where {
		if !ev.evalCond(r, c) {
			return false
		}
	}
	return true
}

func (ev *evaluator) lookupField(field string) (Value, bool) {
	if field == "name" {
		if ev.res.Name == "" {
			return Value{}, false
		}
		return String(ev.res.Name), true
	}
	v, ok := ev.res.Attrs[field]
	return v, ok
}

func (ev *evaluator) evalCond(r *Rule, c *Condition) bool {
	v, ok := ev.lookupField(c.Field)
	if c.Op == CondExists {
		return ok
	}
	if !ok {
		return false
	}
	switch c.Op {
	case CondEq, CondNeq:
		want := c.Values[0]
		if v.Kind != want.Kind {
			ev.selectorTypeMismatch(r, c, v, want)
			return false
		}
		eq := valuesEqual(v, want)
		if c.Op == CondNeq {
			return !eq
		}
		return eq
	case CondIn:
		kindOK := false
		for _, want := range c.Values {
			if want.Kind == v.Kind {
				kindOK = true
				if valuesEqual(v, want) {
					return true
				}
			}
		}
		if !kindOK {
			ev.selectorTypeMismatch(r, c, v, c.Values[0])
		}
		return false
	}
	return false
}

func (ev *evaluator) selectorTypeMismatch(r *Rule, c *Condition, got, want Value) {
	if ev.reportedConds[c] {
		return
	}
	ev.reportedConds[c] = true
	d := ev.diags.addf(ruleSrc(r), c.Pos, c.End,
		"type mismatch in selector condition '%s': attribute '%s' of %s is the %s %s, not a %s",
		c, c.Field, ev.res.describe(), got.Kind, got, want.Kind)
	d.Hint = "the rule is treated as not matching this resource"
}

// --- constraint checking ---

// checkValue checks a value against a constraint expression. It returns
// whether the constraint is satisfied; if not, fail is the smallest
// subexpression that failed. If the value's type does not match the
// constraint at all, mismatch is the offending comparison.
func checkValue(v Value, c Constraint) (ok bool, fail Constraint, mismatch *Comparison) {
	switch t := c.(type) {
	case *RequiredConstraint:
		return true, nil, nil // presence is checked separately
	case *Comparison:
		if v.Kind != t.Value.Kind {
			return false, t, t
		}
		if compareValues(v, t.Value, t.Op) {
			return true, nil, nil
		}
		return false, t, nil
	case *AndConstraint:
		for _, term := range t.Terms {
			if ok, fail, mismatch := checkValue(v, term); !ok {
				return false, fail, mismatch
			}
		}
		return true, nil, nil
	case *OrConstraint:
		var firstMismatch *Comparison
		allMismatch := true
		for _, alt := range t.Alts {
			ok, _, m := checkValue(v, alt)
			if m != nil {
				if firstMismatch == nil {
					firstMismatch = m
				}
				continue
			}
			allMismatch = false
			if ok {
				return true, nil, nil
			}
		}
		if allMismatch {
			return false, t, firstMismatch
		}
		return false, t, nil
	}
	return true, nil, nil
}

func compareValues(a, b Value, op CompareOp) bool {
	switch a.Kind {
	case NumberKind, SizeKind, DurationKind:
		cmp := 0
		if a.Num < b.Num {
			cmp = -1
		} else if a.Num > b.Num {
			cmp = 1
		}
		return op.holds(cmp)
	case BoolKind:
		switch op {
		case OpEq:
			return a.Bool == b.Bool
		case OpNeq:
			return a.Bool != b.Bool
		}
	case StringKind:
		switch op {
		case OpEq:
			return a.Str == b.Str
		case OpNeq:
			return a.Str != b.Str
		}
	}
	return false
}

// topLevelRequired returns the `required` constraint if it appears as a
// top-level conjunct of the expression, or nil.
func topLevelRequired(c Constraint) *RequiredConstraint {
	switch t := c.(type) {
	case *RequiredConstraint:
		return t
	case *AndConstraint:
		for _, term := range t.Terms {
			if req, ok := term.(*RequiredConstraint); ok {
				return req
			}
		}
	}
	return nil
}

// --- defaults ---

type defaultCandidate struct {
	rule     *Rule
	prop     *Property
	value    Value
	pos      Position
	end      Position
	implicit bool // value comes from an exact value constraint
}

// implicitDefault returns the exact value comparison that acts as an
// implicit default for the constraint, if any. An exact value constraint
// (optionally combined with `required`) acts as a default when unset.
func implicitDefault(c Constraint) (*Comparison, bool) {
	switch t := c.(type) {
	case *Comparison:
		if t.Op == OpEq {
			return t, true
		}
	case *AndConstraint:
		var eq *Comparison
		for _, term := range t.Terms {
			switch tt := term.(type) {
			case *RequiredConstraint:
				// ignore
			case *Comparison:
				if tt.Op != OpEq || (eq != nil && !valuesEqual(eq.Value, tt.Value)) {
					return nil, false
				}
				if eq == nil {
					eq = tt
				}
			default:
				return nil, false
			}
		}
		if eq != nil {
			return eq, true
		}
	}
	return nil, false
}

func (ev *evaluator) collectDefaults(rcs []ruleProp) []defaultCandidate {
	var cands []defaultCandidate
	for _, rc := range rcs {
		sv := rc.prop.scalar()
		if sv == nil {
			continue
		}
		if def := sv.Default; def != nil {
			cands = append(cands, defaultCandidate{
				rule: rc.rule, prop: rc.prop,
				value: def.Value, pos: def.ValuePos, end: def.ValueEnd,
			})
		} else if cmp, ok := implicitDefault(sv.Constraint); ok {
			cands = append(cands, defaultCandidate{
				rule: rc.rule, prop: rc.prop,
				value: cmp.Value, pos: cmp.Pos, end: cmp.End,
				implicit: true,
			})
		}
	}
	return cands
}

func (ev *evaluator) norm(r *Rule) []normCond {
	if n, ok := ev.norms[r]; ok {
		return n
	}
	n := normalizeRule(r)
	ev.norms[r] = n
	return n
}

// resolveDefault picks the default from the most specific matching rule.
// If several candidates provide different values and none is strictly
// more specific than all the others, it returns a diagnostic describing
// the ambiguity (without recording it).
func (ev *evaluator) resolveDefault(path string, cands []defaultCandidate) (defaultCandidate, *Diagnostic) {
	for i, c := range cands {
		winner := true
		for j, o := range cands {
			if i == j || valuesEqual(c.value, o.value) {
				continue
			}
			if !strictlyImplies(ev.norm(c.rule), ev.norm(o.rule)) {
				winner = false
				break
			}
		}
		if winner {
			return c, nil
		}
	}

	// Ambiguous: no candidate is strictly more specific than all others
	// that disagree with it.
	d := &Diagnostic{
		Pos:     cands[0].pos,
		End:     cands[0].end,
		Message: sprintf("ambiguous default for property '%s' of %s", path, ev.res.describe()),
		src:     ruleSrc(cands[0].rule),
	}
	d.Detail = []string{"matching rules provide different defaults:"}
	rules := make([]*Rule, 0, len(cands))
	for _, c := range cands {
		rules = append(rules, c.rule)
		d.Detail = append(d.Detail,
			fmt.Sprintf("  %s: %s", c.rule.Pos, c.rule.Header()),
			fmt.Sprintf("      %s", c.prop),
		)
	}
	d.Detail = append(d.Detail, "no rule is more specific than all the others")
	d.Hint = fmt.Sprintf("add a more specific rule that decides the default, e.g.:\n    for %s if %s {\n        %s: default %s\n    }",
		ev.res.Kind, mergedSelector(rules), path, cands[len(cands)-1].value)
	return defaultCandidate{}, d
}

// --- references ---

// refCandidate is a candidate identity for a reference-valued property,
// from an explicit `default <ref>` clause or a bare identity reference
// acting as an implicit default.
type refCandidate struct {
	rule       *Rule
	prop       *Property
	ref        *Reference
	kind       string // resolved target kind
	name       string // resolved target name; "" if unresolved
	unresolved string // reason the dynamic reference could not resolve; "" if resolved
	implicit   bool
	pos, end   Position
}

func (ev *evaluator) collectRefs(rcs []ruleProp) []refCandidate {
	var cands []refCandidate
	for _, rc := range rcs {
		rv := rc.prop.ref()
		if rv == nil {
			continue
		}
		switch {
		case rv.Default != nil:
			cands = append(cands, ev.refCandidate(rc, rv.Default.Ref, rv.Default.Pos, false))
		case rv.Ref != nil:
			// An identity reference acts as an implicit default, whether or
			// not it also carries an object constraint.
			cands = append(cands, ev.refCandidate(rc, rv.Ref, rv.Ref.Pos, true))
		}
	}
	return cands
}

func (ev *evaluator) refCandidate(rc ruleProp, ref *Reference, pos Position, implicit bool) refCandidate {
	kind, name, unresolved := ev.resolveReference(ref)
	return refCandidate{
		rule: rc.rule, prop: rc.prop, ref: ref,
		kind: kind, name: name, unresolved: unresolved,
		implicit: implicit, pos: pos, end: ref.End,
	}
}

// resolveReference resolves a reference to a concrete (kind, name). For a
// dynamic reference the expression is evaluated against the resource's
// attributes and normalized; unresolved is non-empty if it cannot resolve.
func (ev *evaluator) resolveReference(ref *Reference) (kind, name, unresolved string) {
	if ref.Mode == StaticRef {
		return ref.Kind, ref.Name, ""
	}
	v, ok := ev.lookupField(ref.Expr)
	if !ok {
		return ref.Kind, "", fmt.Sprintf("attribute '%s' is not set on %s", ref.Expr, ev.res.describe())
	}
	if v.Kind != StringKind {
		return ref.Kind, "", fmt.Sprintf("attribute '%s' is the %s %s, not a string", ref.Expr, v.Kind, v)
	}
	norm, err := normalizeDynamicName(v.Str)
	if err != nil {
		return ref.Kind, "", err.Error()
	}
	return ref.Kind, norm, ""
}

// resolveRef picks the reference identity from the most specific matching
// rule, mirroring resolveDefault. Two candidates agree when they point at
// the same (kind, name).
func (ev *evaluator) resolveRef(path string, cands []refCandidate) (refCandidate, *Diagnostic) {
	for i, c := range cands {
		winner := true
		for j, o := range cands {
			if i == j || refCandEqual(c, o) {
				continue
			}
			if !strictlyImplies(ev.norm(c.rule), ev.norm(o.rule)) {
				winner = false
				break
			}
		}
		if winner {
			return c, nil
		}
	}

	d := &Diagnostic{
		Pos:     cands[0].pos,
		End:     cands[0].end,
		Message: sprintf("ambiguous reference for property '%s' of %s", path, ev.res.describe()),
		src:     ruleSrc(cands[0].rule),
	}
	d.Detail = []string{"matching rules point the reference at different resources:"}
	for _, c := range cands {
		d.Detail = append(d.Detail,
			fmt.Sprintf("  %s: %s", c.rule.Pos, c.rule.Header()),
			fmt.Sprintf("      %s", c.prop),
		)
	}
	d.Detail = append(d.Detail, "no rule is more specific than all the others")
	return refCandidate{}, d
}

func refCandEqual(a, b refCandidate) bool {
	return a.kind == b.kind && a.name == b.name
}

// --- diagnostics ---

func (ev *evaluator) violation(path string, val Value, defCand *defaultCandidate, rc ruleProp, fail Constraint) {
	span := fail.span()
	var d *Diagnostic
	if defCand == nil {
		d = ev.diags.addf(ruleSrc(rc.rule), span.Start, span.End,
			"%s: property '%s' value %s violates constraint '%s'",
			ev.res.describe(), path, val, fail)
	} else {
		d = ev.diags.addf(ruleSrc(rc.rule), span.Start, span.End,
			"%s: default value %s for property '%s' violates constraint '%s'",
			ev.res.describe(), val, path, fail)
		d.Related = append(d.Related, RelatedInfo{
			Pos:     defCand.pos,
			Message: fmt.Sprintf("the default is defined at %s in rule: %s", defCand.pos, defCand.rule.Header()),
		})
	}
	d.Detail = []string{
		fmt.Sprintf("the constraint is defined at %s in rule:", rc.rule.Pos),
		"  " + rc.rule.Header(),
		"      " + rc.prop.String(),
	}
}

func (ev *evaluator) typeMismatch(path string, val Value, rc ruleProp, cmp *Comparison) {
	d := ev.diags.addf(ruleSrc(rc.rule), cmp.Pos, cmp.End,
		"%s: property '%s' has %s value %s, but the constraint '%s' expects a %s",
		ev.res.describe(), path, val.Kind, val, cmp, cmp.Value.Kind)
	d.Detail = []string{
		fmt.Sprintf("the constraint is defined at %s in rule:", rc.rule.Pos),
		"  " + rc.rule.Header(),
		"      " + rc.prop.String(),
	}
}

func (ev *evaluator) requiredMissing(path string, rc ruleProp, req *RequiredConstraint) {
	d := ev.diags.addf(ruleSrc(rc.rule), req.Pos, req.End,
		"%s: property '%s' is required but not set, and no default applies",
		ev.res.describe(), path)
	d.Detail = []string{
		fmt.Sprintf("required by rule at %s:", rc.rule.Pos),
		"  " + rc.rule.Header(),
		"      " + rc.prop.String(),
	}
	d.Hint = fmt.Sprintf("set '%s' on the resource, or add 'default <value>' to a matching rule", path)
}
