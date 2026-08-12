package gcsemu

import (
	"fmt"
	"regexp"
	"strings"
)

// compileGlob translates a GCS matchGlob pattern into an anchored regular expression.
//
// The supported syntax mirrors the Cloud Storage matchGlob parameter:
//
//	"?"      matches exactly one character other than '/'
//	"*"      matches any number of characters within a path segment (never '/')
//	"**"     matches any number of characters, including '/'
//	"[abc]"  matches one character from the set; ranges (a-z) and negation ([!abc], [^abc]) are supported
//	"{a,b}"  matches any one of the comma-separated alternatives, and may nest.
//	         As Cloud Storage requires, it may contain neither "/" nor "**".
//	"\x"     matches the literal character x
//
// Use EscapeGlobLiteral to splice a literal object key into a pattern.
func compileGlob(pattern string) (*regexp.Regexp, error) {
	pat := []rune(pattern)

	var sb strings.Builder
	sb.WriteString("^")

	depth := 0

	// atBoundary reports whether the pattern is at a directory boundary, which Cloud
	// Storage defines as "the beginning of the pattern and immediately after each
	// slash". Only "**/" appearing at one may collapse to nothing.
	atBoundary := true

	for i := 0; i < len(pat); i++ {
		// nextAtBoundary is whether what follows this construct sits at a boundary;
		// only a literal '/' puts it there.
		nextAtBoundary := false

		switch c := pat[i]; c {
		case '\\':
			if i+1 >= len(pat) {
				return nil, fmt.Errorf("glob %q: trailing backslash", pattern)
			}
			i++
			sb.WriteString(regexp.QuoteMeta(string(pat[i])))
			// An escaped character carries no structural role, so an escaped slash
			// opens no directory boundary — matching the way it doesn't count as one
			// for the brace restrictions either.

		case '*':
			// "**" crosses path segments, a single "*" does not.
			if i+1 < len(pat) && pat[i+1] == '*' {
				if depth > 0 {
					return nil, fmt.Errorf("glob %q: brace expansion must not contain '**'", pattern)
				}
				i++
				if atBoundary && i+1 < len(pat) && pat[i+1] == '/' {
					// "**/" at a directory boundary matches zero characters too, so
					// "a/**/b" matches "a/b" as well as "a/x/b".
					i++
					sb.WriteString("(?:.*/)?")
					nextAtBoundary = true
				} else {
					sb.WriteString(".*")
				}
			} else {
				sb.WriteString("[^/]*")
			}

		case '?':
			sb.WriteString("[^/]")

		case '[':
			end, expr, err := globCharClass(pattern, pat, i)
			if err != nil {
				return nil, err
			}
			sb.WriteString(expr)
			i = end

		case '{':
			depth++
			sb.WriteString("(?:")

		case '}':
			if depth == 0 {
				return nil, fmt.Errorf("glob %q: unmatched '}'", pattern)
			}
			depth--
			sb.WriteString(")")

		case ',':
			// Only a separator inside an alternation; a literal comma elsewhere.
			if depth > 0 {
				sb.WriteString("|")
			} else {
				sb.WriteString(",")
			}

		default:
			if c == '/' && depth > 0 {
				return nil, fmt.Errorf("glob %q: brace expansion must not contain '/'", pattern)
			}
			sb.WriteString(regexp.QuoteMeta(string(c)))
			nextAtBoundary = c == '/'
		}

		atBoundary = nextAtBoundary
	}

	if depth != 0 {
		return nil, fmt.Errorf("glob %q: unmatched '{'", pattern)
	}

	sb.WriteString("$")
	return regexp.Compile(sb.String())
}

// globCharClass translates the character class starting at pat[start] (which must be '[').
// It returns the index of the closing ']' along with the equivalent regexp expression.
func globCharClass(pattern string, pat []rune, start int) (end int, expr string, err error) {
	i := start + 1

	var sb strings.Builder
	sb.WriteRune('[')

	negated := i < len(pat) && (pat[i] == '!' || pat[i] == '^')
	if negated {
		sb.WriteRune('^')
		i++
	}

	// A ']' in the first position is a literal rather than the terminator.
	if i < len(pat) && pat[i] == ']' {
		sb.WriteString(`\]`)
		i++
	}

	for ; i < len(pat) && pat[i] != ']'; i++ {
		if pat[i] == '\\' {
			if i+1 >= len(pat) {
				return 0, "", fmt.Errorf("glob %q: trailing backslash", pattern)
			}
			i++
		} else if pat[i] == '-' {
			// Range separator; pass through unescaped.
			sb.WriteRune('-')
			continue
		}
		sb.WriteString(escapeCharClassRune(pat[i]))
	}

	if i >= len(pat) {
		return 0, "", fmt.Errorf("glob %q: unterminated '['", pattern)
	}

	// Character classes never match the path separator.
	if negated {
		sb.WriteRune('/')
	}

	sb.WriteRune(']')
	return i, sb.String(), nil
}

func escapeCharClassRune(c rune) string {
	switch c {
	case '\\', ']', '^', '-', '[':
		return `\` + string(c)
	}
	return string(c)
}

// globMeta is the set of characters an escape may precede, which Cloud Storage
// documents as exactly "?, *, \, [, ], { and }".
//
// A comma is deliberately absent: it separates alternatives only inside braces, and
// since EscapeGlobLiteral escapes every brace, a comma in escaped text can never sit
// inside one. Escaping it anyway would be an escape sequence Cloud Storage doesn't
// define.
const globMeta = `\*?[]{}`

// EscapeGlobLiteral escapes every character compileGlob reads as pattern syntax, so
// that a literal object key (or key prefix) can be spliced into a glob pattern.
//
// Object keys are opaque strings and routinely contain glob metacharacters, so a
// caller scoping a search to the folder "a[1]/" has to escape that prefix before
// concatenating a pattern onto it. Unescaped, the prefix is read as a pattern in its
// own right and the search silently matches nothing.
func EscapeGlobLiteral(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, c := range s {
		if strings.ContainsRune(globMeta, c) {
			sb.WriteByte('\\')
		}
		sb.WriteRune(c)
	}
	return sb.String()
}
