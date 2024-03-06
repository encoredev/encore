package directive

import (
	"go/token"
	"unicode"
	"unicode/utf8"
)

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Fields splits the string s around each instance of one or more consecutive white space
// characters, as defined by unicode.IsSpace, returning a slice of substrings of s or an
// empty slice if s contains only white space.
func fields(startPos token.Pos, s string) []Field {
	// First count the fields.
	// This is an exact count if s is ASCII, otherwise it is an approximation.
	n := 0
	wasSpace := 1
	// setBits is used to track which bits are set in the bytes of s.
	setBits := uint8(0)
	for i := 0; i < len(s); i++ {
		r := s[i]
		setBits |= r
		isSpace := int(asciiSpace[r])
		n += wasSpace & ^isSpace
		wasSpace = isSpace
	}

	if setBits >= utf8.RuneSelf {
		// Some runes in the input string are not ASCII.
		return fieldsFunc(startPos, s, unicode.IsSpace)
	}
	// ASCII fast path
	a := make([]Field, n)
	na := 0
	fieldStart := 0
	i := 0
	// Skip spaces in the front of the input.
	for i < len(s) && asciiSpace[s[i]] != 0 {
		i++
	}
	fieldStart = i
	inQuote := false
	escaped := false
	for i < len(s) {
		if !escaped {
			switch {
			// check if we enter a quoted field which is always the characters ="
			case !inQuote && s[i] == '=' && i+1 < len(s) && s[i+1] == '"':
				inQuote = true

			// check if we exit a quoted field which is always the character " followed by a space
			case inQuote && s[i] == '"' && i+1 < len(s) && asciiSpace[s[i+1]] == 1:
				inQuote = false

			// check if we're about to escape something
			case inQuote && s[i] == '\\':
				escaped = true
			}
		} else {
			escaped = false
		}

		if inQuote || asciiSpace[s[i]] == 0 {
			i++
			continue
		}
		a[na] = Field{
			Value: s[fieldStart:i],
			start: startPos + token.Pos(fieldStart),
			end:   startPos + token.Pos(i),
		}
		na++
		i++
		// Skip spaces in between fields.
		for i < len(s) && asciiSpace[s[i]] != 0 {
			i++
		}
		fieldStart = i
	}
	if fieldStart < len(s) { // Last field might end at EOF.
		a[na] = Field{
			Value: s[fieldStart:],
			start: startPos + token.Pos(fieldStart),
			end:   startPos + token.Pos(len(s)),
		}
	}
	return a
}

// FieldsFunc splits the string s at each run of Unicode code points c satisfying f(c)
// and returns an array of slices of s. If all code points in s satisfy f(c) or the
// string is empty, an empty slice is returned.
//
// FieldsFunc makes no guarantees about the order in which it calls f(c)
// and assumes that f always returns the same value for a given c.
func fieldsFunc(startPos token.Pos, s string, f func(rune) bool) []Field {
	// A span is used to record a slice of s of the form s[start:end].
	// The start index is inclusive and the end index is exclusive.
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 32)

	// Find the field start and end indices.
	// Doing this in a separate pass (rather than slicing the string s
	// and collecting the result substrings right away) is significantly
	// more efficient, possibly due to cache effects.
	start := -1 // valid span start if >= 0
	for end, rune := range s {
		if f(rune) {
			if start >= 0 {
				spans = append(spans, span{start, end})
				// Set start to a negative value.
				// Note: using -1 here consistently and reproducibly
				// slows down this code by a several percent on amd64.
				start = ^start
			}
		} else {
			if start < 0 {
				start = end
			}
		}
	}

	// Last field might end at EOF.
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// Create strings from recorded field indices.
	a := make([]Field, len(spans))
	for i, span := range spans {
		a[i] = Field{
			Value: s[span.start:span.end],
			start: startPos + token.Pos(len([]byte(s[:span.start]))),
		}
		a[i].end = a[i].start + token.Pos(len([]byte(a[i].Value)))
	}

	return a
}
