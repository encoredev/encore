use std::fmt;
use std::rc::Rc;

use crate::position::{Position, SourceFile};

/// Describes a single problem found while parsing, validating, or evaluating
/// ECL rules.
#[derive(Clone)]
pub struct Diagnostic {
    /// primary location of the problem
    pub pos: Position,
    /// optional end of the offending range (same line as `pos`)
    pub end: Position,
    /// one-line summary
    pub message: String,
    /// optional additional lines providing context
    pub detail: Vec<String>,
    /// optional remediation suggestion
    pub hint: String,
    pub related: Vec<RelatedInfo>,

    /// for snippet rendering; may be `None`
    pub(crate) src: Option<Rc<SourceFile>>,
}

/// Points at a secondary source location relevant to a diagnostic, such as the
/// other rule involved in a conflict.
#[derive(Clone)]
pub struct RelatedInfo {
    pub pos: Position,
    pub message: String,
}

impl Diagnostic {
    pub(crate) fn new(
        src: Option<Rc<SourceFile>>,
        pos: Position,
        end: Position,
        message: impl Into<String>,
    ) -> Diagnostic {
        Diagnostic {
            pos,
            end,
            message: message.into(),
            detail: Vec::new(),
            hint: String::new(),
            related: Vec::new(),
            src,
        }
    }

    /// A compact one-line form: `file:line:col: message`.
    pub fn summary(&self) -> String {
        format!("{}: {}", self.pos, self.message)
    }

    fn render(&self) -> String {
        let mut b = String::new();
        b.push_str(&format!("{}: error: {}\n", self.pos, self.message));
        self.render_snippet(&mut b);
        for line in &self.detail {
            b.push_str(&format!("  {line}\n"));
        }
        for r in &self.related {
            b.push_str(&format!("  note: {}\n", r.message));
        }
        if !self.hint.is_empty() {
            b.push_str(&format!("  help: {}\n", self.hint));
        }
        b.trim_end_matches('\n').to_string()
    }

    #[allow(clippy::explicit_counter_loop)]
    fn render_snippet(&self, b: &mut String) {
        let line = match self.src.as_ref().and_then(|s| s.line(self.pos.line)) {
            Some(l) => l,
            None => return,
        };
        let num = self.pos.line.to_string();
        let gutter = " ".repeat(num.len());
        b.push_str(&format!(" {gutter} |\n"));
        b.push_str(&format!(" {num} | {line}\n"));

        // Build the caret line, mirroring tabs so the caret aligns with the
        // source line above regardless of tab rendering width.
        let mut lead = String::new();
        let mut col = 1usize;
        for r in line.chars() {
            if col >= self.pos.column {
                break;
            }
            if r == '\t' {
                lead.push('\t');
            } else {
                lead.push(' ');
            }
            col += 1;
        }
        let mut width = 1usize;
        if self.end.is_valid()
            && self.end.line == self.pos.line
            && self.end.column > self.pos.column
        {
            width = self.end.column - self.pos.column;
        }
        b.push_str(&format!(" {gutter} | {lead}{}\n", "^".repeat(width)));
    }
}

impl fmt::Display for Diagnostic {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.render())
    }
}

impl fmt::Debug for Diagnostic {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.render())
    }
}

/// A list of diagnostics. All errors returned by this crate are of this type.
#[derive(Clone, Default)]
pub struct ErrorList(pub Vec<Diagnostic>);

impl ErrorList {
    pub(crate) fn new() -> ErrorList {
        ErrorList(Vec::new())
    }

    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    pub fn len(&self) -> usize {
        self.0.len()
    }

    pub(crate) fn push(&mut self, d: Diagnostic) {
        self.0.push(d);
    }

    pub(crate) fn extend(&mut self, other: ErrorList) {
        self.0.extend(other.0);
    }

    /// Appends a diagnostic built from the given position and message,
    /// returning a mutable reference so the caller can set hint/detail/related.
    pub(crate) fn add(
        &mut self,
        src: Option<Rc<SourceFile>>,
        start: Position,
        end: Position,
        message: impl Into<String>,
    ) -> &mut Diagnostic {
        self.0.push(Diagnostic::new(src, start, end, message));
        self.0.last_mut().unwrap()
    }

    pub(crate) fn sort(&mut self) {
        self.0.sort_by(|a, b| {
            a.pos
                .file
                .cmp(&b.pos.file)
                .then(a.pos.line.cmp(&b.pos.line))
                .then(a.pos.column.cmp(&b.pos.column))
                .then(a.message.cmp(&b.message))
        });
    }

    /// Returns `Ok(())` if empty, otherwise `Err(self)`.
    pub(crate) fn into_result(self) -> Result<(), ErrorList> {
        if self.0.is_empty() {
            Ok(())
        } else {
            Err(self)
        }
    }
}

impl fmt::Display for ErrorList {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let parts: Vec<String> = self.0.iter().map(|d| d.to_string()).collect();
        f.write_str(&parts.join("\n\n"))
    }
}

impl fmt::Debug for ErrorList {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Display::fmt(self, f)
    }
}

impl std::error::Error for ErrorList {}

#[cfg(test)]
mod tests {
    use crate::eval::Resource;
    use crate::parser::parse_file;
    use crate::testutil::{parse_set, str_attrs};

    #[test]
    fn render_parse_error() {
        let pr = parse_file(
            "policy.encore",
            "for service if env.type = \"production\" {\n}\n",
        );
        assert!(!pr.errors.is_empty());
        assert_eq!(
            pr.errors.to_string(),
            "policy.encore:1:25: error: '=' is not an operator\n   |\n 1 | for service if env.type = \"production\" {\n   |                         ^\n  help: use '==' for equality comparisons"
        );
    }

    #[test]
    fn render_caret_width() {
        let pr = parse_file("policy.encore", "for service {\n    memory: >= 512G\n}\n");
        assert_eq!(
            pr.errors.to_string(),
            "policy.encore:2:16: error: unknown unit 'G' in '512G'\n   |\n 2 |     memory: >= 512G\n   |                ^^^^\n  help: did you mean '512GB'? valid units are B, KB, Ki, MB, Mi, GB, Gi, TB, Ti (size) and ms, s, m, h, d (duration)"
        );
    }

    #[test]
    fn render_ambiguous_default() {
        let rs = parse_set(
            "for service if env.type == \"production\" {\n    cpu: default 1\n}\nfor service if team == \"payments\" {\n    cpu: default 2\n}\n",
        );
        let err = match rs.evaluate(&Resource {
            kind: "service".into(),
            name: "api".into(),
            attrs: str_attrs(&[("env.type", "production"), ("team", "payments")]),
            ..Default::default()
        }) {
            Err(e) => e,
            Ok(_) => panic!("expected error"),
        };
        assert_eq!(
            err.to_string(),
            "policy.encore:2:18: error: ambiguous default for property 'cpu' of service \"api\"\n   |\n 2 |     cpu: default 1\n   |                  ^\n  matching rules provide different defaults:\n    policy.encore:1:1: for service if env.type == \"production\"\n        cpu: default 1\n    policy.encore:4:1: for service if team == \"payments\"\n        cpu: default 2\n  no rule is more specific than all the others\n  help: add a more specific rule that decides the default, e.g.:\n    for service if env.type == \"production\" && team == \"payments\" {\n        cpu: default 2\n    }"
        );
    }

    #[test]
    fn render_default_violation() {
        let rs = parse_set("for service {\n    cpu: <= 4\n}\nservice \"api\" {\n    cpu: 8\n}\n");
        let err = match rs.evaluate(&Resource {
            kind: "service".into(),
            name: "api".into(),
            ..Default::default()
        }) {
            Err(e) => e,
            Ok(_) => panic!("expected error"),
        };
        assert_eq!(
            err.to_string(),
            "policy.encore:2:10: error: service \"api\": default value 8 for property 'cpu' violates constraint '<= 4'\n   |\n 2 |     cpu: <= 4\n   |          ^^^^\n  the constraint is defined at policy.encore:1:1 in rule:\n    for service\n        cpu: <= 4\n  note: the default is defined at policy.encore:5:10 in rule: service \"api\""
        );
    }

    #[test]
    fn render_multiple_errors() {
        let pr = parse_file(
            "policy.encore",
            "for service {\n    cpu = 1\n    mem = 2\n}\n",
        );
        assert_eq!(pr.errors.len(), 2);
        assert_eq!(
            pr.errors.0[0].summary(),
            "policy.encore:2:9: property rules use ':', not '='"
        );
        assert_eq!(
            pr.errors.0[1].summary(),
            "policy.encore:3:9: property rules use ':', not '='"
        );
        assert!(pr.errors.to_string().contains("\n\n"));
    }

    #[test]
    fn tab_alignment() {
        let pr = parse_file("policy.encore", "for service {\n\tcpu = 1\n}\n");
        assert!(pr.errors.0[0]
            .to_string()
            .contains(" 2 | \tcpu = 1\n   | \t    ^"));
    }
}
