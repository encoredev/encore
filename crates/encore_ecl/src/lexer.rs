use std::rc::Rc;

use crate::diagnostic::ErrorList;
use crate::position::{Position, SourceFile};
use crate::token::{keyword, Token, TokenKind};

pub(crate) struct Lexer<'a> {
    src: Rc<SourceFile>,
    offset: usize,
    line: usize, // 1-based
    col: usize,  // 1-based, counted in runes
    diags: &'a mut ErrorList,
}

impl<'a> Lexer<'a> {
    pub(crate) fn new(src: Rc<SourceFile>, diags: &'a mut ErrorList) -> Lexer<'a> {
        Lexer {
            src,
            offset: 0,
            line: 1,
            col: 1,
            diags,
        }
    }

    /// Tokenizes the entire file. Runs of newlines collapse into a single
    /// newline token, and newlines inside `[...]` lists are suppressed so that
    /// lists may span lines.
    pub(crate) fn lex(&mut self) -> Vec<Token> {
        let mut toks: Vec<Token> = Vec::new();
        let mut bracket_depth = 0i32;
        loop {
            let t = self.next();
            match t.kind {
                TokenKind::Newline => {
                    if bracket_depth > 0 {
                        continue;
                    }
                    if toks.is_empty() || toks.last().unwrap().kind == TokenKind::Newline {
                        continue;
                    }
                }
                TokenKind::LBracket => bracket_depth += 1,
                TokenKind::RBracket if bracket_depth > 0 => {
                    bracket_depth -= 1;
                }
                _ => {}
            }
            let is_eof = t.kind == TokenKind::Eof;
            toks.push(t);
            if is_eof {
                return toks;
            }
        }
    }

    fn pos(&self) -> Position {
        Position {
            file: self.src.name.clone(),
            offset: self.offset,
            line: self.line,
            column: self.col,
        }
    }

    fn eof(&self) -> bool {
        self.offset >= self.src.src.len()
    }

    fn peek(&self) -> u8 {
        if self.eof() {
            0
        } else {
            self.src.src.as_bytes()[self.offset]
        }
    }

    fn peek_at(&self, n: usize) -> u8 {
        if self.offset + n >= self.src.src.len() {
            0
        } else {
            self.src.src.as_bytes()[self.offset + n]
        }
    }

    /// Consumes one rune.
    fn advance(&mut self) -> char {
        let r = self.src.src[self.offset..].chars().next().unwrap();
        self.offset += r.len_utf8();
        if r == '\n' {
            self.line += 1;
            self.col = 1;
        } else {
            self.col += 1;
        }
        r
    }

    fn errorf(&mut self, start: Position, end: Position, message: String) {
        self.diags.add(Some(self.src.clone()), start, end, message);
    }

    fn mk(&self, kind: TokenKind, start: &Position) -> Token {
        let end = self.pos();
        Token {
            kind,
            text: self.src.src[start.offset..end.offset].to_string(),
            pos: start.clone(),
            end,
            num: 0.0,
            unit: String::new(),
            str: String::new(),
        }
    }

    fn next(&mut self) -> Token {
        self.skip_space_and_comments();
        let start = self.pos();
        if self.eof() {
            return Token {
                kind: TokenKind::Eof,
                pos: start.clone(),
                end: start,
                text: String::new(),
                num: 0.0,
                unit: String::new(),
                str: String::new(),
            };
        }

        let c = self.peek();
        match c {
            b'\n' => {
                self.advance();
                return self.mk(TokenKind::Newline, &start);
            }
            b'"' => return self.lex_string(),
            b'0'..=b'9' => return self.lex_number(),
            _ if is_ident_start(c) => return self.lex_ident(),
            _ => {}
        }

        self.advance();
        match c {
            b'{' => return self.mk(TokenKind::LBrace, &start),
            b'}' => return self.mk(TokenKind::RBrace, &start),
            b'[' => return self.mk(TokenKind::LBracket, &start),
            b']' => return self.mk(TokenKind::RBracket, &start),
            b',' => return self.mk(TokenKind::Comma, &start),
            b':' => return self.mk(TokenKind::Colon, &start),
            b'.' => return self.mk(TokenKind::Dot, &start),
            b'-' => return self.mk(TokenKind::Minus, &start),
            b'=' => {
                if self.peek() == b'=' {
                    self.advance();
                    return self.mk(TokenKind::Eq, &start);
                }
                return self.mk(TokenKind::Assign, &start);
            }
            b'!' => {
                if self.peek() == b'=' {
                    self.advance();
                    return self.mk(TokenKind::Neq, &start);
                }
                let end = self.pos();
                self.errorf(
                    start,
                    end,
                    "unexpected '!'; use '!=' for inequality".to_string(),
                );
                return self.next();
            }
            b'>' => {
                if self.peek() == b'=' {
                    self.advance();
                    return self.mk(TokenKind::Ge, &start);
                }
                return self.mk(TokenKind::Gt, &start);
            }
            b'<' => {
                if self.peek() == b'=' {
                    self.advance();
                    return self.mk(TokenKind::Le, &start);
                }
                return self.mk(TokenKind::Lt, &start);
            }
            b'&' => {
                if self.peek() == b'&' {
                    self.advance();
                    return self.mk(TokenKind::AndAnd, &start);
                }
                return self.mk(TokenKind::Amp, &start);
            }
            b'|' => {
                if self.peek() == b'|' {
                    self.advance();
                    return self.mk(TokenKind::OrOr, &start);
                }
                return self.mk(TokenKind::Pipe, &start);
            }
            _ => {}
        }

        let end = self.pos();
        self.errorf(
            start,
            end,
            format!("unexpected character {}", char_quote(c as char)),
        );
        self.next()
    }

    /// Skips spaces, tabs, carriage returns, and comments. Newlines are
    /// significant and not skipped. Block comments are treated as plain
    /// whitespace, even when they span lines.
    fn skip_space_and_comments(&mut self) {
        while !self.eof() {
            let c = self.peek();
            if c == b' ' || c == b'\t' || c == b'\r' {
                self.advance();
            } else if c == b'/' && self.peek_at(1) == b'/' {
                while !self.eof() && self.peek() != b'\n' {
                    self.advance();
                }
            } else if c == b'/' && self.peek_at(1) == b'*' {
                let start = self.pos();
                self.advance();
                self.advance();
                let mut closed = false;
                while !self.eof() {
                    if self.peek() == b'*' && self.peek_at(1) == b'/' {
                        self.advance();
                        self.advance();
                        closed = true;
                        break;
                    }
                    self.advance();
                }
                if !closed {
                    let mut end = start.clone();
                    end.column += 2;
                    end.offset += 2;
                    self.errorf(start, end, "unterminated block comment".to_string());
                }
            } else {
                return;
            }
        }
    }

    fn lex_string(&mut self) -> Token {
        let start = self.pos();
        self.advance(); // opening quote
        let mut b = String::new();
        loop {
            if self.eof() || self.peek() == b'\n' {
                let mut end = start.clone();
                end.column += 1;
                end.offset += 1;
                self.errorf(
                    start.clone(),
                    end,
                    "unterminated string literal".to_string(),
                );
                break;
            }
            let r = self.advance();
            if r == '"' {
                break;
            }
            if r == '\\' {
                if self.eof() || self.peek() == b'\n' {
                    continue;
                }
                let mut esc_pos = self.pos();
                esc_pos.column -= 1; // point at the backslash
                esc_pos.offset -= 1;
                let e = self.advance();
                match e {
                    '"' => b.push('"'),
                    '\\' => b.push('\\'),
                    'n' => b.push('\n'),
                    't' => b.push('\t'),
                    'r' => b.push('\r'),
                    _ => {
                        let end = self.pos();
                        self.errorf(
                            esc_pos,
                            end,
                            format!("invalid escape sequence '\\{e}' in string literal"),
                        );
                        b.push(e);
                    }
                }
                continue;
            }
            b.push(r);
        }
        let end = self.pos();
        Token {
            kind: TokenKind::String,
            text: self.src.src[start.offset..end.offset].to_string(),
            pos: start,
            end,
            num: 0.0,
            unit: String::new(),
            str: b,
        }
    }

    fn lex_number(&mut self) -> Token {
        let start = self.pos();
        while !self.eof() && self.peek().is_ascii_digit() {
            self.advance();
        }
        if self.peek() == b'.' && self.peek_at(1).is_ascii_digit() {
            self.advance();
            while !self.eof() && self.peek().is_ascii_digit() {
                self.advance();
            }
        }
        let num_end = self.offset;

        // Unit suffix: a run of letters immediately following the number,
        // e.g. "512Mi" or "30d". Validated by the parser.
        while !self.eof() && self.peek().is_ascii_alphabetic() {
            self.advance();
        }
        let end = self.pos();

        let text = self.src.src[start.offset..end.offset].to_string();
        let num_text = &self.src.src[start.offset..num_end];
        let num = match num_text.parse::<f64>() {
            Ok(n) => n,
            Err(_) => {
                self.errorf(
                    start.clone(),
                    end.clone(),
                    format!("invalid number {}", crate::value::go_quote(num_text)),
                );
                0.0
            }
        };
        let unit = self.src.src[num_end..end.offset].to_string();
        Token {
            kind: TokenKind::Number,
            text,
            pos: start,
            end,
            num,
            unit,
            str: String::new(),
        }
    }

    fn lex_ident(&mut self) -> Token {
        let start = self.pos();
        while !self.eof() && is_ident_part(self.peek()) {
            self.advance();
        }
        let end = self.pos();
        let name = self.src.src[start.offset..end.offset].to_string();
        let kind = keyword(&name).unwrap_or(TokenKind::Ident);
        Token {
            kind,
            text: name.clone(),
            pos: start,
            end,
            num: 0.0,
            unit: String::new(),
            str: name,
        }
    }
}

fn is_ident_start(b: u8) -> bool {
    let r = b as char;
    r == '_' || r.is_alphabetic()
}

fn is_ident_part(b: u8) -> bool {
    let r = b as char;
    r == '_' || r.is_alphabetic() || r.is_numeric()
}

/// Renders a char the way Go's `%q` verb does, e.g. `'@'`.
fn char_quote(c: char) -> String {
    match c {
        '\'' => "'\\''".to_string(),
        '\\' => "'\\\\'".to_string(),
        '\n' => "'\\n'".to_string(),
        '\t' => "'\\t'".to_string(),
        '\r' => "'\\r'".to_string(),
        c if c == ' ' || c.is_ascii_graphic() || (!c.is_control() && c as u32 >= 0x80) => {
            format!("'{c}'")
        }
        c if (c as u32) < 0x80 => format!("'\\x{:02x}'", c as u32),
        c => format!("'\\u{:04x}'", c as u32),
    }
}

#[cfg(test)]
mod tests {
    use crate::position::Position;
    use crate::testutil::lex_all;
    use crate::token::{Token, TokenKind};

    fn kinds_of(toks: &[Token]) -> Vec<TokenKind> {
        toks.iter().map(|t| t.kind).collect()
    }

    #[test]
    fn lexer_tokens() {
        use TokenKind::*;
        let (toks, diags) = lex_all("cpu: >= 1.5 & <= 4Gi | default 2 // trailing comment");
        assert!(diags.is_empty());
        assert_eq!(
            kinds_of(&toks),
            vec![Ident, Colon, Ge, Number, Amp, Le, Number, Pipe, Default, Number, Eof]
        );
        assert_eq!(toks[3].num, 1.5);
        assert_eq!(toks[3].unit, "");
        assert_eq!(toks[6].num, 4.0);
        assert_eq!(toks[6].unit, "Gi");
    }

    #[test]
    fn lexer_operators() {
        use TokenKind::*;
        let (toks, diags) = lex_all("== != >= <= > < && & || | = . , [ ] { } -");
        assert!(diags.is_empty());
        assert_eq!(
            kinds_of(&toks),
            vec![
                Eq, Neq, Ge, Le, Gt, Lt, AndAnd, Amp, OrOr, Pipe, Assign, Dot, Comma, LBracket,
                RBracket, LBrace, RBrace, Minus, Eof
            ]
        );
    }

    #[test]
    fn lexer_keywords() {
        use TokenKind::*;
        let (toks, diags) =
            lex_all("for where in exists required default import true false forx version");
        assert!(diags.is_empty());
        assert_eq!(
            kinds_of(&toks),
            vec![For, Where, In, Exists, Required, Default, Import, True, False, Ident, Ident, Eof]
        );
        assert_eq!(toks[9].str, "forx");
        assert_eq!(toks[10].str, "version");
    }

    #[test]
    fn lexer_positions() {
        let (toks, diags) = lex_all("for service {\n  cpu: 1\n}");
        assert!(diags.is_empty());
        assert_eq!(
            toks[0].pos,
            Position {
                file: "test.encore".to_string(),
                offset: 0,
                line: 1,
                column: 1
            }
        );
        assert_eq!(toks[1].pos.column, 5); // service
        assert_eq!(toks[2].pos.column, 13);
        assert_eq!(toks[3].kind, TokenKind::Newline);
        assert_eq!(
            toks[4].pos,
            Position {
                file: "test.encore".to_string(),
                offset: 16,
                line: 2,
                column: 3
            }
        );
        assert_eq!(toks[4].end.column, 6);
    }

    #[test]
    fn lexer_newline_collapsing() {
        use TokenKind::*;
        let (toks, diags) = lex_all("\n\n\na\n\n\nb in [x,\n  y]\n");
        assert!(diags.is_empty());
        assert_eq!(
            kinds_of(&toks),
            vec![Ident, Newline, Ident, In, LBracket, Ident, Comma, Ident, RBracket, Newline, Eof]
        );
    }

    #[test]
    fn lexer_comments() {
        use TokenKind::*;
        let (toks, diags) = lex_all("// line comment\na /* inline */ b\n/* multi\nline */ d");
        assert!(diags.is_empty());
        assert_eq!(kinds_of(&toks), vec![Ident, Ident, Newline, Ident, Eof]);
    }

    #[test]
    fn lexer_strings() {
        let (toks, diags) = lex_all(r#""plain" "with \"escapes\"\n\t\\""#);
        assert!(diags.is_empty());
        assert_eq!(toks[0].str, "plain");
        assert_eq!(toks[1].str, "with \"escapes\"\n\t\\");
    }

    #[test]
    fn lexer_errors() {
        let cases = [
            ("\"unterminated", "unterminated string literal"),
            ("\"bad \\q escape\"", "invalid escape sequence '\\q'"),
            ("/* unterminated", "unterminated block comment"),
            ("a @ b", "unexpected character '@'"),
            ("a ! b", "unexpected '!'; use '!=' for inequality"),
        ];
        for (src, message) in cases {
            let (_, diags) = lex_all(src);
            assert!(!diags.is_empty(), "src: {src:?}");
            assert!(
                diags.0[0].message.contains(message),
                "src: {src:?}, got: {}",
                diags.0[0].message
            );
        }
    }

    #[test]
    fn lexer_number_units() {
        let (toks, diags) = lex_all("512Mi 30d 1.5h 100ms 2TB 0.25");
        assert!(diags.is_empty());
        let want = [
            (512.0, "Mi"),
            (30.0, "d"),
            (1.5, "h"),
            (100.0, "ms"),
            (2.0, "TB"),
            (0.25, ""),
        ];
        for (i, (num, unit)) in want.iter().enumerate() {
            assert_eq!(toks[i].num, *num);
            assert_eq!(toks[i].unit, *unit);
        }
    }
}
