/// Glob pattern matching for Redis KEYS, SCAN, PSUBSCRIBE etc.
///
/// Supports *, ?, [abc], [a-z], [^a], \ escape.
///
/// Simple glob matching: supports *, ?, [abc], [a-z], [^a].
pub fn glob_match(pattern: &str, text: &str) -> bool {
    let pat = pattern.as_bytes();
    let txt = text.as_bytes();
    glob_match_inner(pat, txt)
}

fn glob_match_inner(pat: &[u8], txt: &[u8]) -> bool {
    let mut pi = 0;
    let mut ti = 0;
    let mut star_pi = usize::MAX;
    let mut star_ti = 0;

    while ti < txt.len() {
        if pi < pat.len() && (pat[pi] == b'?' || pat[pi] == txt[ti]) {
            pi += 1;
            ti += 1;
        } else if pi < pat.len() && pat[pi] == b'*' {
            star_pi = pi;
            star_ti = ti;
            pi += 1;
        } else if pi < pat.len() && pat[pi] == b'[' {
            let (matched, end) = match_char_class(&pat[pi..], txt[ti]);
            if matched {
                pi += end;
                ti += 1;
            } else if star_pi != usize::MAX {
                pi = star_pi + 1;
                star_ti += 1;
                ti = star_ti;
            } else {
                return false;
            }
        } else if pi < pat.len() && pat[pi] == b'\\' && pi + 1 < pat.len() {
            pi += 1;
            if pat[pi] == txt[ti] {
                pi += 1;
                ti += 1;
            } else if star_pi != usize::MAX {
                pi = star_pi + 1;
                star_ti += 1;
                ti = star_ti;
            } else {
                return false;
            }
        } else if star_pi != usize::MAX {
            pi = star_pi + 1;
            star_ti += 1;
            ti = star_ti;
        } else {
            return false;
        }
    }

    while pi < pat.len() && pat[pi] == b'*' {
        pi += 1;
    }

    pi == pat.len()
}

/// Match a character class like [abc] or [a-z] or [^a]. Returns (matched, bytes consumed).
fn match_char_class(pat: &[u8], ch: u8) -> (bool, usize) {
    if pat.is_empty() || pat[0] != b'[' {
        return (false, 0);
    }

    let mut i = 1;
    let negate = if i < pat.len() && pat[i] == b'^' {
        i += 1;
        true
    } else {
        false
    };

    let mut matched = false;
    while i < pat.len() && pat[i] != b']' {
        if i + 2 < pat.len() && pat[i + 1] == b'-' {
            if ch >= pat[i] && ch <= pat[i + 2] {
                matched = true;
            }
            i += 3;
        } else {
            if pat[i] == ch {
                matched = true;
            }
            i += 1;
        }
    }

    if i < pat.len() && pat[i] == b']' {
        i += 1;
    }

    if negate {
        matched = !matched;
    }

    (matched, i)
}

/// Filter a list of strings by a glob pattern. Used by SSCAN, HSCAN etc.
pub fn match_keys_vec(keys: &[String], pattern: &str) -> Vec<String> {
    if pattern == "*" {
        return keys.to_vec();
    }
    keys.iter()
        .filter(|k| glob_match(pattern, k))
        .cloned()
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_glob_match_basic() {
        assert!(glob_match("*", "anything"));
        assert!(glob_match("hello", "hello"));
        assert!(!glob_match("hello", "world"));
        assert!(glob_match("h?llo", "hello"));
        assert!(glob_match("h*o", "hello"));
        assert!(glob_match("h[ae]llo", "hello"));
        assert!(!glob_match("h[ae]llo", "hillo"));
    }

    #[test]
    fn test_glob_match_patterns() {
        assert!(glob_match("event*", "event123"));
        assert!(glob_match("event*", "event"));
        assert!(!glob_match("event*", "other"));
        assert!(glob_match("*news*", "the-news-today"));
        assert!(glob_match("h[a-z]llo", "hello"));
    }
}
