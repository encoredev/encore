package scrub

import "io"

func newScanner(r io.ByteReader) *scanner {
	return &scanner{r: r}
}

type scanner struct {
	r          io.ByteReader
	it         scanItem
	err        error // first error encountered; EOF on successful completion
	pos        int64
	lastRead   byte
	unreadByte byte // unread byte in storage, or 0 if none.

	// stack keeps track of the stack of objects/arrays we've started
	// but not yet completed parsing. It's used to determine whether
	// a value encountered in parsing an object is a key or a value.
	stack []scanState
}

// Next advances the scanner to the next token.
// It reports false when the whole input stream has been consumed,
// or when reading it encountered an error.
func (s *scanner) Next() bool {
	s.it = s.scan()
	return s.err == nil || s.it.tok != 0
}

func (s *scanner) Item() scanItem {
	return s.it
}

// Err reports the first error encountered during scanning,
// or nil if no errors were encountered.
//
// When the underlying reader reports io.EOF this is not reported as an
// error but as the successful completion of scanning, so Err reports nil.
func (s *scanner) Err() error {
	if s.err == io.EOF {
		return nil
	}
	return s.err
}

// scan consumes a single scan item.
func (s *scanner) scan() scanItem {
	it, preContext, postContext := s.scanOne()

	// If we're inside an object, update the key detection
	if n := len(s.stack); n > 0 {
		if st := &s.stack[n-1]; st.tok == objectBegin {
			switch it.tok {
			case objectBegin, arrayBegin, literal:
				st.isKey = !st.isKey
			}
			if postContext == ':' {
				st.isKey = true
			} else if preContext == ':' {
				st.isKey = false
			}

			it.isMapKey = st.isKey
		}
	}

	switch it.tok {
	case objectBegin, arrayBegin:
		s.stack = append(s.stack, scanState{tok: it.tok})
	case objectEnd, arrayEnd:
		// Pop the last entry off the stack, if we have one.
		// Assume begin/end are balanced; it's not valid JSON if they're not
		// and this is just a best-effort approach anyway.
		if n := len(s.stack); n > 0 {
			s.stack = s.stack[:n-1]
		}
	}

	return it
}

// scanOne scans a single item.
// The extraContext reports whether the scanning encountered a
// newline, comma, or colon in the process.
func (s *scanner) scanOne() (it scanItem, preContext, postContext byte) {
	for {
		c := s.readToken()
		postContext = s.peekToken()
		switch c {
		case '"':
			it = s.scanString()
			return
		case '{':
			it = scanItem{from: s.pos - 1, to: s.pos, tok: objectBegin}
			return
		case '}':
			it = scanItem{from: s.pos - 1, to: s.pos, tok: objectEnd}
			return
		case '[':
			it = scanItem{from: s.pos - 1, to: s.pos, tok: arrayBegin}
			return
		case ']':
			it = scanItem{from: s.pos - 1, to: s.pos, tok: arrayEnd}
			return
		case ':', ',':
			preContext = c
			continue
		case 0:
			return
		default:
			it = s.scanLiteral()
			return
		}
	}
}

func (s *scanner) scanString() scanItem {
	// Use s.pos-1 since scan already consumed the first byte.
	it := scanItem{from: s.pos - 1, tok: literal}

	for {
		b := s.readByte()

		if b == '\\' {
			b = s.readByte()
			// Escaped symbols never signal the end of the string,
			// so it's safe to just continue here regardless of value.
			continue
		}

		switch b {
		case '"':
			it.to = s.pos
			return it
		case '\n', '\r':
			// newline or line feed, treat this as the end of the string.
			s.unread()
			it.to = s.pos
			return it
		case 0:
			// end of input
			it.to = s.pos
			return it
		}
	}
}

func (s *scanner) scanLiteral() scanItem {
	// Use s.pos-1 since scan already consumed the first byte.
	it := scanItem{from: s.pos - 1, tok: literal}
	for {
		b := s.readByte()
		switch b {
		case '"', ' ', '\r', '\n', '\t', ',', ':', '{', '[', ']', '}':
			// new token to follow
			s.unread()
			it.to = s.pos
			return it
		case 0:
			// end of input
			it.to = s.pos
			return it
		}
	}
}

// readToken returns the next token in the input.
// If there is no more data it reports 0.
func (s *scanner) readToken() byte {
	for {
		b := s.readByte()
		// Ignore whitespace.
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return b
	}
}

// peekToken peeks at the next token.
// It is equivalent to read() followed by unread().
func (s *scanner) peekToken() byte {
	b := s.readToken()
	if b != 0 {
		s.unread()
	}
	return b
}

// readByte reads a single byte and returns it.
// If there is no more data, it reports 0.
func (s *scanner) readByte() byte {
	// Do we have an unread byte?
	var b byte
	if b = s.unreadByte; b != 0 {
		s.lastRead = b
		s.unreadByte = 0
		s.pos++
		return b
	}

	if s.err != nil {
		return 0
	}

	b, s.err = s.r.ReadByte()
	if s.err == nil {
		s.lastRead = b
		s.pos++
	}
	return b
}

func (s *scanner) unread() {
	if s.unreadByte != 0 {
		panic("cannot unread multiple bytes in a row")
	}
	s.unreadByte = s.lastRead
	s.pos--
}

type scanItem struct {
	from, to int64
	tok      token
	isMapKey bool
}

func (it scanItem) Bounds() Bounds {
	return Bounds{
		From: int(it.from),
		To:   int(it.to),
	}
}

// token represents a single
type token uint8

//go:generate stringer -type=token
const (
	unknown token = iota

	objectBegin // {
	objectEnd   // }
	arrayBegin  // [
	arrayEnd    // ]
	literal
)

type scanState struct {
	tok   token
	isKey bool
}
