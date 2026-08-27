use std::rc::Rc;

use crate::ast::{
    CompareOp, Comparison, CondOp, Condition, Constraint, File, Import, ObjectConstraint, Property,
    PropertyValue, RefDefault, RefMode, RefValue, Reference, RequiredConstraint, Rule,
    ScalarDefault, ScalarValue, Version,
};
use crate::diagnostic::{Diagnostic, ErrorList};
use crate::lexer::Lexer;
use crate::position::{Position, SourceFile, Span};
use crate::token::{Token, TokenKind};
use crate::value::{boolean, format_float, number, ValueKind};

const MAX_PARSE_ERRORS: usize = 20;

/// The outcome of parsing a single file: the (possibly partial) AST and any
/// errors found.
pub struct ParseResult {
    pub file: File,
    pub errors: ErrorList,
}

impl ParseResult {
    /// Returns the parsed file if there were no errors, otherwise the errors.
    pub fn into_result(self) -> Result<File, ErrorList> {
        if self.errors.is_empty() {
            Ok(self.file)
        } else {
            Err(self.errors)
        }
    }
}

/// Parses a single ECL source file. The filename is used in positions and
/// diagnostics only; no file I/O is performed.
pub fn parse_file(filename: impl Into<String>, src: impl AsRef<[u8]>) -> ParseResult {
    let filename = filename.into();
    let src_str = String::from_utf8_lossy(src.as_ref()).into_owned();
    let sf = Rc::new(SourceFile::new(filename.clone(), src_str));

    let mut diags = ErrorList::new();
    let toks = {
        let mut lx = Lexer::new(sf.clone(), &mut diags);
        lx.lex()
    };

    let (mut rules, version, imports) = {
        let mut p = Parser {
            src: sf.clone(),
            toks,
            i: 0,
            diags: &mut diags,
            rules: Vec::new(),
            version: None,
            imports: Vec::new(),
        };
        p.parse_file();
        (p.rules, p.version, p.imports)
    };

    // The Go parser attaches the file (for snippet rendering) only to top-level
    // rules, not nested blocks.
    for r in &mut rules {
        r.src = Some(sf.clone());
    }
    let rules: Vec<Rc<Rule>> = rules.into_iter().map(Rc::new).collect();

    let file = File {
        path: filename,
        version,
        imports,
        rules,
        src: sf,
    };
    diags.sort();
    ParseResult {
        file,
        errors: diags,
    }
}

struct Parser<'a> {
    src: Rc<SourceFile>,
    toks: Vec<Token>,
    i: usize,
    diags: &'a mut ErrorList,
    rules: Vec<Rule>,
    version: Option<Version>,
    imports: Vec<Import>,
}

fn blank_rule(pos: Position) -> Rule {
    Rule {
        pos,
        kind: String::new(),
        kind_pos: Position::default(),
        kind_end: Position::default(),
        name: String::new(),
        name_pos: Position::default(),
        dyn_expr: String::new(),
        dyn_expr_pos: Position::default(),
        dyn_expr_end: Position::default(),
        wheres: Vec::new(),
        props: Vec::new(),
        blocks: Vec::new(),
        src: None,
    }
}

impl<'a> Parser<'a> {
    // --- token helpers ---

    fn cur(&self) -> &Token {
        &self.toks[self.i]
    }

    fn at(&self, k: TokenKind) -> bool {
        self.cur().kind == k
    }

    fn advance(&mut self) -> Token {
        let t = self.toks[self.i].clone();
        if t.kind != TokenKind::Eof {
            self.i += 1;
        }
        t
    }

    fn accept(&mut self, k: TokenKind) -> bool {
        if self.at(k) {
            self.advance();
            true
        } else {
            false
        }
    }

    /// Consumes a token of the given kind, or reports an error and returns
    /// `None`.
    fn expect(&mut self, k: TokenKind, context: &str) -> Option<Token> {
        if self.at(k) {
            return Some(self.advance());
        }
        let t = self.cur().clone();
        self.error_at_token(
            &t,
            format!("expected {} {}, found {}", k, context, t.describe()),
        );
        None
    }

    fn skip_newlines(&mut self) {
        while self.at(TokenKind::Newline) {
            self.advance();
        }
    }

    /// Requires the current statement to end here: a newline (consumed), or
    /// `}` / end of file (left in place).
    fn expect_terminator(&mut self, context: &str) -> bool {
        match self.cur().kind {
            TokenKind::Newline => {
                self.advance();
                true
            }
            TokenKind::RBrace | TokenKind::Eof => true,
            _ => {
                let t = self.cur().clone();
                self.error_at_token(
                    &t,
                    format!("expected newline after {}, found {}", context, t.describe()),
                );
                false
            }
        }
    }

    fn too_many_errors(&mut self) -> bool {
        if self.diags.len() < MAX_PARSE_ERRORS {
            return false;
        }
        let last_is_too_many = self
            .diags
            .0
            .last()
            .map(|d| d.message.starts_with("too many errors"))
            .unwrap_or(false);
        if !last_is_too_many {
            let pos = self.cur().pos.clone();
            let end = self.cur().end.clone();
            self.error_at(pos, end, "too many errors; stopping".to_string());
        }
        true
    }

    // --- error helpers ---

    fn error_at(&mut self, start: Position, end: Position, message: String) -> &mut Diagnostic {
        let src = Some(self.src.clone());
        self.diags.add(src, start, end, message)
    }

    fn error_at_token(&mut self, t: &Token, message: String) -> &mut Diagnostic {
        self.error_at(t.pos.clone(), t.end.clone(), message)
    }

    /// Skips tokens until just past the next newline, or until `}` or end of
    /// file (left in place).
    fn sync_line(&mut self) {
        loop {
            match self.cur().kind {
                TokenKind::Newline => {
                    self.advance();
                    return;
                }
                TokenKind::RBrace | TokenKind::Eof => return,
                _ => {
                    self.advance();
                }
            }
        }
    }

    /// Skips tokens until the next declaration keyword, a `}`, or end of file.
    fn sync_top_level(&mut self) {
        loop {
            match self.cur().kind {
                TokenKind::For
                | TokenKind::Import
                | TokenKind::Where
                | TokenKind::Ident
                | TokenKind::RBrace
                | TokenKind::Eof => return,
                _ => {
                    self.advance();
                }
            }
        }
    }

    /// Recovers from an unexpected token in declaration position: consumes at
    /// least one token, then skips to the next declaration start, `}`, or EOF.
    fn sync_decl(&mut self) {
        self.advance();
        loop {
            match self.cur().kind {
                TokenKind::For
                | TokenKind::Where
                | TokenKind::Import
                | TokenKind::Ident
                | TokenKind::RBrace
                | TokenKind::Eof => return,
                _ => {
                    self.advance();
                }
            }
        }
    }

    // --- grammar ---

    fn parse_file(&mut self) {
        self.skip_newlines();
        if self.at_version_decl() {
            self.parse_version();
            self.skip_newlines();
        }
        while self.at(TokenKind::Import) {
            self.parse_import();
            self.skip_newlines();
        }
        while !self.at(TokenKind::Eof) && !self.too_many_errors() {
            self.parse_decl(&[], true);
            self.skip_newlines();
        }
    }

    fn parse_decl(&mut self, outer: &[Condition], top_level: bool) {
        if self.at_version_decl() {
            let t = self.cur().clone();
            let d = self.error_at_token(
                &t,
                "the version declaration must be the first statement in the file".to_string(),
            );
            d.hint = "move 'version' to the top of the file".to_string();
            self.sync_line();
            return;
        }
        match self.cur().kind {
            TokenKind::For => {
                if let Some(r) = self.parse_for_rule(outer) {
                    self.rules.push(r);
                }
            }
            TokenKind::Ident => {
                if let Some(r) = self.parse_resource_block(outer) {
                    self.rules.push(r);
                }
            }
            TokenKind::If => self.parse_if_block(outer),
            TokenKind::Where => {
                // Migration aid: the standalone `if` block is now `if`.
                let t = self.cur().clone();
                let d = self.error_at_token(
                    &t,
                    "'where' blocks are now written as 'if' blocks".to_string(),
                );
                d.hint =
                    r#"scope rules to an environment with 'if', e.g.: if env.type == "production" { ... }"#
                        .to_string();
                self.parse_if_block(outer); // recover by parsing it as an if block
            }
            TokenKind::Import => {
                let t = self.cur().clone();
                self.error_at_token(
                    &t,
                    "import declarations must appear before the first rule".to_string(),
                );
                self.parse_import();
            }
            _ => {
                let t = self.cur().clone();
                if top_level {
                    self.error_at_token(
                        &t,
                        format!(
                            "expected 'for', a resource block, or 'if' to begin a declaration, found {}",
                            t.describe()
                        ),
                    );
                } else {
                    self.error_at_token(
                        &t,
                        format!(
                            "expected 'for', a resource block, or a nested 'if' block, found {}",
                            t.describe()
                        ),
                    );
                }
                self.sync_decl();
            }
        }
    }

    /// Parses `if <condition> { <decl>* }` and desugars it: the block's
    /// conditions are prepended to every rule inside, composing via `&&`. The
    /// conditions are marked env-scoped, since an `if` block is evaluated in the
    /// top-level environment scope; `validate` checks they reference only
    /// declared environment attributes.
    fn parse_if_block(&mut self, outer: &[Condition]) {
        self.advance(); // 'if' (or 'where', when recovering)
        let mut conds = if self.at(TokenKind::LBrace) {
            let t = self.cur().clone();
            let d = self.error_at_token(&t, "expected a condition after 'if'".to_string());
            d.hint = r#"e.g.: if env.type == "production" { ... }"#.to_string();
            Vec::new()
        } else {
            self.parse_selector()
        };
        for c in &mut conds {
            c.env_scoped = true;
        }
        let combined = combine_conds(outer, conds);

        if self
            .expect(TokenKind::LBrace, "to begin the if block")
            .is_none()
        {
            self.sync_top_level();
            return;
        }
        self.skip_newlines();
        while !self.at(TokenKind::RBrace) && !self.at(TokenKind::Eof) && !self.too_many_errors() {
            self.parse_decl(&combined, false);
            self.skip_newlines();
        }
        self.expect(TokenKind::RBrace, "to close the if block");
        self.expect_terminator("the if block");
    }

    fn parse_for_rule(&mut self, outer: &[Condition]) -> Option<Rule> {
        let kw = self.advance(); // 'for'
        let mut rule = blank_rule(kw.pos.clone());

        let kind_tok = self.cur().clone();
        match kind_tok.kind {
            TokenKind::Ident => {
                self.advance();
                rule.kind = kind_tok.str.clone();
                rule.kind_pos = kind_tok.pos.clone();
                rule.kind_end = kind_tok.end.clone();
            }
            TokenKind::String => {
                let d = self.error_at_token(
                    &kind_tok,
                    format!(
                        "expected a resource kind after 'for', found {}",
                        kind_tok.describe()
                    ),
                );
                d.hint = format!(
                    "the resource kind comes before the name: {} {{ ... }} configures all of a kind",
                    kind_tok.text
                );
                self.sync_top_level();
                return None;
            }
            _ => {
                let d = self.error_at_token(
                    &kind_tok,
                    format!(
                        "expected a resource kind after 'for', found {}",
                        kind_tok.describe()
                    ),
                );
                d.hint = "e.g.: for service { ... }".to_string();
                self.sync_top_level();
                return None;
            }
        }

        // `for` blocks take no name or expression — only a selector.
        match self.cur().kind {
            TokenKind::String => {
                let t = self.cur().clone();
                let d = self.error_at_token(&t, "a named block omits 'for'".to_string());
                d.hint = format!(
                    "write: {} {} {{ ... }} to configure one resource",
                    rule.kind, t.text
                );
                self.sync_top_level();
                return None;
            }
            TokenKind::Ident => {
                let t = self.cur().clone();
                let d = self.error_at_token(&t, "a dynamic block omits 'for'".to_string());
                d.hint = format!("write: {} <expr> {{ ... }}", rule.kind);
                self.sync_top_level();
                return None;
            }
            _ => {}
        }

        self.parse_rule_header_tail(&mut rule, outer);
        Some(rule)
    }

    fn parse_resource_block(&mut self, outer: &[Condition]) -> Option<Rule> {
        let kind_tok = self.advance(); // kind identifier
        let mut rule = blank_rule(kind_tok.pos.clone());
        rule.kind = kind_tok.str.clone();
        rule.kind_pos = kind_tok.pos.clone();
        rule.kind_end = kind_tok.end.clone();

        // Migration aid: 'define' is no longer a keyword.
        if kind_tok.str == "define" && (self.at(TokenKind::Ident) || self.at(TokenKind::String)) {
            let d = self.error_at_token(
                &kind_tok,
                "the 'define' keyword has been removed".to_string(),
            );
            d.hint = "declare managed resources directly, e.g.: sql_cluster \"main\" { ... }"
                .to_string();
            self.sync_top_level();
            return None;
        }

        match self.cur().kind {
            TokenKind::String => {
                let t = self.advance();
                rule.name = t.str.clone();
                rule.name_pos = t.pos.clone();
                if t.str.is_empty() {
                    self.error_at_token(&t, "resource name must not be empty".to_string());
                }
            }
            TokenKind::Ident => {
                let (expr, span) = match self.parse_field_path("dynamic block") {
                    Some(x) => x,
                    None => {
                        self.sync_top_level();
                        return Some(rule);
                    }
                };
                rule.dyn_expr = expr;
                rule.dyn_expr_pos = span.start;
                rule.dyn_expr_end = span.end;
            }
            TokenKind::Where => {
                let t = self.cur().clone();
                let d = self.error_at_token(
                    &t,
                    format!(
                        "to match resources of kind '{}' by selector, use 'for'",
                        rule.kind
                    ),
                );
                d.hint = format!("write: for {} if ... {{ ... }}", rule.kind);
                self.sync_top_level();
                return Some(rule);
            }
            TokenKind::LBrace => {
                let t = self.cur().clone();
                let d = self.error_at_token(
                    &t,
                    "a resource block needs a name or expression".to_string(),
                );
                d.hint = format!(
                    "write '{} \"name\" {{ ... }}' for one resource, or 'for {} {{ ... }}' for all",
                    rule.kind, rule.kind
                );
                self.sync_top_level();
                return Some(rule);
            }
            TokenKind::Colon | TokenKind::Assign => {
                let t = self.cur().clone();
                let d = self.error_at_token(
                    &t,
                    "property rules must appear inside a rule body, not at this level".to_string(),
                );
                d.hint = format!(
                    "wrap it in a block, e.g.: for {} {{ {}: ... }}",
                    rule.kind, rule.kind
                );
                self.sync_line();
                return None;
            }
            _ => {
                let t = self.cur().clone();
                self.error_at_token(
                    &t,
                    format!(
                        "expected a resource name or expression after '{}', found {}",
                        rule.kind,
                        t.describe()
                    ),
                );
                self.sync_top_level();
                return Some(rule);
            }
        }

        self.parse_rule_header_tail(&mut rule, outer);
        Some(rule)
    }

    fn parse_rule_header_tail(&mut self, rule: &mut Rule, outer: &[Condition]) {
        let own = if self.accept(TokenKind::If) {
            self.parse_selector()
        } else if self.at(TokenKind::Where) {
            // Migration aid: rule conditions now use 'if', not 'where'.
            let t = self.cur().clone();
            let d = self.error_at_token(
                &t,
                "conditions on a rule now use 'if', not 'where'".to_string(),
            );
            d.hint = r#"e.g.: for service if env.type == "production" { ... }"#.to_string();
            self.advance();
            self.parse_selector()
        } else {
            Vec::new()
        };
        let own_len = own.len();
        rule.wheres = combine_conds(outer, own);

        if !self.at(TokenKind::LBrace) {
            let t = self.cur().clone();
            let d = self.error_at_token(
                &t,
                format!(
                    "expected '{{' to begin the rule body, found {}",
                    t.describe()
                ),
            );
            if t.kind == TokenKind::Ident && own_len > 0 {
                d.hint = "selector conditions are combined with '&&'".to_string();
            }
            self.sync_top_level();
            return;
        }
        self.parse_rule_body(rule);
    }

    fn parse_rule_body(&mut self, rule: &mut Rule) {
        self.advance(); // '{'
        self.skip_newlines();
        while !self.at(TokenKind::RBrace) && !self.at(TokenKind::Eof) && !self.too_many_errors() {
            // Migration aid: 'require' blocks have been removed.
            if self.at(TokenKind::Ident)
                && self.cur().str == "require"
                && self.toks[self.i + 1].kind == TokenKind::Ident
            {
                let t = self.cur().clone();
                let d = self.error_at_token(&t, "the 'require' block has been removed".to_string());
                d.hint = "constrain a referenced resource with nested object syntax, e.g.: cluster: { backup_retention: >= 30d }".to_string();
                self.sync_line();
                self.skip_newlines();
                continue;
            }
            if self.at_nested_block() {
                if let Some(b) = self.parse_resource_block(&[]) {
                    rule.blocks.push(Rc::new(b));
                }
            } else if let Some(prop) = self.parse_property() {
                rule.props.push(Rc::new(prop));
            }
            self.skip_newlines();
        }
        self.expect(TokenKind::RBrace, "to close the rule body");
        self.expect_terminator("the rule");
    }

    fn at_nested_block(&self) -> bool {
        if !self.at(TokenKind::Ident) {
            return false;
        }
        matches!(
            self.toks[self.i + 1].kind,
            TokenKind::String | TokenKind::Ident
        )
    }

    // --- selectors ---

    fn parse_selector(&mut self) -> Vec<Condition> {
        let mut conds = Vec::new();
        loop {
            if let Some(c) = self.parse_condition() {
                conds.push(c);
            } else {
                // Recovery: skip to something that can continue the selector.
                loop {
                    match self.cur().kind {
                        TokenKind::AndAnd
                        | TokenKind::Amp
                        | TokenKind::OrOr
                        | TokenKind::Pipe
                        | TokenKind::LBrace
                        | TokenKind::Newline
                        | TokenKind::Eof => break,
                        _ => {
                            self.advance();
                        }
                    }
                }
            }
            match self.cur().kind {
                TokenKind::AndAnd => {
                    self.advance();
                }
                TokenKind::Amp => {
                    let t = self.cur().clone();
                    let d = self.error_at_token(
                        &t,
                        "selector conditions are combined with '&&', not '&'".to_string(),
                    );
                    d.hint = "'&' combines property constraints; '&&' combines selector conditions"
                        .to_string();
                    self.advance();
                }
                TokenKind::OrOr | TokenKind::Pipe => {
                    let t = self.cur().clone();
                    let d = self
                        .error_at_token(&t, format!("'{}' is not supported in selectors", t.text));
                    d.hint =
                        "split the rule into separate rules, or use 'in [\"a\", \"b\"]' for membership"
                            .to_string();
                    self.advance();
                }
                _ => return conds,
            }
        }
    }

    fn parse_condition(&mut self) -> Option<Condition> {
        let (field, field_span) = self.parse_field_path("selector condition")?;
        let mut cond = Condition {
            pos: field_span.start.clone(),
            end: Position::default(),
            field,
            field_end: field_span.end,
            op: CondOp::Eq,
            values: Vec::new(),
            env_scoped: false,
        };

        let op_tok = self.cur().clone();
        match op_tok.kind {
            TokenKind::Eq | TokenKind::Neq => {
                self.advance();
                let (v, vspan) = self.parse_value(&format!("after '{}'", op_tok.text))?;
                cond.op = if op_tok.kind == TokenKind::Neq {
                    CondOp::Neq
                } else {
                    CondOp::Eq
                };
                cond.values = vec![v];
                cond.end = vspan.end;
            }
            TokenKind::Assign => {
                let d = self.error_at_token(&op_tok, "'=' is not an operator".to_string());
                d.hint = "use '==' for equality comparisons".to_string();
                self.advance();
                let (v, vspan) = self.parse_value("after '='")?;
                cond.op = CondOp::Eq;
                cond.values = vec![v];
                cond.end = vspan.end;
            }
            TokenKind::Exists => {
                self.advance();
                cond.op = CondOp::Exists;
                cond.end = op_tok.end.clone();
            }
            TokenKind::In => {
                self.advance();
                self.expect(TokenKind::LBracket, "after 'in'")?;
                loop {
                    let (v, _vspan) = self.parse_value("in the membership list")?;
                    cond.values.push(v);
                    if !self.accept(TokenKind::Comma) {
                        break;
                    }
                    if self.at(TokenKind::RBracket) {
                        break; // trailing comma
                    }
                }
                let end = self.expect(TokenKind::RBracket, "to close the membership list")?;
                cond.op = CondOp::In;
                cond.end = end.end;
            }
            _ => {
                let d = self.error_at_token(
                    &op_tok,
                    format!(
                        "expected '==', '!=', 'in', or 'exists' after '{}', found {}",
                        cond.field,
                        op_tok.describe()
                    ),
                );
                if op_tok.kind == TokenKind::LBrace {
                    d.message = format!(
                        "incomplete selector condition: '{}' needs an operator such as '==' or 'exists'",
                        cond.field
                    );
                }
                return None;
            }
        }
        Some(cond)
    }

    /// Parses a dotted identifier path like "env.type".
    fn parse_field_path(&mut self, context: &str) -> Option<(String, Span)> {
        let first = self.cur().clone();
        if first.kind != TokenKind::Ident {
            if first.kind.is_keyword() {
                let d = self.error_at_token(
                    &first,
                    format!(
                        "'{}' is a reserved keyword and cannot be used as a field name",
                        first.text
                    ),
                );
                d.hint = "rename the field, or quote the value if you meant a string".to_string();
                return None;
            }
            self.error_at_token(
                &first,
                format!(
                    "expected a field name in {}, found {}",
                    context,
                    first.describe()
                ),
            );
            return None;
        }
        self.advance();
        let mut path = first.str.clone();
        let mut end = first.end.clone();
        while self.at(TokenKind::Dot) {
            self.advance();
            let seg = self.cur().clone();
            if seg.kind != TokenKind::Ident {
                self.error_at_token(
                    &seg,
                    format!("expected an identifier after '.', found {}", seg.describe()),
                );
                return None;
            }
            self.advance();
            path = format!("{path}.{}", seg.str);
            end = seg.end.clone();
        }
        Some((
            path,
            Span {
                start: first.pos.clone(),
                end,
            },
        ))
    }

    // --- properties ---

    fn parse_property(&mut self) -> Option<Property> {
        let (path, span) = match self.parse_field_path("property rule") {
            Some(x) => x,
            None => {
                self.sync_line();
                return None;
            }
        };
        let mut prop = Property {
            pos: span.start.clone(),
            path_end: span.end.clone(),
            path: path.clone(),
            value: PropertyValue::Scalar(ScalarValue {
                constraint: None,
                default: None,
            }),
        };

        match self.cur().kind {
            TokenKind::Colon => {
                self.advance();
            }
            TokenKind::Assign => {
                let t = self.cur().clone();
                let d = self.error_at_token(&t, "property rules use ':', not '='".to_string());
                d.hint = format!("write: {path}: <constraint>");
                self.advance();
            }
            _ => {
                let t = self.cur().clone();
                self.error_at_token(
                    &t,
                    format!(
                        "expected ':' after property path '{path}', found {}",
                        t.describe()
                    ),
                );
                self.sync_line();
                return None;
            }
        }

        let (c, scalar_def, ref_def, ok) = self.parse_property_expr();
        if !ok {
            prop.value = self.build_property_value(&path, c, scalar_def, ref_def);
            self.sync_line();
            return Some(prop);
        }

        // 'default' must be the final clause.
        if (scalar_def.is_some() || ref_def.is_some())
            && matches!(
                self.cur().kind,
                TokenKind::Pipe | TokenKind::Amp | TokenKind::AndAnd | TokenKind::OrOr
            )
        {
            let t = self.cur().clone();
            let d = self.error_at_token(
                &t,
                "'default' must be the last clause in a property rule".to_string(),
            );
            d.hint = format!("write constraints first: {path}: <constraint> | default <value>");
            prop.value = self.build_property_value(&path, c, scalar_def, ref_def);
            self.sync_line();
            return Some(prop);
        }

        prop.value = self.build_property_value(&path, c, scalar_def, ref_def);
        if !self.expect_terminator(&format!("the property rule for '{path}'")) {
            self.sync_line();
        }
        Some(prop)
    }

    fn parse_property_expr(
        &mut self,
    ) -> (
        Option<Constraint>,
        Option<ScalarDefault>,
        Option<RefDefault>,
        bool,
    ) {
        if self.at(TokenKind::Default) {
            let (s, r, ok) = self.parse_default_clause();
            return (None, s, r, ok);
        }
        self.parse_or_expr()
    }

    fn parse_or_expr(
        &mut self,
    ) -> (
        Option<Constraint>,
        Option<ScalarDefault>,
        Option<RefDefault>,
        bool,
    ) {
        let (first, ok) = self.parse_and_expr();
        if !ok {
            return (first, None, None, false);
        }
        let mut alts = vec![first.unwrap()];
        loop {
            match self.cur().kind {
                TokenKind::Pipe => {
                    self.advance();
                }
                TokenKind::OrOr => {
                    let t = self.cur().clone();
                    let d = self.error_at_token(
                        &t,
                        "constraint alternatives are combined with '|', not '||'".to_string(),
                    );
                    d.hint = "'||' is not part of the language; use a single '|'".to_string();
                    self.advance();
                }
                _ => return (Some(or_of(alts)), None, None, true),
            }
            if self.at(TokenKind::Default) {
                let (s, r, ok) = self.parse_default_clause();
                return (Some(or_of(alts)), s, r, ok);
            }
            let (alt, ok) = self.parse_and_expr();
            if !ok {
                return (Some(or_of(alts)), None, None, false);
            }
            alts.push(alt.unwrap());
        }
    }

    fn parse_and_expr(&mut self) -> (Option<Constraint>, bool) {
        let (first, ok) = self.parse_term();
        if !ok {
            return (first, false);
        }
        let mut terms = vec![first.unwrap()];
        loop {
            match self.cur().kind {
                TokenKind::Amp => {
                    self.advance();
                }
                TokenKind::AndAnd => {
                    let t = self.cur().clone();
                    let d = self.error_at_token(
                        &t,
                        "property constraints are combined with '&', not '&&'".to_string(),
                    );
                    d.hint = "'&&' combines selector conditions; '&' combines property constraints"
                        .to_string();
                    self.advance();
                }
                _ => {
                    if terms.len() == 1 {
                        return (Some(terms.pop().unwrap()), true);
                    }
                    return (Some(Constraint::And(terms)), true);
                }
            }
            let (term, ok) = self.parse_term();
            if !ok {
                return (Some(Constraint::And(terms)), false);
            }
            terms.push(term.unwrap());
        }
    }

    fn parse_term(&mut self) -> (Option<Constraint>, bool) {
        let t = self.cur().clone();
        match t.kind {
            TokenKind::Required => {
                self.advance();
                (
                    Some(Constraint::Required(RequiredConstraint {
                        pos: t.pos,
                        end: t.end,
                    })),
                    true,
                )
            }
            TokenKind::Ge
            | TokenKind::Le
            | TokenKind::Gt
            | TokenKind::Lt
            | TokenKind::Neq
            | TokenKind::Eq => {
                self.advance();
                let (v, vspan) = match self.parse_value(&format!("after '{}'", t.text)) {
                    Some(x) => x,
                    None => return (None, false),
                };
                let op = comparison_op(t.kind);
                if is_ordering_op(op) && !v.kind.is_ordered() {
                    let kind = v.kind;
                    let vstr = v.to_string();
                    let d = self.error_at(
                        t.pos.clone(),
                        vspan.end.clone(),
                        format!(
                            "ordering comparison '{}' requires a number, size, or duration, found {} value {}",
                            t.text, kind, vstr
                        ),
                    );
                    if kind == ValueKind::Bool || kind == ValueKind::String {
                        d.hint = format!("use '==' or '!=' to compare {kind} values");
                    }
                }
                (
                    Some(Constraint::Comparison(Comparison {
                        pos: t.pos,
                        end: vspan.end,
                        op,
                        value: v,
                        implicit: false,
                    })),
                    true,
                )
            }
            TokenKind::Assign => {
                let d = self.error_at_token(&t, "'=' is not an operator".to_string());
                d.hint = "use '==' for an exact match, or omit the operator entirely".to_string();
                self.advance();
                let (v, vspan) = match self.parse_value("after '='") {
                    Some(x) => x,
                    None => return (None, false),
                };
                (
                    Some(Constraint::Comparison(Comparison {
                        pos: t.pos,
                        end: vspan.end,
                        op: CompareOp::Eq,
                        value: v,
                        implicit: false,
                    })),
                    true,
                )
            }
            TokenKind::Default => {
                // e.g. `cpu: >= 1 & default 2` — wrong combinator before default.
                let d = self.error_at_token(
                    &t,
                    "'default' must be separated from constraints with '|'".to_string(),
                );
                d.hint = "write: <constraint> | default <value>".to_string();
                (None, false)
            }
            TokenKind::LBrace => {
                let obj = self.parse_object_constraint();
                (Some(Constraint::Object(obj)), true)
            }
            TokenKind::Ident => {
                if self.at_reference() {
                    match self.parse_reference() {
                        Some(r) => (Some(Constraint::Reference(r)), true),
                        None => (None, false),
                    }
                } else {
                    // A bare identifier is not a value; strings must be quoted.
                    self.advance();
                    let d = self.error_at_token(&t, "string values must be quoted".to_string());
                    d.hint = format!("write {}", crate::value::go_quote(&t.str));
                    (
                        Some(Constraint::Comparison(Comparison {
                            pos: t.pos,
                            end: t.end,
                            op: CompareOp::Eq,
                            value: crate::value::string(t.str),
                            implicit: true,
                        })),
                        true,
                    )
                }
            }
            _ => {
                let (v, vspan) = match self.parse_value("as a constraint") {
                    Some(x) => x,
                    None => return (None, false),
                };
                (
                    Some(Constraint::Comparison(Comparison {
                        pos: vspan.start,
                        end: vspan.end,
                        op: CompareOp::Eq,
                        value: v,
                        implicit: true,
                    })),
                    true,
                )
            }
        }
    }

    fn parse_default_clause(&mut self) -> (Option<ScalarDefault>, Option<RefDefault>, bool) {
        let kw = self.advance(); // 'default'
        if self.at_reference() {
            match self.parse_reference() {
                Some(r) => (
                    None,
                    Some(RefDefault {
                        pos: kw.pos,
                        reference: r,
                    }),
                    true,
                ),
                None => (None, None, false),
            }
        } else {
            match self.parse_value("after 'default'") {
                Some((v, vspan)) => (
                    Some(ScalarDefault {
                        pos: kw.pos,
                        value: v,
                        value_pos: vspan.start,
                        value_end: vspan.end,
                    }),
                    None,
                    true,
                ),
                None => (None, None, false),
            }
        }
    }

    fn at_reference(&self) -> bool {
        self.at(TokenKind::Ident)
            && matches!(
                self.toks[self.i + 1].kind,
                TokenKind::Dot | TokenKind::LBracket
            )
    }

    fn parse_reference(&mut self) -> Option<Reference> {
        let kind_tok = self.advance(); // kind identifier
        let mut r = Reference {
            mode: RefMode::Static,
            kind: kind_tok.str.clone(),
            name: String::new(),
            expr: String::new(),
            pos: kind_tok.pos.clone(),
            end: kind_tok.end.clone(),
            kind_pos: kind_tok.pos.clone(),
            kind_end: kind_tok.end.clone(),
        };
        match self.cur().kind {
            TokenKind::Dot => {
                self.advance();
                let name = self.cur().clone();
                match name.kind {
                    TokenKind::Ident | TokenKind::String => {
                        self.advance();
                        r.mode = RefMode::Static;
                        r.name = name.str.clone();
                        r.end = name.end.clone();
                        if name.str.is_empty() {
                            self.error_at_token(
                                &name,
                                "reference name must not be empty".to_string(),
                            );
                        }
                        Some(r)
                    }
                    _ => {
                        self.error_at_token(
                            &name,
                            format!(
                                "expected a resource name after '{}.', found {}",
                                kind_tok.str,
                                name.describe()
                            ),
                        );
                        None
                    }
                }
            }
            TokenKind::LBracket => {
                self.advance();
                let (expr, _span) = self.parse_field_path("dynamic reference")?;
                let end = self.expect(TokenKind::RBracket, "to close the dynamic reference")?;
                r.mode = RefMode::Dynamic;
                r.expr = expr;
                r.end = end.end;
                Some(r)
            }
            _ => {
                let t = self.cur().clone();
                self.error_at_token(
                    &t,
                    format!("expected '.' or '[' in reference to '{}'", kind_tok.str),
                );
                None
            }
        }
    }

    fn parse_object_constraint(&mut self) -> ObjectConstraint {
        let lb = self.advance(); // '{'
        let mut obj = ObjectConstraint {
            pos: lb.pos.clone(),
            end: lb.end.clone(),
            props: Vec::new(),
        };
        self.skip_newlines();
        while !self.at(TokenKind::RBrace) && !self.at(TokenKind::Eof) && !self.too_many_errors() {
            if let Some(mut prop) = self.parse_property() {
                self.reject_object_default(&mut prop);
                obj.props.push(Rc::new(prop));
            }
            self.skip_newlines();
        }
        if let Some(rb) = self.expect(TokenKind::RBrace, "to close the object constraint") {
            obj.end = rb.end;
        }
        obj
    }

    fn reject_object_default(&mut self, prop: &mut Property) {
        match &mut prop.value {
            PropertyValue::Scalar(sv) => {
                if sv.default.is_some() {
                    let (pos, end) = {
                        let d = sv.default.as_ref().unwrap();
                        (d.pos.clone(), d.value_end.clone())
                    };
                    let diag = self.error_at(
                        pos,
                        end,
                        "'default' is not allowed inside an object constraint".to_string(),
                    );
                    diag.hint = "an object constraint only constrains the referenced resource; set defaults in rules for its own kind".to_string();
                    sv.default = None;
                }
            }
            PropertyValue::Ref(rv) => {
                if rv.default.is_some() {
                    let (pos, end) = {
                        let d = rv.default.as_ref().unwrap();
                        (d.pos.clone(), d.reference.end.clone())
                    };
                    let diag = self.error_at(
                        pos,
                        end,
                        "'default' is not allowed inside an object constraint".to_string(),
                    );
                    diag.hint =
                        "an object constraint only constrains the referenced resource".to_string();
                    rv.default = None;
                }
            }
        }
    }

    fn build_property_value(
        &mut self,
        path: &str,
        c: Option<Constraint>,
        scalar_def: Option<ScalarDefault>,
        ref_def: Option<RefDefault>,
    ) -> PropertyValue {
        let rt = split_ref_terms(c.as_ref());
        if !rt.mix_msg.is_empty() {
            let d = self.error_at(
                rt.mix_span.start.clone(),
                rt.mix_span.end.clone(),
                rt.mix_msg.clone(),
            );
            d.hint = "a reference property takes a reference and/or an object constraint, not scalar comparisons".to_string();
        }
        if rt.has_ref || ref_def.is_some() {
            if let Some(sd) = &scalar_def {
                let pos = sd.value_pos.clone();
                let end = sd.value_end.clone();
                self.error_at(
                    pos,
                    end,
                    format!(
                        "property '{path}' is a reference; its default must be a reference too"
                    ),
                );
            }
            PropertyValue::Ref(RefValue {
                reference: rt.reference,
                object: rt.object,
                default: ref_def,
            })
        } else {
            PropertyValue::Scalar(ScalarValue {
                constraint: c,
                default: scalar_def,
            })
        }
    }

    // --- values ---

    fn parse_value(&mut self, context: &str) -> Option<(crate::value::Value, Span)> {
        let t = self.cur().clone();
        match t.kind {
            TokenKind::Minus => {
                self.advance();
                let num = self.cur().clone();
                if num.kind != TokenKind::Number {
                    self.error_at_token(
                        &num,
                        format!("expected a number after '-', found {}", num.describe()),
                    );
                    return None;
                }
                let (mut v, span) = self.parse_value(context)?;
                v.num = -v.num;
                Some((
                    v,
                    Span {
                        start: t.pos,
                        end: span.end,
                    },
                ))
            }
            TokenKind::Number => {
                self.advance();
                let v = self.number_value(&t);
                Some((v, t.span()))
            }
            TokenKind::String => {
                self.advance();
                Some((crate::value::string(t.str.clone()), t.span()))
            }
            TokenKind::True => {
                self.advance();
                Some((boolean(true), t.span()))
            }
            TokenKind::False => {
                self.advance();
                Some((boolean(false), t.span()))
            }
            TokenKind::Ident => {
                self.advance();
                let d = self.error_at_token(&t, "string values must be quoted".to_string());
                d.hint = format!("write {}", crate::value::go_quote(&t.str));
                Some((crate::value::string(t.str.clone()), t.span()))
            }
            _ => {
                if t.kind.is_keyword() {
                    let d = self.error_at_token(
                        &t,
                        format!(
                            "'{}' is a reserved keyword and cannot be used as a value",
                            t.text
                        ),
                    );
                    d.hint = format!("quote it if you meant the string \"{}\"", t.text);
                    return None;
                }
                self.error_at_token(
                    &t,
                    format!("expected a value {}, found {}", context, t.describe()),
                );
                None
            }
        }
    }

    fn number_value(&mut self, t: &Token) -> crate::value::Value {
        if t.unit.is_empty() {
            return number(t.num);
        }
        if crate::value::size_factor(&t.unit).is_some() {
            return crate::value::size(t.num, &t.unit).unwrap();
        }
        if crate::value::duration_factor(&t.unit).is_some() {
            return crate::value::duration(t.num, &t.unit).unwrap();
        }
        let all = crate::value::all_unit_names();
        let s = crate::util::suggest(&t.unit, &all);
        let d = self.error_at_token(t, format!("unknown unit '{}' in '{}'", t.unit, t.text));
        if !s.is_empty() {
            d.hint = format!(
                "did you mean '{}{}'? valid units are {} (size) and {} (duration)",
                format_float(t.num),
                s,
                crate::value::size_unit_list(),
                crate::value::duration_unit_list()
            );
        } else {
            d.hint = format!(
                "valid units are {} (size) and {} (duration)",
                crate::value::size_unit_list(),
                crate::value::duration_unit_list()
            );
        }
        // Recover as a plain number so parsing can continue.
        number(t.num)
    }

    fn at_version_decl(&self) -> bool {
        self.at(TokenKind::Ident)
            && self.cur().str == "version"
            && self.toks[self.i + 1].kind == TokenKind::Number
    }

    fn parse_version(&mut self) {
        self.advance(); // 'version'
        let t = match self.expect(TokenKind::Number, "after 'version'") {
            Some(t) => t,
            None => {
                self.sync_line();
                return;
            }
        };
        if !t.unit.is_empty() || t.num != (t.num as i64) as f64 {
            self.error_at_token(
                &t,
                format!("version must be an integer, found '{}'", t.text),
            );
        } else if (t.num as i64) != 1 {
            let d =
                self.error_at_token(&t, format!("unsupported language version {}", t.num as i64));
            d.hint = "this parser supports version 1".to_string();
        }
        self.version = Some(Version {
            pos: t.pos.clone(),
            num: t.num as i64,
        });
        self.expect_terminator("the version declaration");
    }

    fn parse_import(&mut self) {
        let kw = self.advance(); // 'import'
        let t = match self.expect(TokenKind::String, "with the file path after 'import'") {
            Some(t) => t,
            None => {
                self.sync_line();
                return;
            }
        };
        if t.str.is_empty() {
            self.error_at_token(&t, "import path must not be empty".to_string());
        }
        self.imports.push(Import {
            pos: kw.pos.clone(),
            path: t.str.clone(),
            path_pos: t.pos.clone(),
            path_end: t.end.clone(),
        });
        self.expect_terminator("the import declaration");
    }
}

// --- free helpers ---

/// Concatenates enclosing if-block conditions with a rule's own, copying so
/// rules never share backing arrays.
fn combine_conds(outer: &[Condition], own: Vec<Condition>) -> Vec<Condition> {
    if outer.is_empty() {
        return own;
    }
    let mut conds = Vec::with_capacity(outer.len() + own.len());
    conds.extend(outer.iter().cloned());
    conds.extend(own);
    conds
}

fn or_of(alts: Vec<Constraint>) -> Constraint {
    if alts.len() == 1 {
        alts.into_iter().next().unwrap()
    } else {
        Constraint::Or(alts)
    }
}

fn comparison_op(k: TokenKind) -> CompareOp {
    match k {
        TokenKind::Eq => CompareOp::Eq,
        TokenKind::Neq => CompareOp::Neq,
        TokenKind::Ge => CompareOp::Ge,
        TokenKind::Le => CompareOp::Le,
        TokenKind::Gt => CompareOp::Gt,
        TokenKind::Lt => CompareOp::Lt,
        _ => panic!("not a comparison token"),
    }
}

fn is_ordering_op(op: CompareOp) -> bool {
    matches!(
        op,
        CompareOp::Ge | CompareOp::Le | CompareOp::Gt | CompareOp::Lt
    )
}

struct RefTerms {
    reference: Option<Reference>,
    object: Option<ObjectConstraint>,
    has_ref: bool,
    mix_span: Span,
    mix_msg: String,
}

fn ref_terms_none() -> RefTerms {
    RefTerms {
        reference: None,
        object: None,
        has_ref: false,
        mix_span: Span::default(),
        mix_msg: String::new(),
    }
}

/// Inspects a constraint expression for reference and object terms.
fn split_ref_terms(c: Option<&Constraint>) -> RefTerms {
    match c {
        None => ref_terms_none(),
        Some(Constraint::Reference(r)) => RefTerms {
            reference: Some(r.clone()),
            object: None,
            has_ref: true,
            mix_span: Span::default(),
            mix_msg: String::new(),
        },
        Some(Constraint::Object(o)) => RefTerms {
            reference: None,
            object: Some(o.clone()),
            has_ref: true,
            mix_span: Span::default(),
            mix_msg: String::new(),
        },
        Some(Constraint::And(terms)) => {
            let mut reference: Option<Reference> = None;
            let mut object: Option<ObjectConstraint> = None;
            let mut has_scalar = false;
            for term in terms {
                match term {
                    Constraint::Reference(rr) => {
                        if reference.is_some() {
                            return RefTerms {
                                reference,
                                object,
                                has_ref: true,
                                mix_span: rr.span(),
                                mix_msg: "a property cannot have more than one reference"
                                    .to_string(),
                            };
                        }
                        reference = Some(rr.clone());
                    }
                    Constraint::Object(oo) => {
                        if object.is_some() {
                            return RefTerms {
                                reference,
                                object,
                                has_ref: true,
                                mix_span: oo.span(),
                                mix_msg: "a property cannot have more than one object constraint"
                                    .to_string(),
                            };
                        }
                        object = Some(oo.clone());
                    }
                    _ => has_scalar = true,
                }
            }
            if (reference.is_some() || object.is_some()) && has_scalar {
                return RefTerms {
                    reference,
                    object,
                    has_ref: true,
                    mix_span: c.unwrap().span(),
                    mix_msg: "a reference cannot be combined with scalar constraints".to_string(),
                };
            }
            if reference.is_some() || object.is_some() {
                return RefTerms {
                    reference,
                    object,
                    has_ref: true,
                    mix_span: Span::default(),
                    mix_msg: String::new(),
                };
            }
            ref_terms_none()
        }
        Some(Constraint::Or(alts)) => {
            for alt in alts {
                if split_ref_terms(Some(alt)).has_ref {
                    return RefTerms {
                        reference: None,
                        object: None,
                        has_ref: true,
                        mix_span: c.unwrap().span(),
                        mix_msg: "a reference cannot appear in a '|' alternative".to_string(),
                    };
                }
            }
            ref_terms_none()
        }
        _ => ref_terms_none(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::ast::File;
    use crate::position::Position;
    use crate::testutil::{assert_err_contains, parse_set};
    use crate::value::{boolean, number, values_equal};

    fn parse_ok(src: &str) -> File {
        let pr = parse_file("p.encore", src);
        assert!(pr.errors.is_empty(), "unexpected errors:\n{}", pr.errors);
        pr.file
    }

    #[test]
    fn parse_rule_headers() {
        let src = r#"
for service {
    cpu: default 1
}
service "api" {
    cpu: default 2
}
for service if env.type == "production" {
    cpu: default 3
}
service "api" if env.type == "production" && team == "payments" {
    cpu: default 4
}
for bucket if tags.data exists {
    versioning: true
}
for service if env.type in ["production", "staging"] {
    cpu: >= 0.5
}
for service if env.type != "preview" {
    cpu: >= 0.5
}
"#;
        let f = parse_ok(src);
        assert_eq!(f.rules.len(), 7);
        let headers: Vec<String> = f.rules.iter().map(|r| r.header()).collect();
        assert_eq!(
            headers,
            vec![
                "for service",
                "service \"api\"",
                "for service if env.type == \"production\"",
                "service \"api\" if env.type == \"production\" && team == \"payments\"",
                "for bucket if tags.data exists",
                "for service if env.type in [\"production\", \"staging\"]",
                "for service if env.type != \"preview\"",
            ]
        );
        assert_eq!(f.rules[1].name, "api");
        assert_eq!(f.rules[3].wheres.len(), 2);
        assert_eq!(f.rules[5].wheres[0].op, CondOp::In);
        assert_eq!(f.rules[5].wheres[0].values.len(), 2);
    }

    #[test]
    fn parse_dynamic_block() {
        let src = r#"
for service if tags.domain exists {
    instance: default service_instance[tags.domain]
    service_instance tags.domain {
        cpu: default 2
    }
}
"#;
        let f = parse_ok(src);
        assert_eq!(f.rules.len(), 1);
        let parent = &f.rules[0];
        assert_eq!(parent.props.len(), 1);
        assert_eq!(parent.blocks.len(), 1);
        let b = &parent.blocks[0];
        assert_eq!(b.kind, "service_instance");
        assert_eq!(b.dyn_expr, "tags.domain");
        assert_eq!(b.header(), "service_instance tags.domain");
    }

    #[test]
    fn parse_property_rules() {
        let src = r#"
for service {
    cpu: >= 0.25 & <= 8 | default 0.5
    memory: >= 256Mi & <= 16Gi | default 512Mi
    instances.min: default 1
    public_access: false
    region: "europe-west1" | "europe-north1"
    tier: "small" | "medium" | "large"
    backup_retention: required & >= 30d
    timeout: != 30s
    provider.gcp.cloud_run.cpu_always_allocated: true
    explicit: false | default false
}
"#;
        let f = parse_ok(src);
        assert_eq!(f.rules.len(), 1);
        let props: Vec<String> = f.rules[0].props.iter().map(|p| p.to_string()).collect();
        assert_eq!(
            props,
            vec![
                "cpu: >= 0.25 & <= 8 | default 0.5",
                "memory: >= 256Mi & <= 16Gi | default 512Mi",
                "instances.min: default 1",
                "public_access: false",
                "region: \"europe-west1\" | \"europe-north1\"",
                "tier: \"small\" | \"medium\" | \"large\"",
                "backup_retention: required & >= 30d",
                "timeout: != 30s",
                "provider.gcp.cloud_run.cpu_always_allocated: true",
                "explicit: false | default false",
            ]
        );

        // `default` binds last.
        let cpu = f.rules[0].props[0].scalar().unwrap();
        match &cpu.constraint {
            Some(Constraint::And(terms)) => assert_eq!(terms.len(), 2),
            _ => panic!("expected And constraint"),
        }
        assert!(values_equal(
            &cpu.default.as_ref().unwrap().value,
            &number(0.5)
        ));

        // A bare exact value parses as an implicit equality comparison.
        let pub_ = f.rules[0].props[3].scalar().unwrap();
        match &pub_.constraint {
            Some(Constraint::Comparison(cmp)) => {
                assert_eq!(cmp.op, CompareOp::Eq);
                assert!(cmp.implicit);
                assert!(values_equal(&cmp.value, &boolean(false)));
            }
            _ => panic!("expected Comparison"),
        }
        assert!(pub_.default.is_none());
    }

    #[test]
    fn parse_version_and_imports() {
        let src = "version 1\nimport \"policies/common.encore\"\nimport \"policies/storage.encore\"\n\nfor service {\n    cpu: default 1\n}\n";
        let f = parse_ok(src);
        assert_eq!(f.version.as_ref().unwrap().num, 1);
        assert_eq!(f.imports.len(), 2);
        assert_eq!(f.imports[0].path, "policies/common.encore");
        assert_eq!(f.imports[1].path, "policies/storage.encore");
    }

    #[test]
    fn parse_single_line_rule() {
        let f = parse_ok("for service { cpu: default 1 }");
        assert_eq!(f.rules.len(), 1);
        assert_eq!(f.rules[0].props.len(), 1);
    }

    #[test]
    fn parse_negative_numbers() {
        let f = parse_ok("for service {\n    offset: >= -2 | default -1\n}\n");
        let sv = f.rules[0].props[0].scalar().unwrap();
        match &sv.constraint {
            Some(Constraint::Comparison(cmp)) => assert!(values_equal(&cmp.value, &number(-2.0))),
            _ => panic!("expected Comparison"),
        }
        assert!(values_equal(
            &sv.default.as_ref().unwrap().value,
            &number(-1.0)
        ));
    }

    #[test]
    fn parse_errors() {
        let cases: &[(&str, &[&str])] = &[
            (
                "for service if env.type = production {\n}\n",
                &["'=' is not an operator", "use '==' for equality comparisons"],
            ),
            (
                "for service if a == 1 || b == 2 {\n}\n",
                &["'||' is not supported in selectors", "split the rule into separate rules"],
            ),
            (
                "for service if a == 1 & b == 2 {\n}\n",
                &["selector conditions are combined with '&&', not '&'"],
            ),
            (
                "for service {\n    cpu: >= 1 && <= 4\n}\n",
                &["property constraints are combined with '&', not '&&'"],
            ),
            (
                "for service {\n    cpu: default 2 | <= 4\n}\n",
                &["'default' must be the last clause in a property rule"],
            ),
            (
                "for service {\n    cpu: >= 1 & default 2\n}\n",
                &["'default' must be separated from constraints with '|'"],
            ),
            (
                "for service {\n    cpu >= 1\n}\n",
                &["expected ':' after property path 'cpu'"],
            ),
            (
                "for service {\n    memory: >= 512G\n}\n",
                &["unknown unit 'G' in '512G'", "did you mean '512GB'?"],
            ),
            (
                "for service {\n    region: >= \"a\"\n}\n",
                &["ordering comparison '>=' requires a number, size, or duration", "string"],
            ),
            (
                "for service {\n    flag: < true\n}\n",
                &["ordering comparison '<' requires a number, size, or duration", "bool"],
            ),
            (
                "for service\n",
                &["expected '{' to begin the rule body, found newline"],
            ),
            (
                "for service {\n    cpu: default 1\n",
                &["expected '}' to close the rule body, found end of file"],
            ),
            (
                "for \"api\" {\n}\n",
                &["expected a resource kind after 'for'", "the resource kind comes before the name"],
            ),
            (
                "for service \"api\" {\n}\n",
                &["a named block omits 'for'", "write: service \"api\" { ... }"],
            ),
            (
                "for service tags.domain {\n}\n",
                &["a dynamic block omits 'for'"],
            ),
            (
                "sql_cluster {\n}\n",
                &["a resource block needs a name or expression"],
            ),
            (
                "version 2\nfor service {\n}\n",
                &["unsupported language version 2", "this parser supports version 1"],
            ),
            (
                "for service {\n}\nversion 1\n",
                &["the version declaration must be the first statement"],
            ),
            (
                "for service {\n}\nimport \"x.encore\"\n",
                &["import declarations must appear before the first rule"],
            ),
            (
                "for service if default == 1 {\n}\n",
                &["'default' is a reserved keyword and cannot be used as a field name"],
            ),
            (
                "for service if env.type == if {\n}\n",
                &["'if' is a reserved keyword and cannot be used as a value"],
            ),
            (
                "for service if env.type {\n}\n",
                &["incomplete selector condition: 'env.type' needs an operator such as '==' or 'exists'"],
            ),
            (
                "\"oops\"\n",
                &["expected 'for', a resource block, or 'if' to begin a declaration", "string \"oops\""],
            ),
            (
                "for service if env.type in [] {\n}\n",
                &["expected a value in the membership list"],
            ),
            (
                "for service {\n    cpu: >= 1 <= 4\n}\n",
                &["expected newline after the property rule for 'cpu'"],
            ),
        ];
        for (src, want) in cases {
            let pr = parse_file("policy.encore", src);
            assert!(!pr.errors.is_empty(), "src: {src:?}");
            assert_err_contains(&pr.errors, want);
        }
    }

    #[test]
    fn parse_error_recovery() {
        let src = "for service {\n    cpu: default 2 | <= 4\n}\nfor bucket {\n    versioning = true\n}\nfor sql_database {\n    backup_retention: >= 30x\n}\n";
        let pr = parse_file("policy.encore", src);
        assert_eq!(pr.errors.len(), 3);
        assert!(pr.errors.0[0]
            .message
            .contains("'default' must be the last clause"));
        assert!(pr.errors.0[1]
            .message
            .contains("property rules use ':', not '='"));
        assert!(pr.errors.0[2].message.contains("unknown unit 'x'"));
        assert_eq!(pr.file.rules.len(), 3);
    }

    #[test]
    fn parse_too_many_errors() {
        let mut src = String::new();
        for _ in 0..30 {
            src.push_str("for service {\n    cpu = 1\n}\n");
        }
        let pr = parse_file("policy.encore", &src);
        assert!(!pr.errors.is_empty());
        assert!(pr.errors.len() <= MAX_PARSE_ERRORS + 1);
        assert!(pr.errors.to_string().contains("too many errors"));
    }

    #[test]
    fn parse_error_positions() {
        let pr = parse_file(
            "policy.encore",
            "for service if env.type = \"production\" {\n}\n",
        );
        assert_eq!(pr.errors.len(), 1);
        assert_eq!(
            pr.errors.0[0].pos,
            Position {
                file: "policy.encore".to_string(),
                offset: 24,
                line: 1,
                column: 25
            }
        );
    }

    #[test]
    fn parse_required_inside_or_alternative() {
        let rs = parse_set("for service {\n    cpu: required | >= 2\n}\n");
        let err = rs.validate().unwrap_err();
        assert_err_contains(
            &err,
            &[
                "'required' cannot be part of a '|' alternative",
                "combine it with '&' instead",
            ],
        );
    }
}
