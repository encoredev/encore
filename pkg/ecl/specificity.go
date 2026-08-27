package ecl

import (
	"sort"
	"strings"
)

// This file implements rule specificity: a rule is more specific than
// another if its selector logically implies the other's selector.
// A rule's optional resource name is folded into the selector as a
// `name == "..."` condition, so it participates in specificity naturally.

// normCond is a normalized selector condition used for implication and
// contradiction checks.
type normCond struct {
	field  string
	op     CondOp
	values []Value
}

func (c normCond) String() string {
	ast := Condition{Field: c.field, Op: c.op, Values: c.values}
	return ast.String()
}

// normalizeRule returns the rule's effective selector conditions,
// including the resource name from the rule header.
func normalizeRule(r *Rule) []normCond {
	conds := make([]normCond, 0, len(r.Where)+1)
	if r.Name != "" {
		conds = append(conds, normCond{field: "name", op: CondEq, values: []Value{String(r.Name)}})
	}
	for _, c := range r.Where {
		conds = append(conds, normCond{field: c.Field, op: c.Op, values: c.Values})
	}
	return conds
}

// implies reports whether selector a logically implies selector b:
// every resource matching a also matches b.
func implies(a, b []normCond) bool {
	for _, bc := range b {
		ok := false
		for _, ac := range a {
			if entails(ac, bc) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// strictlyImplies reports whether a is strictly more specific than b.
func strictlyImplies(a, b []normCond) bool {
	return implies(a, b) && !implies(b, a)
}

// entails reports whether condition a being true guarantees condition b
// is true.
func entails(a, b normCond) bool {
	if a.field != b.field {
		return false
	}
	switch b.op {
	case CondExists:
		// Any condition on the field requires it to exist.
		return true
	case CondEq:
		v := b.values[0]
		switch a.op {
		case CondEq:
			return valuesEqual(a.values[0], v)
		case CondIn:
			return len(a.values) == 1 && valuesEqual(a.values[0], v)
		}
	case CondIn:
		switch a.op {
		case CondEq:
			return valueInSet(a.values[0], b.values)
		case CondIn:
			return valueSubset(a.values, b.values)
		}
	case CondNeq:
		w := b.values[0]
		switch a.op {
		case CondNeq:
			return valuesEqual(a.values[0], w)
		case CondEq:
			return !valuesEqual(a.values[0], w)
		case CondIn:
			return !valueInSet(w, a.values)
		}
	}
	return false
}

// contradicts reports whether two conditions can never both hold for the
// same resource.
func contradicts(a, b normCond) bool {
	if a.field != b.field {
		return false
	}
	// Normalize so the "smaller" op comes first for fewer cases.
	if a.op > b.op {
		a, b = b, a
	}
	switch {
	case a.op == CondEq && b.op == CondEq:
		return !valuesEqual(a.values[0], b.values[0])
	case a.op == CondEq && b.op == CondNeq:
		return valuesEqual(a.values[0], b.values[0])
	case a.op == CondEq && b.op == CondIn:
		return !valueInSet(a.values[0], b.values)
	case a.op == CondNeq && b.op == CondIn:
		return len(b.values) == 1 && valuesEqual(b.values[0], a.values[0])
	case a.op == CondIn && b.op == CondIn:
		return valuesDisjoint(a.values, b.values)
	}
	return false
}

// selectorsCanCoMatch reports whether some resource could match both
// selectors. This is a conservative check: it only detects direct
// per-field contradictions.
func selectorsCanCoMatch(a, b []normCond) bool {
	for _, ac := range a {
		for _, bc := range b {
			if contradicts(ac, bc) {
				return false
			}
		}
	}
	return true
}

// mergedSelector renders the union of conditions of the given rules as a
// selector expression, for use in hints.
func mergedSelector(rules []*Rule) string {
	seen := make(map[string]bool)
	var conds []normCond
	for _, r := range rules {
		for _, c := range normalizeRule(r) {
			key := c.String()
			if !seen[key] {
				seen[key] = true
				conds = append(conds, c)
			}
		}
	}
	sort.SliceStable(conds, func(i, j int) bool { return conds[i].field < conds[j].field })
	parts := make([]string, len(conds))
	for i, c := range conds {
		parts[i] = c.String()
	}
	return strings.Join(parts, " && ")
}

func valueInSet(v Value, set []Value) bool {
	for _, s := range set {
		if valuesEqual(v, s) {
			return true
		}
	}
	return false
}

func valueSubset(a, b []Value) bool {
	for _, v := range a {
		if !valueInSet(v, b) {
			return false
		}
	}
	return true
}

func valuesDisjoint(a, b []Value) bool {
	for _, v := range a {
		if valueInSet(v, b) {
			return false
		}
	}
	return true
}
