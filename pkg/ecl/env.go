package ecl

import (
	"fmt"
	"maps"
)

// Definition is a managed infrastructure resource instantiated by a static
// named block (`<kind> "name"`) of a managed kind whose selector matches an
// environment.
type Definition struct {
	Kind string
	Name string
	Rule *Rule // the (first) named block that instantiated the resource
}

// Definitions returns the managed infrastructure resources instantiated by
// static named blocks for an environment with the given selector
// attributes. A resource instantiated by several blocks is returned once.
// Dynamic blocks are not enumerated here, since their names depend on the
// resources evaluated by EvaluateEnv.
func (rs *RuleSet) Definitions(attrs map[string]Value) ([]Definition, error) {
	defs, diags := rs.definitions(attrs)
	if len(diags) > 0 {
		diags.sort()
		return nil, diags
	}
	return defs, nil
}

type resourceKey struct{ kind, name string }

func (rs *RuleSet) newEvaluator(res *Resource) *evaluator {
	return &evaluator{
		rs:            rs,
		res:           res,
		norms:         make(map[*Rule][]normCond),
		reportedConds: make(map[*Condition]bool),
	}
}

func (rs *RuleSet) definitions(attrs map[string]Value) ([]Definition, ErrorList) {
	var defs []Definition
	var diags ErrorList
	seen := make(map[resourceKey]bool)
	rs.rules(func(r *Rule) {
		if r.Name == "" || !rs.isManaged(r.Kind) {
			return
		}
		ev := rs.newEvaluator(&Resource{Kind: r.Kind, Name: r.Name, Attrs: attrs})
		if ev.matches(r) {
			k := resourceKey{r.Kind, r.Name}
			if !seen[k] {
				seen[k] = true
				defs = append(defs, Definition{Kind: r.Kind, Name: r.Name, Rule: r})
			}
		}
		diags = append(diags, ev.diags...)
	})
	return defs, diags
}

// EnvResult is the outcome of evaluating all resources of an environment
// together.
type EnvResult struct {
	// Results holds one result per evaluated resource: the input resources
	// in their given order, followed by resources instantiated by static
	// managed blocks and dynamic blocks that were not in the input.
	Results []*Result
}

// Get returns the result for the resource with the given kind and name,
// or nil if there is none.
func (er *EnvResult) Get(kind, name string) *Result {
	for _, r := range er.Results {
		if r.Resource.Kind == kind && r.Resource.Name == name {
			return r
		}
	}
	return nil
}

// EvaluateEnv evaluates all resources of an environment together:
//
//   - static named blocks of managed kinds matching envAttrs are
//     instantiated as resources (unless a resource with the same kind and
//     name is already in the input),
//   - dynamic blocks nested in matching rules are fired against the input
//     resources, instantiating and configuring kind/normalize(expr) per
//     match (resources sharing a normalized name merge),
//   - every resource is evaluated individually as with Evaluate,
//   - reference-valued properties are checked: the target must exist in the
//     environment, and any nested object constraints must hold.
//
// envAttrs are the selector attributes of the environment itself (e.g.
// env.type, env.name, provider); they decide which blocks apply, become the
// attributes of instantiated resources, and are merged into each input
// resource's attributes (the resource's own attributes win on conflict).
// Result.Resource may therefore point to an augmented copy of an input
// resource.
//
// On failure it returns an ErrorList describing every problem found across
// all resources.
func (rs *RuleSet) EvaluateEnv(envAttrs map[string]Value, resources []*Resource) (*EnvResult, error) {
	var diags ErrorList

	// Stage 1: augment input resources with env attributes.
	inputs := make([]*Resource, 0, len(resources))
	present := make(map[resourceKey]bool, len(resources))
	for _, res := range resources {
		if len(envAttrs) > 0 {
			attrs := make(map[string]Value, len(envAttrs)+len(res.Attrs))
			maps.Copy(attrs, envAttrs)
			maps.Copy(attrs, res.Attrs)
			cp := *res
			cp.Attrs = attrs
			res = &cp
		}
		inputs = append(inputs, res)
		present[resourceKey{res.Kind, res.Name}] = true
	}
	all := append([]*Resource(nil), inputs...)

	// Stage 2: instantiate static managed named blocks.
	defs, defDiags := rs.definitions(envAttrs)
	diags = append(diags, defDiags...)
	for _, def := range defs {
		k := resourceKey{def.Kind, def.Name}
		if !present[k] {
			present[k] = true
			all = append(all, &Resource{Kind: def.Kind, Name: def.Name, Attrs: envAttrs})
		}
	}

	// Stage 3: fire dynamic blocks against the input resources.
	newRes, overlay, dynDiags := rs.fireDynamicBlocks(envAttrs, inputs, present)
	diags = append(diags, dynDiags...)
	all = append(all, newRes...)

	// Stage 4: evaluate every resource.
	results := make([]*Result, 0, len(all))
	byKey := make(map[resourceKey]*Result, len(all))
	for _, res := range all {
		result, rdiags := rs.evaluate(res, overlay)
		if len(rdiags) > 0 {
			diags = append(diags, rdiags...)
			continue
		}
		results = append(results, result)
		k := resourceKey{res.Kind, res.Name}
		if _, dup := byKey[k]; !dup {
			byKey[k] = result
		}
	}

	// Stage 5: resolve and check references.
	for _, result := range results {
		for _, ref := range result.References {
			rs.checkRef(result, ref, byKey, &diags)
		}
	}

	if len(diags) > 0 {
		diags.sort()
		return nil, diags
	}
	return &EnvResult{Results: results}, nil
}

// fireDynamicBlocks evaluates dynamic blocks nested in matching rules
// against the input resources. For each block firing it instantiates the
// target resource (for managed kinds) and registers a synthesized named
// rule so the block's properties merge during evaluation. Resources whose
// expression normalizes to the same name share one instance.
func (rs *RuleSet) fireDynamicBlocks(envAttrs map[string]Value, inputs []*Resource, present map[resourceKey]bool) (newRes []*Resource, overlay []*Rule, diags ErrorList) {
	collisions := make(map[resourceKey]string) // (kind, name) -> source value
	type cloneKey struct {
		block *Rule
		name  string
	}
	cloned := make(map[cloneKey]bool)

	rs.rules(func(parent *Rule) {
		if len(parent.Blocks) == 0 {
			return
		}
		for _, in := range inputs {
			ev := rs.newEvaluator(in)
			if !ev.matches(parent) {
				diags = append(diags, ev.diags...)
				continue
			}
			for _, b := range parent.Blocks {
				if !ev.whereMatches(b) {
					continue
				}
				name, raw, ok := ev.blockName(b, &diags)
				if !ok {
					continue
				}
				key := resourceKey{b.Kind, name}
				if prev, seen := collisions[key]; seen && prev != raw {
					d := diags.addf(ruleSrc(b), b.DynExprPos, b.DynExprEnd,
						"dynamic block names %q and %q both normalize to %s %q",
						prev, raw, b.Kind, name)
					d.Hint = "give the resources distinct names that do not collide after normalization"
					continue
				}
				collisions[key] = raw

				if rs.isManaged(b.Kind) && !present[key] {
					present[key] = true
					newRes = append(newRes, &Resource{Kind: b.Kind, Name: name, Attrs: envAttrs})
				}
				if ck := (cloneKey{b, name}); !cloned[ck] {
					cloned[ck] = true
					overlay = append(overlay, b.asNamed(name))
				}
			}
			diags = append(diags, ev.diags...)
		}
	})
	return newRes, overlay, diags
}

// whereMatches reports whether a nested block's own selector matches the
// resource the enclosing rule iterates over.
func (ev *evaluator) whereMatches(b *Rule) bool {
	for _, c := range b.Where {
		if !ev.evalCond(b, c) {
			return false
		}
	}
	return true
}

// blockName resolves the resource name a nested block instantiates for the
// resource ev iterates over, returning the normalized name and the source
// value. ok is false if the dynamic expression could not be resolved.
func (ev *evaluator) blockName(b *Rule, diags *ErrorList) (name, raw string, ok bool) {
	if b.DynExpr == "" {
		return b.Name, b.Name, true // static nested block
	}
	v, found := ev.lookupField(b.DynExpr)
	if !found {
		d := diags.addf(ruleSrc(b), b.DynExprPos, b.DynExprEnd,
			"%s: dynamic block attribute '%s' is not set", ev.res.describe(), b.DynExpr)
		d.Hint = "the block is only instantiated for resources where the attribute is set"
		return "", "", false
	}
	if v.Kind != StringKind {
		diags.addf(ruleSrc(b), b.DynExprPos, b.DynExprEnd,
			"%s: dynamic block attribute '%s' is the %s %s, not a string",
			ev.res.describe(), b.DynExpr, v.Kind, v)
		return "", "", false
	}
	norm, err := normalizeDynamicName(v.Str)
	if err != nil {
		diags.addf(ruleSrc(b), b.DynExprPos, b.DynExprEnd, "%s: %s", ev.res.describe(), err)
		return "", "", false
	}
	return norm, v.Str, true
}

// asNamed clones a dynamic (or static nested) block into a named rule for
// the resource it instantiates, so the existing matching, specificity, and
// merge logic applies. The block's own selector is dropped: it was already
// evaluated against the resource that triggered the block.
func (b *Rule) asNamed(name string) *Rule {
	return &Rule{
		Pos:     b.Pos,
		Kind:    b.Kind,
		KindPos: b.KindPos,
		KindEnd: b.KindEnd,
		Name:    name,
		NamePos: b.NamePos,
		Props:   b.Props,
		Blocks:  b.Blocks,
		file:    b.file,
	}
}

// checkRef checks one resolved reference against the environment: the target
// resource must exist, and any nested object constraints must hold against
// its resolved configuration.
func (rs *RuleSet) checkRef(result *Result, rr ResolvedRef, byKey map[resourceKey]*Result, diags *ErrorList) {
	res := result.Resource
	src := ruleSrc(rr.Rule)
	refPos, refEnd := Position{}, Position{}
	if rr.Prop != nil {
		refPos, refEnd = rr.Prop.Pos, rr.Prop.PathEnd
	}
	var ruleDetail []string
	if rr.Rule != nil {
		ruleDetail = []string{
			fmt.Sprintf("the constraint is defined at %s in rule:", rr.Rule.Pos),
			"  " + rr.Rule.Header(),
		}
	}

	if rr.unresolved != "" {
		d := diags.addf(src, refPos, refEnd,
			"%s: cannot resolve the reference for property '%s': %s",
			res.describe(), rr.Path, rr.unresolved)
		d.Detail = ruleDetail
		return
	}
	if rr.TargetName == "" {
		kind := rr.TargetKind
		if kind == "" {
			kind = "resource"
		}
		d := diags.addf(src, refPos, refEnd,
			"%s: property '%s' is not set, but a constraint applies to the referenced %s",
			res.describe(), rr.Path, kind)
		d.Detail = ruleDetail
		d.Hint = fmt.Sprintf("set '%s' on the resource or add a default to a matching rule", rr.Path)
		return
	}
	target := byKey[resourceKey{rr.TargetKind, rr.TargetName}]
	if target == nil {
		d := diags.addf(src, refPos, refEnd,
			"%s: property '%s' references %s %q, but no such resource exists in the environment",
			res.describe(), rr.Path, rr.TargetKind, rr.TargetName)
		d.Detail = ruleDetail
		if rs.isManaged(rr.TargetKind) {
			d.Hint = fmt.Sprintf("instantiate it with: %s %q { ... }", rr.TargetKind, rr.TargetName)
		} else {
			d.Hint = fmt.Sprintf("no %s named %q exists in the application", rr.TargetKind, rr.TargetName)
		}
		return
	}

	if rr.Object == nil {
		return // identity-only entry: the target exists, nothing more to check
	}

	targetDesc := fmt.Sprintf("%s %q", rr.TargetKind, rr.TargetName)
	for _, p := range rr.Object.Props {
		tv, ok := target.Properties[p.Path]
		if !ok {
			d := diags.addf(src, p.Pos, p.PathEnd,
				"%s: the referenced %s does not set property '%s', which the constraint needs",
				res.describe(), targetDesc, p.Path)
			d.Detail = append(append([]string{}, ruleDetail...), "      "+rr.Path+": { "+p.String()+" }")
			d.Hint = fmt.Sprintf("set '%s' on the %s, or give it a default in a rule for that kind", p.Path, rr.TargetKind)
			continue
		}
		sv := p.scalar()
		if sv == nil || sv.Constraint == nil || tv.Ref != nil {
			continue
		}
		ok, fail, mismatch := checkValue(tv.Value, sv.Constraint)
		switch {
		case mismatch != nil:
			d := diags.addf(src, mismatch.Pos, mismatch.End,
				"%s: property '%s' of the referenced %s has %s value %s, but the constraint '%s' expects a %s",
				res.describe(), p.Path, targetDesc, tv.Value.Kind, tv.Value, mismatch, mismatch.Value.Kind)
			d.Detail = ruleDetail
		case !ok:
			span := fail.span()
			d := diags.addf(src, span.Start, span.End,
				"%s: the referenced %s has '%s' = %s, violating the constraint '%s'",
				res.describe(), targetDesc, p.Path, tv.Value, fail)
			d.Detail = ruleDetail
			if tv.Source == SourceDefault && tv.DefaultRule != nil {
				d.Related = append(d.Related, RelatedInfo{
					Pos: tv.DefaultRule.Pos,
					Message: fmt.Sprintf("the value comes from a default in rule at %s: %s",
						tv.DefaultRule.Pos, tv.DefaultRule.Header()),
				})
			}
		}
	}
}
