use std::fmt;
use std::rc::Rc;

use crate::position::{Position, SourceFile, Span};
use crate::value::{go_quote, Value};

/// A parsed ECL source file.
pub struct File {
    pub path: String,
    /// `None` if no version declaration
    pub version: Option<Version>,
    /// import declarations, in source order
    pub imports: Vec<Import>,
    /// rules, in source order
    pub rules: Vec<Rc<Rule>>,

    pub(crate) src: Rc<SourceFile>,
}

/// A `version N` declaration.
pub struct Version {
    pub pos: Position,
    pub num: i64,
}

/// An `import "path"` declaration.
#[derive(Clone)]
pub struct Import {
    /// position of the `import` keyword
    pub pos: Position,
    pub path: String,
    /// position of the path string
    pub path_pos: Position,
    pub path_end: Position,
}

/// A resource block. It takes one of three header forms:
///
/// ```text
/// for <kind> [if <selector>] { ... }       // name == "" && dyn_expr == ""
/// <kind> "<name>" [if <selector>] { ... }  // name != ""
/// <kind> <expr> [if <selector>] { ... }    // dyn_expr != ""
/// ```
pub struct Rule {
    /// position of the `for` keyword or the kind identifier
    pub pos: Position,
    /// resource kind, e.g. "service"
    pub kind: String,
    pub kind_pos: Position,
    pub kind_end: Position,

    /// static named block; "" otherwise
    pub name: String,
    pub name_pos: Position,

    /// attribute path of a dynamic block (`kind <expr> { ... }`),
    /// e.g. "tags.domain"; "" for non-dynamic blocks.
    pub dyn_expr: String,
    pub dyn_expr_pos: Position,
    pub dyn_expr_end: Position,

    /// effective selector conditions joined by `&&`; empty if none
    pub wheres: Vec<Condition>,
    pub props: Vec<Rc<Property>>,
    /// resource blocks nested in this rule's body (dynamic instantiation)
    pub blocks: Vec<Rc<Rule>>,

    /// source file for snippet rendering; `None` for nested blocks (mirrors
    /// the Go parser, which only attaches the file to top-level rules)
    pub(crate) src: Option<Rc<SourceFile>>,
}

impl Rule {
    /// Reconstructs the rule's effective header as source text, e.g.
    /// `for service "api" if env.type == "production"`.
    pub fn header(&self) -> String {
        let mut b = String::new();
        if self.name.is_empty() && self.dyn_expr.is_empty() {
            b.push_str("for ");
        }
        b.push_str(&self.kind);
        if !self.name.is_empty() {
            b.push(' ');
            b.push_str(&go_quote(&self.name));
        } else if !self.dyn_expr.is_empty() {
            b.push(' ');
            b.push_str(&self.dyn_expr);
        }
        if !self.wheres.is_empty() {
            b.push_str(" if ");
            for (i, c) in self.wheres.iter().enumerate() {
                if i > 0 {
                    b.push_str(" && ");
                }
                b.push_str(&c.to_string());
            }
        }
        b
    }
}

/// A selector condition operator.
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum CondOp {
    Eq,
    Neq,
    In,
    Exists,
}

/// A single selector condition, e.g. `env.type == production`.
#[derive(Clone)]
pub struct Condition {
    /// start of the field path
    pub pos: Position,
    /// just past the condition
    pub end: Position,
    /// dotted field path, e.g. "env.type"
    pub field: String,
    pub field_end: Position,
    pub op: CondOp,
    /// one value for ==/!=, one or more for in, none for exists
    pub values: Vec<Value>,

    /// Set for conditions that originate from an `if` block. Such conditions
    /// are evaluated in the top-level environment scope and may only reference
    /// declared environment attributes (see `RuleSet::env_scope`); a `where`
    /// clause on a for/named rule is resource-scoped and leaves this false.
    pub env_scoped: bool,
}

impl fmt::Display for Condition {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.op {
            CondOp::Eq => write!(f, "{} == {}", self.field, self.values[0]),
            CondOp::Neq => write!(f, "{} != {}", self.field, self.values[0]),
            CondOp::Exists => write!(f, "{} exists", self.field),
            CondOp::In => {
                write!(f, "{} in [", self.field)?;
                for (i, v) in self.values.iter().enumerate() {
                    if i > 0 {
                        write!(f, ", ")?;
                    }
                    write!(f, "{v}")?;
                }
                write!(f, "]")
            }
        }
    }
}

/// A single property rule inside a rule or object-constraint body, e.g.
/// `cpu: >= 1 & <= 4 | default 2` or `cluster: sql_cluster.main`.
pub struct Property {
    /// start of the property path
    pub pos: Position,
    pub path_end: Position,
    /// dotted property path, e.g. "instances.min"
    pub path: String,
    /// the right-hand side: a scalar constraint or a reference constraint
    pub value: PropertyValue,
}

impl Property {
    /// Returns the `ScalarValue` if the property is scalar-valued, else `None`.
    pub fn scalar(&self) -> Option<&ScalarValue> {
        match &self.value {
            PropertyValue::Scalar(s) => Some(s),
            _ => None,
        }
    }

    /// Returns the `RefValue` if the property is reference-valued, else `None`.
    pub fn ref_value(&self) -> Option<&RefValue> {
        match &self.value {
            PropertyValue::Ref(r) => Some(r),
            _ => None,
        }
    }
}

impl fmt::Display for Property {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.path, self.value)
    }
}

/// The right-hand side of a property rule.
#[allow(clippy::large_enum_variant)]
pub enum PropertyValue {
    Scalar(ScalarValue),
    Ref(RefValue),
}

impl fmt::Display for PropertyValue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            PropertyValue::Scalar(s) => s.fmt(f),
            PropertyValue::Ref(r) => r.fmt(f),
        }
    }
}

/// Constrains a scalar property with an optional default, e.g.
/// `>= 1 & <= 4 | default 2`.
pub struct ScalarValue {
    /// `None` if default-only
    pub constraint: Option<Constraint>,
    /// `None` if no default clause
    pub default: Option<ScalarDefault>,
}

impl fmt::Display for ScalarValue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if let Some(c) = &self.constraint {
            write!(f, "{c}")?;
            if self.default.is_some() {
                write!(f, " | ")?;
            }
        }
        if let Some(d) = &self.default {
            write!(f, "default {}", d.value)?;
        }
        Ok(())
    }
}

/// Constrains a reference-valued property: an identity reference and/or nested
/// object constraints on the resolved target, with an optional reference
/// default.
pub struct RefValue {
    /// identity; `None` if object-only or default-only
    pub reference: Option<Reference>,
    /// nested constraints on the target; `None` if none
    pub object: Option<ObjectConstraint>,
    /// `default <ref>`; `None` if no default clause
    pub default: Option<RefDefault>,
}

impl fmt::Display for RefValue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let mut parts: Vec<String> = Vec::new();
        if let Some(r) = &self.reference {
            parts.push(r.to_string());
        }
        if let Some(o) = &self.object {
            parts.push(o.to_string());
        }
        let mut s = parts.join(" & ");
        if let Some(d) = &self.default {
            if !s.is_empty() {
                s.push_str(" | ");
            }
            s.push_str(&format!("default {}", d.reference));
        }
        f.write_str(&s)
    }
}

/// Distinguishes static dot-references from dynamic bracket-references.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum RefMode {
    /// kind.name
    Static,
    /// kind[expr]
    Dynamic,
}

/// A reference to another resource, used as a property value.
#[derive(Clone)]
pub struct Reference {
    pub mode: RefMode,
    /// target resource kind, e.g. "sql_cluster"
    pub kind: String,
    /// `Static`: the target name
    pub name: String,
    /// `Dynamic`: dotted attribute path evaluated against the resource
    pub expr: String,

    pub pos: Position,
    pub end: Position,
    pub kind_pos: Position,
    pub kind_end: Position,
}

impl Reference {
    pub(crate) fn span(&self) -> Span {
        Span {
            start: self.pos.clone(),
            end: self.end.clone(),
        }
    }
}

impl fmt::Display for Reference {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.mode {
            RefMode::Dynamic => write!(f, "{}[{}]", self.kind, self.expr),
            RefMode::Static => write!(f, "{}.{}", self.kind, self.name),
        }
    }
}

/// A `{ <property>* }` block constraining the resolved target of a reference.
/// Defaults are not allowed inside it; every listed property must be present on
/// the target resource.
#[derive(Clone)]
pub struct ObjectConstraint {
    /// the `{`
    pub pos: Position,
    /// the `}`
    pub end: Position,
    pub props: Vec<Rc<Property>>,
}

impl ObjectConstraint {
    pub(crate) fn span(&self) -> Span {
        Span {
            start: self.pos.clone(),
            end: self.end.clone(),
        }
    }
}

impl fmt::Display for ObjectConstraint {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let parts: Vec<String> = self.props.iter().map(|p| p.to_string()).collect();
        write!(f, "{{ {} }}", parts.join(", "))
    }
}

/// A `default <value>` clause for a scalar property.
pub struct ScalarDefault {
    /// position of the `default` keyword
    pub pos: Position,
    pub value: Value,
    pub value_pos: Position,
    pub value_end: Position,
}

/// A `default <reference>` clause for a reference property.
pub struct RefDefault {
    /// position of the `default` keyword
    pub pos: Position,
    pub reference: Reference,
}

/// A constraint expression node.
#[derive(Clone)]
pub enum Constraint {
    Comparison(Comparison),
    /// conjunction of constraints joined by `&`
    And(Vec<Constraint>),
    /// disjunction of constraints joined by `|`
    Or(Vec<Constraint>),
    Required(RequiredConstraint),
    /// a reference acting as a (reference-valued) constraint term
    Reference(Reference),
    /// an object constraint acting as a (reference-valued) constraint term
    Object(ObjectConstraint),
}

impl Constraint {
    pub(crate) fn span(&self) -> Span {
        match self {
            Constraint::Comparison(c) => Span {
                start: c.pos.clone(),
                end: c.end.clone(),
            },
            Constraint::Required(c) => Span {
                start: c.pos.clone(),
                end: c.end.clone(),
            },
            Constraint::Reference(r) => r.span(),
            Constraint::Object(o) => o.span(),
            Constraint::And(terms) => Span {
                start: terms[0].span().start,
                end: terms[terms.len() - 1].span().end,
            },
            Constraint::Or(alts) => Span {
                start: alts[0].span().start,
                end: alts[alts.len() - 1].span().end,
            },
        }
    }
}

impl fmt::Display for Constraint {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Constraint::Comparison(c) => c.fmt(f),
            Constraint::Required(c) => c.fmt(f),
            Constraint::Reference(r) => r.fmt(f),
            Constraint::Object(o) => o.fmt(f),
            Constraint::And(terms) => {
                let parts: Vec<String> = terms.iter().map(|t| t.to_string()).collect();
                f.write_str(&parts.join(" & "))
            }
            Constraint::Or(alts) => {
                let parts: Vec<String> = alts.iter().map(|t| t.to_string()).collect();
                f.write_str(&parts.join(" | "))
            }
        }
    }
}

/// A comparison operator in a constraint.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum CompareOp {
    Eq,
    Neq,
    Ge,
    Le,
    Gt,
    Lt,
}

impl CompareOp {
    /// Reports whether `v op b` holds, where `cmp` is the ordering of `v`
    /// relative to `b` (-1, 0, or 1).
    pub(crate) fn holds(self, cmp: i32) -> bool {
        match self {
            CompareOp::Eq => cmp == 0,
            CompareOp::Neq => cmp != 0,
            CompareOp::Ge => cmp >= 0,
            CompareOp::Le => cmp <= 0,
            CompareOp::Gt => cmp > 0,
            CompareOp::Lt => cmp < 0,
        }
    }
}

impl fmt::Display for CompareOp {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let s = match self {
            CompareOp::Eq => "==",
            CompareOp::Neq => "!=",
            CompareOp::Ge => ">=",
            CompareOp::Le => "<=",
            CompareOp::Gt => ">",
            CompareOp::Lt => "<",
        };
        f.write_str(s)
    }
}

/// A single comparison constraint, e.g. `>= 1` or a bare exact value like
/// `false` (in which case `implicit` is true and `op` is `Eq`).
#[derive(Clone)]
pub struct Comparison {
    pub pos: Position,
    pub end: Position,
    pub op: CompareOp,
    pub value: Value,
    /// written as a bare value, without an operator
    pub implicit: bool,
}

impl fmt::Display for Comparison {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.implicit {
            write!(f, "{}", self.value)
        } else {
            write!(f, "{} {}", self.op, self.value)
        }
    }
}

/// The `required` constraint: the final resolved configuration must contain the
/// property.
#[derive(Clone)]
pub struct RequiredConstraint {
    pub pos: Position,
    pub end: Position,
}

impl fmt::Display for RequiredConstraint {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("required")
    }
}

/// Calls `f` for every node in the constraint tree.
pub(crate) fn walk_constraint(c: Option<&Constraint>, f: &mut impl FnMut(&Constraint)) {
    let c = match c {
        Some(c) => c,
        None => return,
    };
    f(c);
    match c {
        Constraint::And(terms) => {
            for t in terms {
                walk_constraint(Some(t), f);
            }
        }
        Constraint::Or(alts) => {
            for a in alts {
                walk_constraint(Some(a), f);
            }
        }
        _ => {}
    }
}

/// Returns the source file of a rule for diagnostics, mirroring Go's `ruleSrc`.
pub(crate) fn rule_src(r: Option<&Rc<Rule>>) -> Option<Rc<SourceFile>> {
    r.and_then(|r| r.src.clone())
}
