package ecl

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type lexer struct {
	src    *sourceFile
	offset int
	line   int // 1-based
	col    int // 1-based, counted in runes
	diags  *ErrorList
}

func newLexer(src *sourceFile, diags *ErrorList) *lexer {
	return &lexer{src: src, line: 1, col: 1, diags: diags}
}

// lex tokenizes the entire file. Runs of newlines collapse into a single
// newline token, and newlines inside [...] lists are suppressed so that
// lists may span lines.
func (lx *lexer) lex() []token {
	var toks []token
	bracketDepth := 0
	for {
		t := lx.next()
		switch t.kind {
		case tokNewline:
			if bracketDepth > 0 {
				continue
			}
			if len(toks) == 0 || toks[len(toks)-1].kind == tokNewline {
				continue
			}
		case tokLBracket:
			bracketDepth++
		case tokRBracket:
			if bracketDepth > 0 {
				bracketDepth--
			}
		}
		toks = append(toks, t)
		if t.kind == tokEOF {
			return toks
		}
	}
}

func (lx *lexer) pos() Position {
	return Position{File: lx.src.name, Offset: lx.offset, Line: lx.line, Column: lx.col}
}

func (lx *lexer) eof() bool { return lx.offset >= len(lx.src.src) }

func (lx *lexer) peek() byte {
	if lx.eof() {
		return 0
	}
	return lx.src.src[lx.offset]
}

func (lx *lexer) peekAt(n int) byte {
	if lx.offset+n >= len(lx.src.src) {
		return 0
	}
	return lx.src.src[lx.offset+n]
}

// advance consumes one rune.
func (lx *lexer) advance() rune {
	r, size := utf8.DecodeRuneInString(lx.src.src[lx.offset:])
	lx.offset += size
	if r == '\n' {
		lx.line++
		lx.col = 1
	} else {
		lx.col++
	}
	return r
}

func (lx *lexer) errorf(start, end Position, format string, args ...any) {
	lx.diags.addf(lx.src, start, end, format, args...)
}

func (lx *lexer) next() token {
	lx.skipSpaceAndComments()
	start := lx.pos()
	if lx.eof() {
		return token{kind: tokEOF, pos: start, end: start}
	}

	mk := func(kind tokenKind) token {
		end := lx.pos()
		return token{kind: kind, pos: start, end: end, text: lx.src.src[start.Offset:end.Offset]}
	}

	c := lx.peek()
	switch {
	case c == '\n':
		lx.advance()
		return mk(tokNewline)
	case c == '"':
		return lx.lexString()
	case c >= '0' && c <= '9':
		return lx.lexNumber()
	case isIdentStart(rune(c)):
		return lx.lexIdent()
	}

	lx.advance()
	switch c {
	case '{':
		return mk(tokLBrace)
	case '}':
		return mk(tokRBrace)
	case '[':
		return mk(tokLBracket)
	case ']':
		return mk(tokRBracket)
	case ',':
		return mk(tokComma)
	case ':':
		return mk(tokColon)
	case '.':
		return mk(tokDot)
	case '-':
		return mk(tokMinus)
	case '=':
		if lx.peek() == '=' {
			lx.advance()
			return mk(tokEq)
		}
		return mk(tokAssign)
	case '!':
		if lx.peek() == '=' {
			lx.advance()
			return mk(tokNeq)
		}
		lx.errorf(start, lx.pos(), "unexpected '!'; use '!=' for inequality")
		return lx.next()
	case '>':
		if lx.peek() == '=' {
			lx.advance()
			return mk(tokGe)
		}
		return mk(tokGt)
	case '<':
		if lx.peek() == '=' {
			lx.advance()
			return mk(tokLe)
		}
		return mk(tokLt)
	case '&':
		if lx.peek() == '&' {
			lx.advance()
			return mk(tokAndAnd)
		}
		return mk(tokAmp)
	case '|':
		if lx.peek() == '|' {
			lx.advance()
			return mk(tokOrOr)
		}
		return mk(tokPipe)
	}

	lx.errorf(start, lx.pos(), "unexpected character %q", rune(c))
	return lx.next()
}

// skipSpaceAndComments skips spaces, tabs, carriage returns, and comments.
// Newlines are significant and not skipped. Block comments are treated as
// plain whitespace, even when they span lines.
func (lx *lexer) skipSpaceAndComments() {
	for !lx.eof() {
		switch c := lx.peek(); {
		case c == ' ' || c == '\t' || c == '\r':
			lx.advance()
		case c == '/' && lx.peekAt(1) == '/':
			for !lx.eof() && lx.peek() != '\n' {
				lx.advance()
			}
		case c == '/' && lx.peekAt(1) == '*':
			start := lx.pos()
			lx.advance()
			lx.advance()
			closed := false
			for !lx.eof() {
				if lx.peek() == '*' && lx.peekAt(1) == '/' {
					lx.advance()
					lx.advance()
					closed = true
					break
				}
				lx.advance()
			}
			if !closed {
				end := start
				end.Column += 2
				end.Offset += 2
				lx.errorf(start, end, "unterminated block comment")
			}
		default:
			return
		}
	}
}

func (lx *lexer) lexString() token {
	start := lx.pos()
	lx.advance() // opening quote
	var b strings.Builder
	for {
		if lx.eof() || lx.peek() == '\n' {
			end := start
			end.Column++
			end.Offset++
			lx.errorf(start, end, "unterminated string literal")
			break
		}
		r := lx.advance()
		if r == '"' {
			break
		}
		if r == '\\' {
			if lx.eof() || lx.peek() == '\n' {
				continue
			}
			escPos := lx.pos()
			escPos.Column-- // point at the backslash
			escPos.Offset--
			e := lx.advance()
			switch e {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				lx.errorf(escPos, lx.pos(), "invalid escape sequence '\\%c' in string literal", e)
				b.WriteRune(e)
			}
			continue
		}
		b.WriteRune(r)
	}
	end := lx.pos()
	return token{
		kind: tokString,
		pos:  start,
		end:  end,
		text: lx.src.src[start.Offset:end.Offset],
		str:  b.String(),
	}
}

func (lx *lexer) lexNumber() token {
	start := lx.pos()
	for !lx.eof() && isDigit(lx.peek()) {
		lx.advance()
	}
	if lx.peek() == '.' && isDigit(lx.peekAt(1)) {
		lx.advance()
		for !lx.eof() && isDigit(lx.peek()) {
			lx.advance()
		}
	}
	numEnd := lx.offset

	// Unit suffix: a run of letters immediately following the number,
	// e.g. "512Mi" or "30d". Validated by the parser.
	for !lx.eof() && isLetter(lx.peek()) {
		lx.advance()
	}
	end := lx.pos()

	text := lx.src.src[start.Offset:end.Offset]
	numText := lx.src.src[start.Offset:numEnd]
	num, err := strconv.ParseFloat(numText, 64)
	if err != nil {
		lx.errorf(start, end, "invalid number %q", numText)
	}
	return token{
		kind: tokNumber,
		pos:  start,
		end:  end,
		text: text,
		num:  num,
		unit: lx.src.src[numEnd:end.Offset],
	}
}

func (lx *lexer) lexIdent() token {
	start := lx.pos()
	for !lx.eof() && isIdentPart(rune(lx.peek())) {
		lx.advance()
	}
	end := lx.pos()
	name := lx.src.src[start.Offset:end.Offset]
	kind := tokIdent
	if kw, ok := keywords[name]; ok {
		kind = kw
	}
	return token{kind: kind, pos: start, end: end, text: name, str: name}
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// addf appends a diagnostic to the list.
func (l *ErrorList) addf(src *sourceFile, start, end Position, format string, args ...any) *Diagnostic {
	d := &Diagnostic{
		Pos:     start,
		End:     end,
		Message: sprintf(format, args...),
		src:     src,
	}
	*l = append(*l, d)
	return d
}
