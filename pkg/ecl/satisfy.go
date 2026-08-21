package ecl

import (
	"fmt"
)

// This file implements satisfiability checking of merged constraints:
// detecting that no possible value could satisfy the combination of
// constraints from a set of rules (impossible ranges, conflicting exact
// values, empty allowed-value sets, and conflicting types).
//
// The analysis is conservative: it fully understands conjunctions of
// comparisons and disjunctions of exact values; other shapes (such as
// disjunctions containing ranges) are skipped rather than guessed at.
// Concrete values are always checked precisely by checkValue.

type satEntry struct {
	cmp *Comparison
	src ruleProp
}

func (e satEntry) describeAt() string {
	span := e.cmp.span()
	return fmt.Sprintf("'%s' at %s in rule: %s", e.cmp, span.Start, e.src.rule.Header())
}

type orEntry struct {
	or     *OrConstraint
	values []Value
	src    ruleProp
}

func (e orEntry) describeAt() string {
	span := e.or.span()
	return fmt.Sprintf("'%s' at %s in rule: %s", e.or, span.Start, e.src.rule.Header())
}

type satChecker struct {
	path     string
	resource string // resource description for messages; "" outside evaluation
	diags    *ErrorList

	kindSet   bool
	kind      ValueKind
	kindFirst satEntry

	lower, upper *satEntry // >= / > and <= / <
	exact        *satEntry // ==
	excluded     []satEntry
	sets         []orEntry // disjunctions of exact values

	bail   bool // saw a shape the analysis does not understand
	failed bool // already reported a conflict
}

// checkSatisfiable verifies that the merged constraints for a property
// admit at least one value, reporting conflicts otherwise.
func (ev *evaluator) checkSatisfiable(path string, rcs []ruleProp) {
	sc := &satChecker{path: path, resource: ev.res.describe(), diags: &ev.diags}
	for _, rc := range rcs {
		sc.feed(rc)
	}
	sc.check()
}

func (sc *satChecker) feed(rc ruleProp) {
	sv := rc.prop.scalar()
	if sv == nil || sv.Constraint == nil {
		return
	}
	for _, term := range conjuncts(sv.Constraint) {
		switch t := term.(type) {
		case *RequiredConstraint:
			// presence is checked separately
		case *Comparison:
			sc.addComparison(satEntry{cmp: t, src: rc})
		case *OrConstraint:
			values, ok := exactAlternatives(t)
			if !ok {
				sc.bail = true
				continue
			}
			for i := range values {
				// Type-check the set members against the property's kind.
				sc.checkKind(values[i].Kind, satEntry{cmp: t.Alts[i].(*Comparison), src: rc})
			}
			sc.sets = append(sc.sets, orEntry{or: t, values: values, src: rc})
		default:
			sc.bail = true
		}
	}
}

// conjuncts flattens top-level conjunctions into a list of terms.
func conjuncts(c Constraint) []Constraint {
	if and, ok := c.(*AndConstraint); ok {
		var out []Constraint
		for _, t := range and.Terms {
			out = append(out, conjuncts(t)...)
		}
		return out
	}
	return []Constraint{c}
}

// exactAlternatives returns the values of a disjunction whose alternatives
// are all exact value comparisons, e.g. `"a" | "b" | "c"`.
func exactAlternatives(or *OrConstraint) ([]Value, bool) {
	values := make([]Value, 0, len(or.Alts))
	for _, alt := range or.Alts {
		cmp, ok := alt.(*Comparison)
		if !ok || cmp.Op != OpEq {
			return nil, false
		}
		values = append(values, cmp.Value)
	}
	return values, true
}

func (sc *satChecker) addComparison(e satEntry) {
	if !sc.checkKind(e.cmp.Value.Kind, e) {
		return
	}
	switch e.cmp.Op {
	case OpEq:
		if sc.exact != nil && !valuesEqual(sc.exact.cmp.Value, e.cmp.Value) {
			sc.conflict(*sc.exact, e,
				"it cannot equal both %s and %s", sc.exact.cmp.Value, e.cmp.Value)
			return
		}
		if sc.exact == nil {
			sc.exact = &e
		}
	case OpNeq:
		sc.excluded = append(sc.excluded, e)
	case OpGe, OpGt:
		if sc.lower == nil || tighterLower(e.cmp, sc.lower.cmp) {
			sc.lower = &e
		}
	case OpLe, OpLt:
		if sc.upper == nil || tighterUpper(e.cmp, sc.upper.cmp) {
			sc.upper = &e
		}
	}
}

func tighterLower(a, b *Comparison) bool {
	if a.Value.Num != b.Value.Num {
		return a.Value.Num > b.Value.Num
	}
	return a.Op == OpGt && b.Op == OpGe
}

func tighterUpper(a, b *Comparison) bool {
	if a.Value.Num != b.Value.Num {
		return a.Value.Num < b.Value.Num
	}
	return a.Op == OpLt && b.Op == OpLe
}

// checkKind enforces that all constraints on the property use the same
// value type.
func (sc *satChecker) checkKind(k ValueKind, e satEntry) bool {
	if sc.failed {
		return false
	}
	if !sc.kindSet {
		sc.kindSet, sc.kind, sc.kindFirst = true, k, e
		return true
	}
	if sc.kind != k {
		first := sc.kindFirst
		span := e.cmp.span()
		d := sc.diags.addf(ruleSrc(e.src.rule), span.Start, span.End,
			"conflicting types for property '%s': '%s' is a %s constraint, but '%s' compares against a %s",
			sc.path, first.cmp, sc.kind, e.cmp, k)
		d.Detail = []string{
			"  " + first.describeAt(),
			"  " + e.describeAt(),
		}
		sc.failed = true
		return false
	}
	return true
}

func (sc *satChecker) check() {
	if sc.failed || sc.bail {
		return
	}

	if sc.exact != nil {
		v := sc.exact.cmp.Value
		for _, bound := range []*satEntry{sc.lower, sc.upper} {
			if bound != nil && !compareValues(v, bound.cmp.Value, bound.cmp.Op) {
				sc.conflict(*sc.exact, *bound, "no value can satisfy both")
				return
			}
		}
		for _, ex := range sc.excluded {
			if valuesEqual(v, ex.cmp.Value) {
				sc.conflict(*sc.exact, ex, "it cannot both equal and not equal %s", v)
				return
			}
		}
		for _, set := range sc.sets {
			if !valueInSet(v, set.values) {
				sc.exactVsSet(*sc.exact, set)
				return
			}
		}
		return
	}

	// Bounds: highest minimum must not exceed lowest maximum.
	if sc.lower != nil && sc.upper != nil {
		lo, hi := sc.lower.cmp, sc.upper.cmp
		if lo.Value.Num > hi.Value.Num ||
			(lo.Value.Num == hi.Value.Num && (lo.Op == OpGt || hi.Op == OpLt)) {
			sc.conflict(*sc.lower, *sc.upper, "no value can satisfy both")
			return
		}
	}

	// Allowed-value sets: the intersection, filtered by bounds and
	// exclusions, must be non-empty.
	if len(sc.sets) > 0 {
		allowed := sc.sets[0].values
		for _, set := range sc.sets[1:] {
			var next []Value
			for _, v := range allowed {
				if valueInSet(v, set.values) {
					next = append(next, v)
				}
			}
			allowed = next
		}
		var viable []Value
		for _, v := range allowed {
			ok := true
			for _, bound := range []*satEntry{sc.lower, sc.upper} {
				if bound != nil && !compareValues(v, bound.cmp.Value, bound.cmp.Op) {
					ok = false
				}
			}
			for _, ex := range sc.excluded {
				if valuesEqual(v, ex.cmp.Value) {
					ok = false
				}
			}
			if ok {
				viable = append(viable, v)
			}
		}
		if len(viable) == 0 {
			sc.emptySets()
			return
		}
	}

	// Booleans only have two values; excluding both is impossible.
	if sc.kindSet && sc.kind == BoolKind && len(sc.sets) == 0 {
		var exTrue, exFalse *satEntry
		for i, ex := range sc.excluded {
			if ex.cmp.Value.Bool {
				exTrue = &sc.excluded[i]
			} else {
				exFalse = &sc.excluded[i]
			}
		}
		if exTrue != nil && exFalse != nil {
			sc.conflict(*exTrue, *exFalse, "a bool cannot differ from both true and false")
		}
	}
}

func (sc *satChecker) header() string {
	if sc.resource != "" {
		return fmt.Sprintf("impossible constraints for property '%s' of %s", sc.path, sc.resource)
	}
	return fmt.Sprintf("impossible constraints for property '%s'", sc.path)
}

func (sc *satChecker) conflict(a, b satEntry, format string, args ...any) {
	sc.failed = true
	span := b.cmp.span()
	d := sc.diags.addf(ruleSrc(b.src.rule), span.Start, span.End,
		"%s: '%s' conflicts with '%s': %s", sc.header(), a.cmp, b.cmp, sprintf(format, args...))
	d.Detail = []string{
		"  " + a.describeAt(),
		"  " + b.describeAt(),
	}
	if a.src.rule != b.src.rule {
		d.Hint = "constraints from all matching rules merge by intersection; a rule cannot weaken another rule's constraints"
	}
}

func (sc *satChecker) exactVsSet(e satEntry, set orEntry) {
	sc.failed = true
	span := e.cmp.span()
	d := sc.diags.addf(ruleSrc(e.src.rule), span.Start, span.End,
		"%s: %s is not one of the allowed values %s", sc.header(), e.cmp.Value, set.or)
	d.Detail = []string{
		"  " + e.describeAt(),
		"  " + set.describeAt(),
	}
	sc.failed = true
}

func (sc *satChecker) emptySets() {
	sc.failed = true
	first := sc.sets[0]
	span := first.or.span()
	d := sc.diags.addf(ruleSrc(first.src.rule), span.Start, span.End,
		"%s: no value satisfies all the allowed-value constraints", sc.header())
	for _, set := range sc.sets {
		d.Detail = append(d.Detail, "  "+set.describeAt())
	}
	for _, bound := range []*satEntry{sc.lower, sc.upper} {
		if bound != nil {
			d.Detail = append(d.Detail, "  "+bound.describeAt())
		}
	}
	for _, ex := range sc.excluded {
		d.Detail = append(d.Detail, "  "+ex.describeAt())
	}
}
