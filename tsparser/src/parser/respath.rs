use std::fmt::Write;

#[derive(Debug, Clone)]
pub struct Path {
    pub segments: Vec<Segment>,
}

impl Path {
    pub fn dynamic_segments(&self) -> impl Iterator<Item = &Segment> {
        self.segments
            .iter()
            .filter(|s| !matches!(s, Segment::Literal(_)))
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
    Param { name: String, value_type: ValueType },
    Wildcard { name: String },
    Fallback { name: String },
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
            Segment::Wildcard { name } => name,
            Segment::Fallback { name } => name,
        }
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

impl Path {
    pub fn parse(path: &str, opts: ParseOptions) -> anyhow::Result<Self> {
        if path == "" {
            anyhow::bail!("empty path");
        } else if !path.starts_with('/') && opts.prefix_slash {
            anyhow::bail!("path must start with '/'");
        } else if path.starts_with('/') && !opts.prefix_slash {
            anyhow::bail!("path must not start with '/'");
        }

        // Ensure this is a valid url path.
        parse_url_path(path)?;

        let mut segments = vec![];

        let mut path = path;
        while path != "" {
            if opts.prefix_slash || segments.len() > 0 {
                path = &path[1..]; // drop leading slash
            }

            // Find the next path segment.
            let val = match path.find('/') {
                Some(0) => {
                    // Empty segment.
                    anyhow::bail!("invalid path: cannot contain empty path segment");
                }
                Some(idx) => {
                    // Non-empty segment.
                    let val = &path[..idx];
                    path = &path[idx..];
                    val
                }
                None => {
                    // Last segment.
                    let val = path;
                    path = "";
                    val
                }
            };

            let seg: Segment = match val.chars().next() {
                Some(':') => Segment::Param {
                    name: val[1..].to_string(),
                    value_type: ValueType::String,
                },
                Some('*') if opts.allow_wildcard => Segment::Wildcard {
                    name: val[1..].to_string(),
                },
                Some('!') if opts.allow_wildcard => Segment::Fallback {
                    name: val[1..].to_string(),
                },
                _ => Segment::Literal(val.to_string()),
            };

            segments.push(seg);
        }

        // Validate the segments.
        for (idx, seg) in segments.iter().enumerate() {
            match seg {
                Segment::Literal(lit) if lit == "" => {
                    anyhow::bail!("invalid path: literal cannot be empty");
                }
                Segment::Param { name, .. } if name == "" => {
                    anyhow::bail!("path parameters must have a name");
                }
                Segment::Wildcard { name } if name == "" => {
                    anyhow::bail!("path parameters must have a name");
                }
                Segment::Wildcard { .. } if idx != segments.len() - 1 => {
                    anyhow::bail!("path wildcards must be the last segment in the path");
                }
                Segment::Fallback { .. } if idx != segments.len() - 1 => {
                    anyhow::bail!("path fallbacks must be the last segment in the path");
                }
                _ => {}
            }
        }

        Ok(Path { segments })
    }
}

fn parse_url_path(path: &str) -> anyhow::Result<()> {
    // The url crate only supports parsing absolute urls, so use a dummy base
    // and ensure it is the same after parsing.
    let base = url::Url::parse("base://url.here")?;
    let url = url::Url::options()
        .base_url(Some(&base))
        .parse(path)
        .map_err(|err| anyhow::anyhow!("invalid path: {}", err))?;

    if url.scheme() != base.scheme() {
        anyhow::bail!("invalid path: cannot contain scheme")
    } else if url.authority() != base.authority() {
        anyhow::bail!("invalid path: cannot contain authority")
    }
    match url.host_str() {
        None => {
            // We should always have a host since the base url has one.
            anyhow::bail!("invalid path: cannot contain host")
        }
        Some(host) => {
            if host != base.host_str().unwrap() {
                anyhow::bail!("invalid path: cannot contain host")
            }
        }
    }

    if url.query().is_some() {
        anyhow::bail!("path must not contain query parameters (the '?' character)")
    } else if url.fragment().is_some() {
        anyhow::bail!("path must not contain url fragments (the '#' character)")
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse() {
        enum Result {
            Ok(Vec<Segment>),
            Err(String),
        }

        let tests = vec![
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
                    },
                    Segment::Wildcard {
                        name: "bar".to_string(),
                    },
                ]),
            ),
            (
                "/:foo/*",
                Err("path parameters must have a name".to_string()),
            ),
            (
                "/:foo/*/bar",
                Err("path parameters must have a name".to_string()),
            ),
            (
                "/:foo/*bar/baz",
                Err("path wildcards must be the last segment in the path".to_string()),
            ),
            (
                "/foo?bar=baz",
                Err("path must not contain query parameters (the '?' character)".to_string()),
            ),
            (
                "/foo#bar",
                Err("path must not contain url fragments (the '#' character)".to_string()),
            ),
            (
                "/foo/!fallback",
                Ok(vec![
                    Segment::Literal("foo".to_string()),
                    Segment::Fallback {
                        name: "fallback".to_string(),
                    },
                ]),
            ),
        ];

        for (path, want) in tests {
            let got = Path::parse(path, Default::default());
            match (got, want) {
                (Ok(got), Ok(want)) => {
                    assert_eq!(got.segments, want, "path {:?}", path);
                }
                (Err(got), Err(want)) => {
                    assert_eq!(got.to_string(), want, "path {:?}", path);
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
