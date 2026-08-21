//! Shared test helpers (ported from `helpers_test.go`).

use std::collections::HashMap;
use std::rc::Rc;

use crate::diagnostic::ErrorList;
use crate::eval::{EvalResult, Resource, RuleSet};
use crate::lexer::Lexer;
use crate::parser::parse_file;
use crate::position::SourceFile;
use crate::token::Token;
use crate::value::{string, values_equal, Value};

/// Parses `src` as a single file and asserts it has no errors.
pub(crate) fn parse_set(src: &str) -> RuleSet {
    let pr = parse_file("policy.encore", src);
    assert!(
        pr.errors.is_empty(),
        "unexpected parse errors:\n{}",
        pr.errors
    );
    RuleSet::new(vec![pr.file])
}

/// Builds an attribute map of string values from key/value pairs.
pub(crate) fn str_attrs(pairs: &[(&str, &str)]) -> HashMap<String, Value> {
    pairs
        .iter()
        .map(|(k, v)| (k.to_string(), string(*v)))
        .collect()
}

/// Asserts that the error's message contains every given substring.
pub(crate) fn assert_err_contains(err: &ErrorList, subs: &[&str]) {
    let s = err.to_string();
    for sub in subs {
        assert!(s.contains(sub), "error:\n{s}\ndoes not contain {sub:?}");
    }
}

pub(crate) fn eval_ok(rs: &RuleSet, res: &Resource) -> EvalResult {
    match rs.evaluate(res) {
        Ok(r) => r,
        Err(e) => panic!("unexpected eval error:\n{e}"),
    }
}

pub(crate) fn eval_err(rs: &RuleSet, res: &Resource) -> ErrorList {
    match rs.evaluate(res) {
        Ok(_) => panic!("expected eval error, got ok"),
        Err(e) => e,
    }
}

/// Asserts that `got` equals `want`, ignoring display-only attributes.
pub(crate) fn assert_value(got: &Value, want: &Value) {
    assert_eq!(got.kind, want.kind);
    assert!(values_equal(got, want), "got {got}, want {want}");
}

pub(crate) fn lex_all(src: &str) -> (Vec<Token>, ErrorList) {
    let mut diags = ErrorList::new();
    let toks = {
        let sf = Rc::new(SourceFile::new("test.encore", src));
        let mut lx = Lexer::new(sf, &mut diags);
        lx.lex()
    };
    (toks, diags)
}
