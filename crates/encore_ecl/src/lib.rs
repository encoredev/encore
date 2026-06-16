//! The Encore Configuration Language (ECL): a small declarative language for
//! defining constraints and defaults over infrastructure resources.
//!
//! This crate is a faithful Rust port of the Go `pkg/ecl` package. The module
//! layout mirrors the Go files one-to-one (`lexer`, `parser`, `value`, `eval`,
//! `env`, ...). A few shapes are translated to idiomatic Rust:
//!
//! - Go interfaces become enums: `PropertyValue` (`ScalarValue` | `RefValue`)
//!   and `Constraint` (`Comparison` | `And` | `Or` | `Required`).
//! - Rules and properties are reference-counted (`Rc<Rule>`, `Rc<Property>`) so
//!   that pointer identity — which the evaluator and validator rely on, and
//!   which synthesized rules from dynamic blocks also need — is preserved
//!   without an id allocator. Identity maps key on the `Rc` address.
//! - `io/fs.FS` becomes the [`FileSystem`] trait, used by [`load`].
//! - Go's `Result` evaluation type is named [`EvalResult`] here to avoid
//!   colliding with [`std::result::Result`]; the `default` AST clause type is
//!   named [`ScalarDefault`] to avoid colliding with [`std::default::Default`].
//!
//! An ECL file contains resource blocks. A named block configures one resource
//! of a kind, optionally scoped by a selector:
//!
//! ```text
//! service "api" if env.type == "production" {
//!     cpu: >= 1 & <= 4 | default 2
//!     memory: >= 1Gi & <= 8Gi | default 1Gi
//! }
//! ```
//!
//! A `for` block configures all resources of a kind matching the selector:
//!
//! ```text
//! for service if env.type == "production" {
//!     cpu: >= 1 & <= 4 | default 1
//! }
//! ```
//!
//! For a given resource, all matching blocks apply: constraints merge by
//! intersection, while defaults resolve to the most specific matching block.
//! Exact value constraints (e.g. `public_access: false`) also act as defaults
//! when the property is unset.
//!
//! Parse one or more files with [`parse_file`] (or [`load`] to follow imports),
//! combine them into a [`RuleSet`], optionally run static analysis with
//! [`RuleSet::validate`], and evaluate resources with [`RuleSet::evaluate`].
//!
//! All errors returned by this crate are of type [`ErrorList`], a list of
//! [`Diagnostic`] values carrying precise source positions, source snippets,
//! related notes, and remediation hints.

mod analyze;
mod ast;
mod diagnostic;
mod env;
mod eval;
mod lexer;
mod load;
mod parser;
mod position;
mod proto;
mod satisfy;
mod specificity;
mod token;
mod util;
mod value;

/// The `encore.ecl.v1` protobuf types for the ECL evaluation-output wire schema
/// (generated from `proto/encore/ecl/v1/ecl.proto`). Build a value with
/// [`RuleSet::to_proto`](crate::RuleSet::to_proto).
pub mod pb {
    include!(concat!(env!("OUT_DIR"), "/encore.ecl.v1.rs"));
}

#[cfg(test)]
mod testutil;

pub use ast::{
    CompareOp, Comparison, CondOp, Condition, Constraint, File, Import, ObjectConstraint, Property,
    PropertyValue, RefDefault, RefMode, RefValue, Reference, Rule, ScalarDefault, ScalarValue,
    Version,
};
pub use diagnostic::{Diagnostic, ErrorList, RelatedInfo};
pub use env::{Definition, EnvResult};
pub use eval::{
    default_schema, EvalResult, Kind, ResolvedProperty, ResolvedRef, ResolvedRefValue, Resource,
    RuleSet, ValueSource,
};
pub use load::{load, DiskFs, FileSystem, MapFs};
pub use parser::{parse_file, ParseResult};
pub use position::{Position, Span};
pub use value::{
    boolean, duration, must_parse_quantity, number, parse_quantity, size, string, Value,
    ValueError, ValueKind,
};
