use std::{fmt::Write, ops::Deref};

use litparser::Sp;
use swc_common::{BytePos, Span};

use crate::span_err::{ErrorWithSpanExt, SpErr};

use super::types::validation;

#[derive(Debug, Clone)]
pub struct Path {
    pub span: Span,
    pub segments: Vec<Sp<Segment>>,
}

impl Path {
    pub fn dynamic_segments(&self) -> impl Iterator<Item = &Sp<Segment>> {
        self.segments
            .iter()
            .filter(|s| !matches!(s.get(), Segment::Literal(_)))
    }

    pub fn has_dynamic_segments(&self) -> bool {
        self.dynamic_segments().next().is_some()
    }
}

impl std::fmt::Display for Path {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        for seg in &self.segments {
            f.write_char('/')?;
            if let Some(sigil) = seg.sigil() {
                f.write_char(sigil)?;
            }
            f.write_str(seg.lit_or_name())?;
        }
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub enum Segment {
    Literal(String),
    Param {
        name: String,
        value_type: ValueType,
        validation: Option<validation::Expr>,
    },
    Wildcard {
        name: String,
        validation: Option<validation::Expr>,
    },
    Fallback {
        name: String,
        validation: Option<validation::Expr>,
    },
}

impl Segment {
    pub fn sigil(&self) -> Option<char> {
        match self {
            Segment::Literal(_s) => None,
            Segment::Param { .. } => Some(':'),
            Segment::Wildcard { .. } => Some('*'),
            Segment::Fallback { .. } => Some('!'),
        }
    }

    pub fn lit_or_name(&self) -> &str {
        match self {
            Segment::Literal(s) => s,
            Segment::Param { name, .. } => name,
            Segment::Wildcard { name, .. } => name,
            Segment::Fallback { name, .. } => name,
        }
    }

    pub fn is_literal(&self) -> bool {
        matches!(self, Segment::Literal(_))
    }

    pub fn is_dynamic(&self) -> bool {
        !self.is_literal()
    }
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum ValueType {
    String,
    Int,
    Bool,
}

pub struct ParseOptions {
    pub allow_wildcard: bool,
    pub allow_fallback: bool,
    pub prefix_slash: bool,
}

impl Default for ParseOptions {
    fn default() -> Self {
        ParseOptions {
            allow_wildcard: true,
            allow_fallback: true,
            prefix_slash: true,
        }
    }
}

#[derive(Debug, thiserror::Error, Clone, PartialEq, Eq)]
pub enum PathParseError {
    #[error("empty path")]
    EmptyPath,
    #[error("path must start with '/'")]
    MustStartWithSlash,
    #[error("path must not start with '/'")]
    MustNotStartWithSlash,
    #[error("path cannot contain empty path segment")]
    EmptySegment,
    #[error("path parameters must have a name")]
    UnnamedParam,
    #[error("wildcard segment must be at the end of the path")]
    WildcardNotAtEnd,
    #[error("fallback segment must be at the end of the path")]
    FallbackNotAtEnd,
    #[error("path cannot contain query parameters (the '?' character)")]
    ContainsQuery,
    #[error("path cannot contain url fragment (the '#' character)")]
    ContainsFragment,
    #[error("path cannot contain url scheme")]
    ContainsScheme,
    #[error("path cannot contain url authority")]
    ContainsAuthority,
    #[error("path cannot contain hostname")]
    ContainsHostname,
    #[error("path is invalid: {0}")]
    Invalid(String),
}

impl Path {
    pub fn parse(
        span: Span,
        path: &str,
        opts: ParseOptions,
    ) -> Result<Self, SpErr<PathParseError>> {
        if path.is_empty() {
            return Err(PathParseError::EmptyPath.with_span(span));
        } else if !path.starts_with('/') && opts.prefix_slash {
            return Err(PathParseError::MustStartWithSlash.with_span(span));
        } else if path.starts_with('/') && !opts.prefix_slash {
            return Err(PathParseError::MustNotStartWithSlash.with_span(span));
        }

        // Ensure this is a valid url path.
        parse_url_path(path).map_err(|err| err.with_span(span))?;

        let mut segments = vec![];

        let path_end = path.len();
        let mut idx = 0;
        while idx < path_end {
            if opts.prefix_slash || !segments.is_empty() {
                idx += 1; // drop leading slash
            }

            let seg_start = idx;
            let seg_end = {
                let remainder = &path[idx..];
                idx + remainder.find('/').unwrap_or(remainder.len())
            };

            // Find the next path segment.
            let val = &path[seg_start..seg_end];
            let seg: Segment = match val.chars().next() {
                Some(':') => Segment::Param {
                    name: val[1..].to_string(),
                    value_type: ValueType::String,
                    validation: None,
                },
                Some('*') if opts.allow_wildcard => Segment::Wildcard {
                    name: val[1..].to_string(),
                    validation: None,
                },
                Some('!') if opts.allow_wildcard => Segment::Fallback {
                    name: val[1..].to_string(),
                    validation: None,
                },
                _ => Segment::Literal(val.to_string()),
            };

            let span = span
                .with_lo(span.lo + BytePos(seg_start as u32))
                .with_hi(span.hi + BytePos(seg_end as u32));

            segments.push(Sp::new(span, seg));
            idx = seg_end;
        }

        // Validate the segments.
        for (idx, seg) in segments.iter().enumerate() {
            match seg.deref() {
                Segment::Literal(lit) if lit.is_empty() && segments.len() > 1 => {
                    return Err(PathParseError::EmptySegment.with_span(seg.span()));
                }
                Segment::Param { name, .. } if name.is_empty() => {
                    return Err(PathParseError::UnnamedParam.with_span(seg.span()));
                }
                Segment::Wildcard { name, .. } if name.is_empty() => {
                    return Err(PathParseError::UnnamedParam.with_span(seg.span()));
                }
                Segment::Wildcard { .. } if idx != segments.len() - 1 => {
                    return Err(PathParseError::WildcardNotAtEnd.with_span(seg.span()));
                }
                Segment::Fallback { .. } if idx != segments.len() - 1 => {
                    return Err(PathParseError::FallbackNotAtEnd.with_span(seg.span()));
                }
                _ => {}
            }
        }

        Ok(Path { span, segments })
    }
}

fn parse_url_path(path: &str) -> Result<(), PathParseError> {
    // The url crate only supports parsing absolute urls, so use a dummy base
    // and ensure it is the same after parsing.
    let base = url::Url::parse("base://url.here").expect("internal error: invalid base url");

    let url = url::Url::options()
        .base_url(Some(&base))
        .parse(path)
        .map_err(|err| PathParseError::Invalid(err.to_string()))?;

    if url.scheme() != base.scheme() {
        return Err(PathParseError::ContainsScheme);
    } else if url.authority() != base.authority() {
        return Err(PathParseError::ContainsAuthority);
    }

    match url.host_str() {
        None => {
            // We should always have a host since the base url has one.
            return Err(PathParseError::ContainsHostname);
        }
        Some(host) => {
            if host != base.host_str().unwrap() {
                return Err(PathParseError::ContainsHostname);
            }
        }
    }

    if url.query().is_some() {
        Err(PathParseError::ContainsQuery)
    } else if url.fragment().is_some() {
        Err(PathParseError::ContainsFragment)
    } else {
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use swc_common::DUMMY_SP;

    use super::*;

    #[test]
    fn test_parse() {
        let tests = vec![
            ("/", Ok(vec![Segment::Literal("".to_string())])),
            ("/foo", Ok(vec![Segment::Literal("foo".to_string())])),
            (
                "/foo/bar",
                Ok(vec![
                    Segment::Literal("foo".to_string()),
                    Segment::Literal("bar".to_string()),
                ]),
            ),
            (
                "/:foo/*bar",
                Ok(vec![
                    Segment::Param {
                        name: "foo".to_string(),
                        value_type: ValueType::String,
                        validation: None,
                    },
                    Segment::Wildcard {
                        name: "bar".to_string(),
                        validation: None,
                    },
                ]),
            ),
            ("", Err(PathParseError::EmptyPath)),
            ("/foo//bar", Err(PathParseError::EmptySegment)),
            ("/foo/", Err(PathParseError::EmptySegment)),
            ("/:foo/*", Err(PathParseError::UnnamedParam)),
            ("/:foo/*/bar", Err(PathParseError::UnnamedParam)),
            ("/:foo/*bar/baz", Err(PathParseError::WildcardNotAtEnd)),
            ("/foo?bar=baz", Err(PathParseError::ContainsQuery)),
            ("/foo#bar", Err(PathParseError::ContainsFragment)),
            (
                "/foo/!fallback",
                Ok(vec![
                    Segment::Literal("foo".to_string()),
                    Segment::Fallback {
                        name: "fallback".to_string(),
                        validation: None,
                    },
                ]),
            ),
        ];

        for (path, want) in tests {
            let got = Path::parse(DUMMY_SP, path, Default::default());
            match (got, want) {
                (Ok(got), Ok(want)) => {
                    let segments: Vec<_> = got.segments.into_iter().map(|s| s.take()).collect();
                    assert_eq!(segments, want, "path {:?}", path);
                }
                (Err(got), Err(want)) => {
                    assert_eq!(got.error, want, "path {:?}", path);
                }
                (Ok(got), Err(want)) => {
                    panic!("got {:?}, want err {:?}, path {:?}", got, want, path);
                }
                (Err(got), Ok(want)) => {
                    panic!("got err {:?}, want {:?}, path {:?}", got, want, path);
                }
            }
        }
    }

    #[test]
    fn test_parse_url_path() {
        let ok_paths = &["foo", "/foo/bar", "/foo/:bar", "/*wildcard", "/!fallback"];
        let err_paths = &["http://foo.com"];

        for path in ok_paths {
            let path = parse_url_path(path);
            assert!(path.is_ok(), "path {:?} should be ok", path);
        }

        for path in err_paths {
            let path = parse_url_path(path);
            assert!(path.is_err(), "path {:?} should be err", path);
        }
    }
}
