package scrub

import (
	"slices"
	"strings"
)

// Cookie redacts the values of the named cookies in a Cookie request header
// value (e.g. "session=abc; theme=dark"), leaving the other cookies and the
// original formatting intact.
//
// Cookie names are matched case-sensitively, as RFC 6265 defines them.
// Every occurrence of a matching name is redacted. A part that cannot be
// parsed as a name=value pair is redacted as well, so that a malformed
// header can't hide a sensitive value.
func Cookie(header string, names []string, replaceWith string) string {
	if len(names) == 0 || header == "" {
		return header
	}

	var b strings.Builder
	b.Grow(len(header))
	for i, part := range strings.Split(header, ";") {
		if i > 0 {
			b.WriteByte(';')
		}

		name, _, hasEq := strings.Cut(part, "=")
		trimmedName := strings.TrimSpace(name)
		switch {
		case hasEq && slices.Contains(names, trimmedName):
			b.WriteString(name)
			b.WriteByte('=')
			b.WriteString(replaceWith)
		case !hasEq && strings.TrimSpace(part) != "":
			// Keep the leading whitespace so the result stays readable.
			b.WriteString(part[:len(part)-len(strings.TrimLeft(part, " \t"))])
			b.WriteString(replaceWith)
		default:
			b.WriteString(part)
		}
	}
	return b.String()
}
