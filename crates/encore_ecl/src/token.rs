use std::fmt;

use crate::position::{Position, Span};

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
#[repr(i32)]
pub(crate) enum TokenKind {
    Eof,
    Newline,
    Ident,
    String,
    Number,

    LBrace,   // {
    RBrace,   // }
    LBracket, // [
    RBracket, // ]
    Comma,    // ,
    Colon,    // :
    Dot,      // .
    Minus,    // -

    Eq,     // ==
    Neq,    // !=
    Ge,     // >=
    Le,     // <=
    Gt,     // >
    Lt,     // <
    AndAnd, // &&
    Amp,    // &
    OrOr,   // ||
    Pipe,   // |
    Assign, // = (always an error; kept so the parser can suggest '==')

    For,
    Where,
    If,
    In,
    Exists,
    Required,
    Default,
    Import,
    True,
    False,
}

impl TokenKind {
    pub(crate) fn is_keyword(self) -> bool {
        (self as i32) >= (TokenKind::For as i32) && (self as i32) <= (TokenKind::False as i32)
    }

    fn name(self) -> &'static str {
        use TokenKind::*;
        match self {
            Eof => "end of file",
            Newline => "newline",
            Ident => "identifier",
            String => "string",
            Number => "number",
            LBrace => "'{'",
            RBrace => "'}'",
            LBracket => "'['",
            RBracket => "']'",
            Comma => "','",
            Colon => "':'",
            Dot => "'.'",
            Minus => "'-'",
            Eq => "'=='",
            Neq => "'!='",
            Ge => "'>='",
            Le => "'<='",
            Gt => "'>'",
            Lt => "'<'",
            AndAnd => "'&&'",
            Amp => "'&'",
            OrOr => "'||'",
            Pipe => "'|'",
            Assign => "'='",
            For => "keyword 'for'",
            Where => "keyword 'where'",
            If => "keyword 'if'",
            In => "keyword 'in'",
            Exists => "keyword 'exists'",
            Required => "keyword 'required'",
            Default => "keyword 'default'",
            Import => "keyword 'import'",
            True => "'true'",
            False => "'false'",
        }
    }
}

impl fmt::Display for TokenKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.name())
    }
}

// Note: "version" is intentionally not a keyword. The version declaration at
// the top of a file is recognized contextually, so that "version" stays usable
// as a property name (e.g. `version: "16"` in a resource block).
pub(crate) fn keyword(name: &str) -> Option<TokenKind> {
    use TokenKind::*;
    Some(match name {
        "for" => For,
        "where" => Where,
        "if" => If,
        "in" => In,
        "exists" => Exists,
        "required" => Required,
        "default" => Default,
        "import" => Import,
        "true" => True,
        "false" => False,
        _ => return None,
    })
}

#[derive(Clone, Debug)]
pub(crate) struct Token {
    pub(crate) kind: TokenKind,
    /// start of the token
    pub(crate) pos: Position,
    /// position just past the token
    pub(crate) end: Position,
    /// raw source text
    pub(crate) text: String,

    /// numeric value for `Number`
    pub(crate) num: f64,
    /// unit suffix for `Number` ("" if none)
    pub(crate) unit: String,
    /// decoded value for `String`, name for `Ident`
    pub(crate) str: String,
}

impl Token {
    /// Renders a token for use in error messages, e.g. "identifier 'cpu'".
    pub(crate) fn describe(&self) -> String {
        match self.kind {
            TokenKind::Ident => format!("identifier '{}'", self.str),
            TokenKind::String => format!("string {}", crate::value::go_quote(&self.str)),
            TokenKind::Number => format!("number '{}'", self.text),
            _ => self.kind.to_string(),
        }
    }

    pub(crate) fn span(&self) -> Span {
        Span {
            start: self.pos.clone(),
            end: self.end.clone(),
        }
    }
}
