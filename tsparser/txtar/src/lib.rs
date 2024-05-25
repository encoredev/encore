//! `txtar` is a rust implementation of the `txtar` Go package.
//!
//! # About
//!
//! `txtar`s purpose is best described in the original Go package:
//!
//! > Package txtar implements a trivial text-based file archive format.
//! >
//! > The goals for the format are:
//! >
//! > - be trivial enough to create and edit by hand.
//! > - be able to store trees of text files describing go command test cases.
//! > - diff nicely in git history and code reviews.
//! >
//! > Non-goals include being a completely general archive format,
//! > storing binary data, storing file modes, storing special files like
//! > symbolic links, and so on.
//!
//!
//! ## format spec
//! The format spec as written in the `txtar` Go package source code:
//!
//! > Txtar format
//! >
//! > A txtar archive is zero or more comment lines and then a sequence of file entries.
//! > Each file entry begins with a file marker line of the form "`-- FILENAME --`"
//! > and is followed by zero or more file content lines making up the file data.
//! > The comment or file content ends at the next file marker line.
//! > The file marker line must begin with the three-byte sequence "`-- `"
//! > and end with the three-byte sequence "` --`", but the enclosed
//! > file name can be surrounding by additional white space,
//! > all of which is stripped.
//! >
//! > If the txtar file is missing a trailing newline on the final line,
//! > parsers should consider a final newline to be present anyway.
//! >
//! > There are no possible syntax errors in a txtar archive.
//!
//! # Example
//!
//! ```rust no_run
//! let txt = "\
//! comment1
//! comment2
//! -- file1 --
//! File 1 text.
//! -- foo/bar --
//! File 2 text.
//! -- empty --
//! -- noNL --
//! hello world";
//!
//! let archive = txtar::from_str(txt);
//! archive.materialize("/tmp/somedir/").unwrap();
//! ```

mod error;

use std::{
    fmt::Display,
    fs,
    io::{self, BufWriter, Write},
    path::{Path, PathBuf},
    str,
};

use clean_path::Clean;

pub use error::MaterializeError;

/**
An archive represents a tree of text files.

This type is used to read txtar files from disk and materialize the
corresponding file layout.

# Examples

```rust no_run
use txtar::Archive;

let txt = "\
comment1
comment2
-- file1 --
File 1 text.
-- foo --
File 2 text.
-- empty --
-- noNL --
hello world";

let archive = Archive::from(txt);
archive.materialize("/tmp/somedir/").unwrap();
```
**/
#[derive(Debug, Default, Eq, PartialEq, Clone)]
pub struct Archive {
    // internal invariant:
    // comment is fix_newlined
    pub comment: String,
    pub files: Vec<File>,
}

#[derive(Debug, Eq, PartialEq, Clone)]
pub struct File {
    pub name: PathBuf,
    // internal invariant:
    // data is fix_newlined
    pub data: String,
}

impl File {
    pub fn new<P: AsRef<Path>>(name: P, data: &str) -> File {
        let name = name.as_ref().to_owned();
        let mut data = data.to_owned();
        fix_newline(&mut data);

        File { name, data }
    }
}

impl Archive {
    fn new(comment: &str, files: Vec<File>) -> Archive {
        let mut comment = comment.to_owned();
        fix_newline(&mut comment);

        Archive { comment, files }
    }

    /// Serialize the archive as txtar into the I/O stream.
    pub fn to_writer<W: Write>(&self, writer: &mut W) -> io::Result<()> {
        write!(writer, "{}", self)
    }

    /// Writes each file in this archive to the directory at the given
    /// path.
    ///
    /// # Errors
    ///
    /// This function will error in the event a file would be written
    /// outside of the directory or if an existing file would be
    /// overwritten. Additionally, any errors caused by the underlying
    /// I/O operations will be propagated.
    pub fn materialize<P: AsRef<Path>>(&self, path: P) -> Result<(), MaterializeError> {
        let path = path.as_ref();
        for File { name, data } in &self.files {
            let name_path = name.clean();
            if name_path.starts_with("../") || name_path.is_absolute() {
                return Err(MaterializeError::DirEscape(
                    name_path.to_string_lossy().to_string(),
                ));
            }

            let rel_path = name_path;
            let path = path.join(rel_path);
            if let Some(p) = path.parent() {
                fs::create_dir_all(p)?;
            }

            let mut file = fs::File::options()
                .write(true)
                .create_new(true)
                .open(path)?;
            let mut w = BufWriter::new(&mut file);
            w.write_all(data.as_bytes())?;
        }

        Ok(())
    }
}

impl Display for Archive {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.comment)?;

        for File { name, data } in &self.files {
            let name = name.display();
            writeln!(f, "-- {name} --")?;
            write!(f, "{data}")?;
        }

        Ok(())
    }
}

impl TryFrom<&[u8]> for Archive {
    type Error = std::str::Utf8Error;

    fn try_from(slice: &[u8]) -> Result<Self, Self::Error> {
        let s = str::from_utf8(slice)?;
        Ok(Archive::from(s))
    }
}

impl From<&str> for Archive {
    fn from(s: &str) -> Archive {
        let (comment, mut name, mut s) = split_file_markers(s);
        let mut files = Vec::new();

        while !name.is_empty() {
            let (data, next_name, rest) = split_file_markers(s);

            let file = File::new(name, data);
            files.push(file);

            name = next_name;
            s = rest;
        }

        Archive::new(comment, files)
    }
}

/// Read an archive from a string of txtar data.
pub fn from_str(s: &str) -> Archive {
    Archive::from(s)
}

/// Try to read an archive from bytes of txtar data.
pub fn from_bytes(slice: &[u8]) -> Result<Archive, std::str::Utf8Error> {
    Archive::try_from(slice)
}

fn split_file_markers(s: &str) -> (&str, &str, &str) {
    const NEWLINE_MARKER: &str = "\n-- ";
    const MARKER: &str = "-- ";
    const MARKER_END: &str = " --";

    let (prefix, rest) = if s.starts_with(MARKER) {
        ("", s)
    } else {
        match s.find(NEWLINE_MARKER) {
            None => return (s, "", ""),
            Some(offset) => s.split_at(offset + 1),
        }
    };
    debug_assert!(rest.starts_with(MARKER));

    let (filename, suffix) = match rest.split_once('\n') {
        None if rest.ends_with(MARKER_END) => (rest, ""),
        None => return (s, "", ""),
        Some((n, pf)) => (n, pf),
    };

    let filename = filename.trim_end_matches('\r');
    debug_assert!(filename.ends_with(MARKER_END));

    let filename = filename
        .strip_prefix(MARKER)
        .and_then(|filename| filename.strip_suffix(MARKER_END))
        .unwrap()
        .trim();
    (prefix, filename, suffix)
}

fn fix_newline(s: &mut String) {
    if !s.is_empty() && !s.ends_with('\n') {
        s.push('\n');
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert_fs::{prelude::*, TempDir};
    use predicates::prelude::{predicate::str::contains, *};
    use similar_asserts::{assert_eq, assert_str_eq};

    const BASIC: &str = "\
comment1
comment2
-- file1 --
File 1 text.
-- foo --
File 2 text.
-- empty --
-- noNL --
hello world";

    #[test]
    fn parse_format() {
        // Test simplest
        {
            let simplest = "-- simplest.txt --";
            let expected = format!("{simplest}\n");
            check_parse_format("simplest", simplest, &expected);
        }

        // Test basic variety of inputs
        {
            let basic = BASIC;
            let expected = format!("{basic}\n");
            check_parse_format("basic", basic, &expected);
        }

        // Test CRLF input
        {
            let crlf = "blah\r\n-- hello --\r\nhello\r\n";
            let expected = "\
Archive { comment: \"blah\\r\\n\", files: [File { name: \"hello\", data: \"hello\\r\\n\" }] }";

            let arch = format!("{:?}", Archive::from(crlf));
            assert_str_eq!(&arch, expected, "parse[CRLF input]",);
        }

        // Test whitespace handling
        {
            let txtar = "--  a  --";
            let expected = "-- a --\n";
            check_parse_format("whitespace", txtar, expected)
        }
    }

    fn check_parse_format(name: &str, txtar: &str, expected: &str) {
        let arch = Archive::from(txtar);
        let txtar = arch.to_string();
        assert_str_eq!(txtar, expected, "parse[{name}]");
    }

    #[test]
    fn materialize_basic() {
        let dir = TempDir::new().unwrap();
        let exists = predicate::path::exists();
        let empty = predicate::str::is_empty().from_utf8().from_file_path();
        {
            let good = Archive::from("-- good.txt --");
            good.materialize(&dir)
                .expect("good.materialize should not error");
            dir.child("good.txt").assert(exists).assert(empty);
        }
        {
            let basic = Archive::from(BASIC);
            basic
                .materialize(&dir)
                .expect("basic.materialize should not error");

            check_contents(&dir, "file1", "File 1 text.");
            check_contents(&dir, "foo", "File 2 text.");
            check_contents(&dir, "noNL", "hello world");
            dir.child("empty").assert(exists).assert(empty);
        }
        {
            let bad_rel = Archive::from("-- ../bad.txt --");
            check_bad_materialize(&dir, bad_rel, "../bad.txt");

            let bad_abs = Archive::from("-- /bad.txt --");
            check_bad_materialize(&dir, bad_abs, "/bad.txt");
        }
    }

    #[test]
    fn materialize_nested() {
        let dir = TempDir::new().unwrap();

        {
            let nested = Archive::from(
                "comment\n\
			 -- foo/foo.txt --\nThis is foo.\n\
			 -- bar/bar.txt --\nThis is bar.\n\
			 -- bar/deep/deeper/abyss.txt --\nThis is in the DEEPS.",
            );
            nested
                .materialize(&dir)
                .expect("nested.materialize should not error");

            check_contents(&dir, "foo/foo.txt", "This is foo.");
            check_contents(&dir, "bar/bar.txt", "This is bar.");
            check_contents(&dir, "bar/deep/deeper/abyss.txt", "This is in the DEEPS.");
        }
        {
            let bad_nested_rel = Archive::from("-- bar/deep/deeper/../../../../escaped.txt --");
            check_bad_materialize(&dir, bad_nested_rel, "../escaped.txt");
        }
    }

    fn check_contents(dir: &TempDir, child: &str, contents: &str) {
        let exists = predicate::path::exists();
        let newline_ending = predicate::str::ends_with("\n").from_utf8().from_file_path();
        dir.child(child)
            .assert(exists)
            .assert(contains(contents))
            .assert(newline_ending);
    }

    fn check_bad_materialize(dir: &TempDir, bad_rel: Archive, expected: &str) {
        let err = bad_rel.materialize(dir);
        match err {
            Err(MaterializeError::DirEscape(p)) => assert_eq!(p, expected.to_string()),
            Err(e) => panic!("expected `MaterializeError::DirEscape`, got {:?}", e),
            Ok(_) => panic!(
                "materialize({}) outside of parent dir should have failed",
                expected
            ),
        }
    }
}
