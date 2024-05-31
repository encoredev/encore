use std::io;
use std::path::{Path, PathBuf};

use crate::parser::doc_comments::doc_comments_before;
use anyhow::Result;
use serde::Serialize;
use swc_common::sync::Lrc;
use swc_common::{BytePos, Span, SyntaxContext};

pub struct FileSet {
    source_map: Lrc<swc_common::SourceMap>,
}

impl std::fmt::Debug for FileSet {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("FileSet").finish()
    }
}

impl FileSet {
    pub(super) fn new(source_map: Lrc<swc_common::SourceMap>) -> Lrc<Self> {
        Lrc::new(Self { source_map })
    }

    pub fn lookup_file<P: Into<Pos>>(&self, pos: P) -> Option<Lrc<SourceFile>> {
        let pos = pos.into();
        if pos.0 == 0 {
            return None;
        }
        let f = self.source_map.lookup_byte_offset(pos.into());
        Some(Lrc::new(SourceFile { file: f.sf }))
    }

    pub fn lookup_line<P: Into<Pos>>(&self, pos: P) -> (Lrc<SourceFile>, Option<usize>) {
        let pos = pos.into();
        match self.source_map.lookup_line(pos.into()) {
            Ok(file_and_line) => {
                let f = Lrc::new(SourceFile {
                    file: file_and_line.sf,
                });
                (f, Some(file_and_line.line))
            }
            Err(file) => (Lrc::new(SourceFile { file }), None),
        }
    }

    pub fn load_file(&self, path: &Path) -> io::Result<Lrc<SourceFile>> {
        let file = self.source_map.load_file(path)?;
        Ok(Lrc::new(SourceFile { file }))
    }

    pub fn new_source_file(&self, file_name: FilePath, src: String) -> Lrc<SourceFile> {
        let file = self
            .source_map
            .new_source_file(file_name.into(), src.into());
        Lrc::new(SourceFile { file })
    }

    pub fn preceding_comments(
        &self,
        comments: &dyn swc_common::comments::Comments,
        pos: Pos,
    ) -> Option<String> {
        doc_comments_before(&self.source_map, comments, pos.into())
    }
}

pub struct SourceFile {
    file: Lrc<swc_common::SourceFile>,
}

impl SourceFile {
    pub fn name(&self) -> FilePath {
        match self.file.name {
            swc_common::FileName::Real(ref p) => FilePath::Real(p.to_owned()),
            swc_common::FileName::Custom(ref s) => FilePath::Custom(s.to_owned()),
            _ => panic!("expected real file name"),
        }
    }
}

impl<'a> From<&'a SourceFile> for swc_common::input::StringInput<'a> {
    fn from(file: &'a SourceFile) -> Self {
        file.file.as_ref().into()
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum FilePath {
    Real(PathBuf),
    Custom(String),
}

impl FilePath {
    pub fn is_tsx(&self) -> bool {
        match self {
            FilePath::Real(p) => p.extension().map_or(false, |ext| ext == "tsx"),
            FilePath::Custom(p) => p.ends_with(".tsx"),
        }
    }

    pub fn is_dts(&self) -> bool {
        match self {
            FilePath::Real(p) => p.ends_with(".d.ts"),
            FilePath::Custom(p) => p.ends_with(".d.ts"),
        }
    }
}

impl std::fmt::Display for FilePath {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            FilePath::Real(p) => write!(f, "FilePath::Real({})", p.display()),
            FilePath::Custom(p) => write!(f, "FilePath::Custom({})", p),
        }
    }
}

impl Into<swc_common::FileName> for FilePath {
    fn into(self) -> swc_common::FileName {
        match self {
            FilePath::Real(p) => swc_common::FileName::Real(p),
            FilePath::Custom(p) => swc_common::FileName::Custom(p),
        }
    }
}

impl From<PathBuf> for FilePath {
    fn from(p: PathBuf) -> Self {
        Self::Real(p)
    }
}

impl From<&str> for FilePath {
    fn from(p: &str) -> Self {
        Self::from(PathBuf::from(p))
    }
}

#[derive(Debug, Clone, Copy, Hash, PartialEq, Eq, Ord, PartialOrd, Serialize)]
pub struct Pos(pub u32);

impl Into<swc_common::BytePos> for Pos {
    fn into(self) -> swc_common::BytePos {
        swc_common::BytePos(self.0)
    }
}

impl From<swc_common::BytePos> for Pos {
    fn from(pos: swc_common::BytePos) -> Self {
        Self(pos.0)
    }
}

#[derive(Debug, Clone, Copy, Hash, PartialEq, Eq, Ord, PartialOrd, Serialize)]
pub struct Range {
    pub start: Pos,
    pub end: Pos,
}

impl Range {
    /// Report the file name this range is in.
    pub fn file(&self, fset: &FileSet) -> FilePath {
        let f = fset.source_map.lookup_byte_offset(self.start.into());
        match &f.sf.name {
            swc_common::FileName::Real(p) => FilePath::Real(p.to_owned()),
            swc_common::FileName::Custom(s) => FilePath::Custom(s.to_owned()),
            _ => panic!("expected real file name"),
        }
    }

    /// Report the file name this range is in.
    pub fn loc(&self, fset: &FileSet) -> Result<Loc> {
        Ok(match fset.source_map.span_to_lines(self.to_span()) {
            Ok(lines) => {
                let file = match &lines.file.name {
                    swc_common::FileName::Real(p) => FilePath::Real(p.to_owned()),
                    swc_common::FileName::Custom(s) => FilePath::Custom(s.to_owned()),
                    _ => anyhow::bail!("expected real file name"),
                };
                match (lines.lines.first(), lines.lines.last()) {
                    (Some(first), Some(last)) => Loc {
                        file,
                        start_pos: (self.start.0 - lines.file.start_pos.0) as usize,
                        end_pos: (self.end.0 - lines.file.start_pos.0) as usize,
                        src_line_start: first.line_index + 1,
                        src_line_end: last.line_index + 1,
                        src_col_start: first.start_col.0,
                        src_col_end: last.end_col.0,
                    },
                    (_, _) => anyhow::bail!("missing line information"),
                }
            }
            Err(_) => anyhow::bail!("missing file information"),
        })
    }

    /// Whether the range contains another range.
    pub fn contains(&self, other: &Self) -> bool {
        self.start <= other.start && other.end <= self.end
    }

    pub fn to_span(&self) -> swc_common::Span {
        swc_common::Span {
            lo: swc_common::BytePos(self.start.0),
            hi: swc_common::BytePos(self.end.0),
            ctxt: SyntaxContext::empty(),
        }
    }
}

pub struct Loc {
    pub file: FilePath,

    /// Start and end positions within the file.
    pub start_pos: usize,
    pub end_pos: usize,

    /// Start and end lines within the file.
    pub src_line_start: usize,
    pub src_line_end: usize,

    /// Start and end columns within the line.
    pub src_col_start: usize,
    pub src_col_end: usize,
}

impl From<swc_common::Span> for Range {
    fn from(span: swc_common::Span) -> Self {
        Self {
            start: Pos(span.lo.0),
            end: Pos(span.hi.0),
        }
    }
}

impl Default for Range {
    fn default() -> Self {
        ZERO_RANGE.clone()
    }
}

pub const ZERO_RANGE: Range = Range {
    start: Pos(0),
    end: Pos(0),
};
