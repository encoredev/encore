package ecl

import (
	"fmt"
	"strconv"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNewline
	tokIdent
	tokString
	tokNumber

	tokLBrace   // {
	tokRBrace   // }
	tokLBracket // [
	tokRBracket // ]
	tokComma    // ,
	tokColon    // :
	tokDot      // .
	tokMinus    // -

	tokEq     // ==
	tokNeq    // !=
	tokGe     // >=
	tokLe     // <=
	tokGt     // >
	tokLt     // <
	tokAndAnd // &&
	tokAmp    // &
	tokOrOr   // ||
	tokPipe   // |
	tokAssign // = (always an error; kept so the parser can suggest '==')

	tokFor
	tokWhere
	tokIf
	tokIn
	tokExists
	tokRequired
	tokDefault
	tokImport
	tokTrue
	tokFalse
)

// Note: "version" is intentionally not a keyword. The version declaration
// at the top of a file is recognized contextually, so that "version" stays
// usable as a property name (e.g. `version: "16"` in a resource block).
var keywords = map[string]tokenKind{
	"for":      tokFor,
	"where":    tokWhere,
	"if":       tokIf,
	"in":       tokIn,
	"exists":   tokExists,
	"required": tokRequired,
	"default":  tokDefault,
	"import":   tokImport,
	"true":     tokTrue,
	"false":    tokFalse,
}

var tokenNames = map[tokenKind]string{
	tokEOF:      "end of file",
	tokNewline:  "newline",
	tokIdent:    "identifier",
	tokString:   "string",
	tokNumber:   "number",
	tokLBrace:   "'{'",
	tokRBrace:   "'}'",
	tokLBracket: "'['",
	tokRBracket: "']'",
	tokComma:    "','",
	tokColon:    "':'",
	tokDot:      "'.'",
	tokMinus:    "'-'",
	tokEq:       "'=='",
	tokNeq:      "'!='",
	tokGe:       "'>='",
	tokLe:       "'<='",
	tokGt:       "'>'",
	tokLt:       "'<'",
	tokAndAnd:   "'&&'",
	tokAmp:      "'&'",
	tokOrOr:     "'||'",
	tokPipe:     "'|'",
	tokAssign:   "'='",
	tokFor:      "keyword 'for'",
	tokWhere:    "keyword 'where'",
	tokIf:       "keyword 'if'",
	tokIn:       "keyword 'in'",
	tokExists:   "keyword 'exists'",
	tokRequired: "keyword 'required'",
	tokDefault:  "keyword 'default'",
	tokImport:   "keyword 'import'",
	tokTrue:     "'true'",
	tokFalse:    "'false'",
}

func (k tokenKind) String() string {
	if s, ok := tokenNames[k]; ok {
		return s
	}
	return fmt.Sprintf("token(%d)", int(k))
}

func (k tokenKind) isKeyword() bool { return k >= tokFor && k <= tokFalse }

type token struct {
	kind tokenKind
	pos  Position // start of the token
	end  Position // position just past the token
	text string   // raw source text

	num  float64 // numeric value for tokNumber
	unit string  // unit suffix for tokNumber ("" if none)
	str  string  // decoded value for tokString, name for tokIdent
}

// describe renders a token for use in error messages, e.g. "identifier 'cpu'".
func (t token) describe() string {
	switch t.kind {
	case tokIdent:
		return fmt.Sprintf("identifier '%s'", t.str)
	case tokString:
		return fmt.Sprintf("string %s", strconv.Quote(t.str))
	case tokNumber:
		return fmt.Sprintf("number '%s'", t.text)
	default:
		return t.kind.String()
	}
}

func (t token) span() Span { return Span{Start: t.pos, End: t.end} }
