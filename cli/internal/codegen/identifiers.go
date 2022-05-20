package codegen

import (
	"strings"
	"unicode"
)

type IdentFormat int

const (
	CamelCase          IdentFormat = iota // camelCase
	PascalCase                            // PascalCase
	SnakeCase                             // snake_case
	ScreamingSnakeCase                    // SCREAMING_SNAKE_CASE
	KebabCase                             // kebab-case
)

// convertIdentifierTo will take a Go identifier and convert it to the
// specified format.
func convertIdentifierTo(goIdentifier string, format IdentFormat) string {
	parts := parseIdentifier(goIdentifier)

	// Step 1: convert case
	for i, part := range parts {
		switch format {
		case CamelCase:
			if i == 0 {
				parts[i] = strings.ToLower(part)
			} else {
				parts[i] = strings.Title(part)
			}

		case PascalCase:
			parts[i] = strings.Title(part)

		case SnakeCase, KebabCase:
			parts[i] = strings.ToLower(part)

		case ScreamingSnakeCase:
			parts[i] = strings.ToUpper(part)
		}
	}

	// Step 2: Join Parts
	switch format {
	case CamelCase, PascalCase:
		return strings.Join(parts, "")
	case SnakeCase, ScreamingSnakeCase:
		return strings.Join(parts, "_")
	case KebabCase:
		return strings.Join(parts, "-")
	default:
		panic("unknown identifier format")
	}
}

// parseIdentifier parses a Go Identifier into the separate parts.
// which can then be recombined as needed.
func parseIdentifier(goIdentifier string) (parts []string) {
	if goIdentifier == "" {
		return nil
	}

	type runeType int
	const (
		other runeType = iota
		digit
		upper
		lower
	)

	runeToType := func(r rune) runeType {
		switch {
		case unicode.IsUpper(r):
			return upper
		case unicode.IsLower(r):
			return lower
		case unicode.IsDigit(r):
			return digit
		default:
			return other
		}
	}

	var str strings.Builder
	recordPart := func() {
		part := str.String()
		str.Reset()
		if part == "" {
			return
		}

		if !stringIsOnly(part, unicode.IsUpper) {
			// strings will always start with uppercase runes, so the if
			// the last uppercase rune is after index 0 but before the last
			// rune in the string, then we need to split it into two parts.
			//
			// i.e. "GetAPIDocs" => { "Get", "APIDocs" } => { "Get", "API", "Docs" }
			lastUpperCase := strings.LastIndexFunc(part, unicode.IsUpper)
			if lastUpperCase > 0 && lastUpperCase != len(part)-1 {
				parts = append(parts, part[:lastUpperCase])
				part = part[lastUpperCase:]
			}

			part = strings.ToLower(part)
		}

		parts = append(parts, part)
	}

	lastType := runeToType(rune(goIdentifier[0]))
	for _, r := range goIdentifier {
		runeType := runeToType(r)

		// If the type of rune has changed
		if lastType > runeType {
			recordPart()
		}
		lastType = runeType

		if runeType != other {
			str.WriteRune(r)
		}
	}

	recordPart()
	return
}

func stringIsOnly(str string, predicate func(r rune) bool) bool {
	for _, r := range str {
		if !predicate(r) {
			return false
		}
	}
	return true
}
