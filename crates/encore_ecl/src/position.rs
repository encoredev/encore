use std::fmt;

/// A location in an ECL source file.
/// Lines and columns are 1-based; columns count runes, not bytes.
#[derive(Clone, Debug, Default, PartialEq, Eq, Hash)]
pub struct Position {
    pub file: String,
    /// byte offset within the file
    pub offset: usize,
    pub line: usize,
    pub column: usize,
}

impl Position {
    /// Reports whether the position refers to an actual source location.
    pub fn is_valid(&self) -> bool {
        self.line > 0
    }
}

impl fmt::Display for Position {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match (self.file.is_empty(), self.is_valid()) {
            (true, false) => write!(f, "<unknown position>"),
            (_, false) => write!(f, "{}", self.file),
            (true, _) => write!(f, "{}:{}", self.line, self.column),
            _ => write!(f, "{}:{}:{}", self.file, self.line, self.column),
        }
    }
}

/// A contiguous range of source text.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Span {
    pub start: Position,
    pub end: Position,
}

/// Holds the contents of a parsed file for snippet rendering.
#[derive(Debug)]
pub(crate) struct SourceFile {
    pub(crate) name: String,
    pub(crate) src: String,
    /// byte offset of the start of each line, 0-based index
    line_start: Vec<usize>,
}

impl SourceFile {
    pub(crate) fn new(name: impl Into<String>, src: impl Into<String>) -> SourceFile {
        let src = src.into();
        let mut starts = vec![0usize];
        for (i, b) in src.bytes().enumerate() {
            if b == b'\n' {
                starts.push(i + 1);
            }
        }
        SourceFile {
            name: name.into(),
            src,
            line_start: starts,
        }
    }

    /// Returns the text of the given 1-based line, without the trailing newline.
    pub(crate) fn line(&self, n: usize) -> Option<&str> {
        if n < 1 || n > self.line_start.len() {
            return None;
        }
        let start = self.line_start[n - 1];
        let end = if n < self.line_start.len() {
            self.line_start[n] - 1
        } else {
            self.src.len()
        };
        Some(self.src[start..end].trim_end_matches('\r'))
    }
}
