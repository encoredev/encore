package ecl

import (
	"strconv"
	"strings"
)

// File is a parsed ECL source file.
type File struct {
	Path    string
	Version *Version  // nil if no version declaration
	Imports []*Import // import declarations, in source order
	Rules   []*Rule   // rules, in source order

	src *sourceFile
}

// Version is a "version N" declaration.
type Version struct {
	Pos Position
	Num int
}

// Import is an `import "path"` declaration.
type Import struct {
	Pos     Position // position of the 'import' keyword
	Path    string
	PathPos Position // position of the path string
	PathEnd Position
}

// Rule is a resource block. It takes one of three header forms:
//
//	for <kind> [if <selector>] { ... }   // Name == "" && DynExpr == ""
//	<kind> "<name>" [if <selector>] { ... }  // Name != ""
//	<kind> <expr> [if <selector>] { ... }    // DynExpr != ""
//
// A named block configures (and, for managed kinds, instantiates) the one
// resource kind/name. A dynamic block instantiates/configures
// kind/normalize(expr) per matching resource of the enclosing block. A
// `for` block configures all resources of the kind matching the selector.
//
// Rules written inside `if` blocks are desugared at parse time: the
// enclosing block's conditions are prepended to Where, so a Rule always
// carries its full effective selector.
type Rule struct {
	Pos     Position // position of the 'for' keyword or the kind identifier
	Kind    string   // resource kind, e.g. "service"
	KindPos Position
	KindEnd Position

	Name    string // static named block; "" otherwise
	NamePos Position

	// DynExpr is the attribute path of a dynamic block (`kind <expr> { ... }`),
	// e.g. "tags.domain"; "" for non-dynamic blocks. A dynamic block is
	// evaluated against the resources matched by its enclosing rule.
	DynExpr    string
	DynExprPos Position
	DynExprEnd Position

	Where  []*Condition // effective selector conditions joined by &&; nil if none
	Props  []*Property
	Blocks []*Rule // resource blocks nested in this rule's body (dynamic instantiation)

	file *File
}

// Header reconstructs the rule's effective header as source text,
// e.g. `for service "api" if env.type == "production"`. For rules
// written inside `if` blocks this is the desugared form.
func (r *Rule) Header() string {
	var b strings.Builder
	if r.Name == "" && r.DynExpr == "" {
		b.WriteString("for ")
	}
	b.WriteString(r.Kind)
	switch {
	case r.Name != "":
		b.WriteString(" ")
		b.WriteString(strconv.Quote(r.Name))
	case r.DynExpr != "":
		b.WriteString(" ")
		b.WriteString(r.DynExpr)
	}
	if len(r.Where) > 0 {
		b.WriteString(" if ")
		for i, c := range r.Where {
			if i > 0 {
				b.WriteString(" && ")
			}
			b.WriteString(c.String())
		}
	}
	return b.String()
}

// CondOp is a selector condition operator.
type CondOp int

const (
	CondEq CondOp = iota
	CondNeq
	CondIn
	CondExists
)

// Condition is a single selector condition, e.g. `env.type == production`.
type Condition struct {
	Pos      Position // start of the field path
	End      Position // just past the condition
	Field    string   // dotted field path, e.g. "env.type"
	FieldEnd Position
	Op       CondOp
	Values   []Value // one value for ==/!=, one or more for in, none for exists

	// EnvScoped is set for conditions that originate from an `if` block. Such
	// conditions are evaluated in the top-level environment scope and may only
	// reference declared environment attributes (see RuleSet.EnvScope); a
	// `if` clause on a for/named rule is resource-scoped and leaves this
	// false.
	EnvScoped bool
}

func (c *Condition) String() string {
	switch c.Op {
	case CondEq:
		return c.Field + " == " + c.Values[0].String()
	case CondNeq:
		return c.Field + " != " + c.Values[0].String()
	case CondExists:
		return c.Field + " exists"
	case CondIn:
		var b strings.Builder
		b.WriteString(c.Field)
		b.WriteString(" in [")
		for i, v := range c.Values {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(v.String())
		}
		b.WriteString("]")
		return b.String()
	default:
		return "<invalid condition>"
	}
}

// Property is a single property rule inside a rule or object-constraint
// body, e.g. `cpu: >= 1 & <= 4 | default 2` or `cluster: sql_cluster.main`.
type Property struct {
	Pos     Position // start of the property path
	PathEnd Position
	Path    string // dotted property path, e.g. "instances.min"

	// Value is the right-hand side: either a scalar constraint (ScalarValue)
	// or a reference constraint (RefValue).
	Value PropertyValue
}

func (p *Property) String() string {
	return p.Path + ": " + p.Value.String()
}

// scalar returns the ScalarValue if the property is scalar-valued, else nil.
func (p *Property) scalar() *ScalarValue {
	sv, _ := p.Value.(*ScalarValue)
	return sv
}

// ref returns the RefValue if the property is reference-valued, else nil.
func (p *Property) ref() *RefValue {
	rv, _ := p.Value.(*RefValue)
	return rv
}

// PropertyValue is the right-hand side of a property rule.
type PropertyValue interface {
	propertyValue()
	String() string
}

// ScalarValue constrains a scalar property with an optional default,
// e.g. `>= 1 & <= 4 | default 2`.
type ScalarValue struct {
	Constraint Constraint // nil if default-only
	Default    *Default   // nil if no default clause
}

func (*ScalarValue) propertyValue() {}

func (v *ScalarValue) String() string {
	var b strings.Builder
	if v.Constraint != nil {
		b.WriteString(v.Constraint.String())
		if v.Default != nil {
			b.WriteString(" | ")
		}
	}
	if v.Default != nil {
		b.WriteString("default ")
		b.WriteString(v.Default.Value.String())
	}
	return b.String()
}

// RefValue constrains a reference-valued property: an identity reference
// and/or nested object constraints on the resolved target, with an optional
// reference default. For example:
//
//	cluster: sql_cluster.main                  // identity (and implicit default)
//	cluster: { backup_retention: >= 30d }      // object-only
//	cluster: sql_cluster.audit & { ... }       // identity + object
//	cluster: default sql_cluster.main          // reference default
type RefValue struct {
	Ref     *Reference        // identity; nil if object-only or default-only
	Object  *ObjectConstraint // nested constraints on the target; nil if none
	Default *RefDefault       // `default <ref>`; nil if no default clause
}

func (*RefValue) propertyValue() {}

func (v *RefValue) String() string {
	var parts []string
	if v.Ref != nil {
		parts = append(parts, v.Ref.String())
	}
	if v.Object != nil {
		parts = append(parts, v.Object.String())
	}
	s := strings.Join(parts, " & ")
	if v.Default != nil {
		if s != "" {
			s += " | "
		}
		s += "default " + v.Default.Ref.String()
	}
	return s
}

// RefMode distinguishes static dot-references from dynamic bracket-references.
type RefMode int

const (
	StaticRef  RefMode = iota // kind.name
	DynamicRef                // kind[expr]
)

// Reference is a reference to another resource, used as a property value:
//
//	StaticRef:  Kind="sql_cluster", Name="main"        (sql_cluster.main)
//	DynamicRef: Kind="sql_cluster", Expr="tags.domain" (sql_cluster[tags.domain])
//
// A reference must resolve to an instantiated resource; it never creates one.
type Reference struct {
	Mode RefMode
	Kind string // target resource kind, e.g. "sql_cluster"
	Name string // StaticRef: the target name
	Expr string // DynamicRef: dotted attribute path evaluated against the resource

	Pos     Position
	End     Position
	KindPos Position
	KindEnd Position
}

func (r *Reference) String() string {
	if r.Mode == DynamicRef {
		return r.Kind + "[" + r.Expr + "]"
	}
	return r.Kind + "." + r.Name
}

func (r *Reference) span() Span { return Span{Start: r.Pos, End: r.End} }

// ObjectConstraint is a `{ <property>* }` block constraining the resolved
// target of a reference. Defaults are not allowed inside it; every listed
// property must be present on the target resource.
type ObjectConstraint struct {
	Pos   Position // the '{'
	End   Position // the '}'
	Props []*Property
}

func (o *ObjectConstraint) String() string {
	parts := make([]string, len(o.Props))
	for i, p := range o.Props {
		parts[i] = p.String()
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func (o *ObjectConstraint) span() Span { return Span{Start: o.Pos, End: o.End} }

// Default is a `default <value>` clause for a scalar property.
type Default struct {
	Pos      Position // position of the 'default' keyword
	Value    Value
	ValuePos Position
	ValueEnd Position
}

// RefDefault is a `default <reference>` clause for a reference property.
type RefDefault struct {
	Pos Position // position of the 'default' keyword
	Ref *Reference
}

// Constraint is a constraint expression node: a Comparison, an
// AndConstraint, an OrConstraint, or a RequiredConstraint.
type Constraint interface {
	String() string
	span() Span
}

// CompareOp is a comparison operator in a constraint.
type CompareOp int

const (
	OpEq CompareOp = iota
	OpNeq
	OpGe
	OpLe
	OpGt
	OpLt
)

func (op CompareOp) String() string {
	switch op {
	case OpEq:
		return "=="
	case OpNeq:
		return "!="
	case OpGe:
		return ">="
	case OpLe:
		return "<="
	case OpGt:
		return ">"
	case OpLt:
		return "<"
	default:
		return "<invalid op>"
	}
}

// eval reports whether `v op b` holds, where cmp is the ordering of v
// relative to b (-1, 0, or 1).
func (op CompareOp) holds(cmp int) bool {
	switch op {
	case OpEq:
		return cmp == 0
	case OpNeq:
		return cmp != 0
	case OpGe:
		return cmp >= 0
	case OpLe:
		return cmp <= 0
	case OpGt:
		return cmp > 0
	case OpLt:
		return cmp < 0
	}
	return false
}

// Comparison is a single comparison constraint, e.g. `>= 1` or a bare
// exact value like `false` (in which case Implicit is true and Op is OpEq).
type Comparison struct {
	Pos      Position
	End      Position
	Op       CompareOp
	Value    Value
	Implicit bool // written as a bare value, without an operator
}

func (c *Comparison) String() string {
	if c.Implicit {
		return c.Value.String()
	}
	return c.Op.String() + " " + c.Value.String()
}

func (c *Comparison) span() Span { return Span{Start: c.Pos, End: c.End} }

// AndConstraint is a conjunction of constraints joined by '&'.
type AndConstraint struct {
	Terms []Constraint
}

func (c *AndConstraint) String() string {
	parts := make([]string, len(c.Terms))
	for i, t := range c.Terms {
		parts[i] = t.String()
	}
	return strings.Join(parts, " & ")
}

func (c *AndConstraint) span() Span {
	return Span{Start: c.Terms[0].span().Start, End: c.Terms[len(c.Terms)-1].span().End}
}

// OrConstraint is a disjunction of constraints joined by '|'.
type OrConstraint struct {
	Alts []Constraint
}

func (c *OrConstraint) String() string {
	parts := make([]string, len(c.Alts))
	for i, t := range c.Alts {
		parts[i] = t.String()
	}
	return strings.Join(parts, " | ")
}

func (c *OrConstraint) span() Span {
	return Span{Start: c.Alts[0].span().Start, End: c.Alts[len(c.Alts)-1].span().End}
}

// RequiredConstraint is the `required` constraint: the final resolved
// configuration must contain the property.
type RequiredConstraint struct {
	Pos Position
	End Position
}

func (c *RequiredConstraint) String() string { return "required" }
func (c *RequiredConstraint) span() Span     { return Span{Start: c.Pos, End: c.End} }

// walkConstraint calls fn for every node in the constraint tree.
func walkConstraint(c Constraint, fn func(Constraint)) {
	if c == nil {
		return
	}
	fn(c)
	switch t := c.(type) {
	case *AndConstraint:
		for _, term := range t.Terms {
			walkConstraint(term, fn)
		}
	case *OrConstraint:
		for _, alt := range t.Alts {
			walkConstraint(alt, fn)
		}
	}
}
