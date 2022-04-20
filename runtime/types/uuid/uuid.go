// Copyright (C) 2013-2018 by Maxim Bublis <b@codemonkey.ru>
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

// Package uuid provides implementations of the Universally Unique Identifier (UUID), as specified in RFC-4122 and DCE 1.1.
//
// RFC-4122[1] provides the specification for versions 1, 3, 4, and 5.
//
// DCE 1.1[2] provides the specification for version 2.
//
// [1] https://tools.ietf.org/html/rfc4122
// [2] http://pubs.opengroup.org/onlinepubs/9696989899/chap5.htm#tagcjh_08_02_01_01
package uuid

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Size of a UUID in bytes.
const Size = 16

// UUID is an array type to represent the value of a UUID, as defined in RFC-4122.
type UUID [Size]byte

// UUID versions.
const (
	_  byte = iota
	V1      // Version 1 (date-time and MAC address)
	V2      // Version 2 (date-time and MAC address, DCE security version)
	V3      // Version 3 (namespace name-based)
	V4      // Version 4 (random)
	V5      // Version 5 (namespace name-based)
)

// UUID layout variants.
const (
	variantNCS byte = iota
	variantRFC4122
	variantMicrosoft
	variantFuture
)

// String parse helpers.
var (
	urnPrefix  = []byte("urn:uuid:")
	byteGroups = []int{8, 4, 4, 4, 12}
)

// Nil is the nil UUID, as specified in RFC-4122, that has all 128 bits set to
// zero.
var Nil = UUID{}

// Version returns the algorithm version used to generate the UUID.
func (u UUID) Version() byte {
	return u[6] >> 4
}

// variant returns the UUID layout variant.
func (u UUID) variant() byte {
	switch {
	case (u[8] >> 7) == 0x00:
		return variantNCS
	case (u[8] >> 6) == 0x02:
		return variantRFC4122
	case (u[8] >> 5) == 0x06:
		return variantMicrosoft
	case (u[8] >> 5) == 0x07:
		fallthrough
	default:
		return variantFuture
	}
}

// Bytes returns a byte slice representation of the UUID.
func (u UUID) Bytes() []byte {
	return u[:]
}

// String returns a canonical RFC-4122 string representation of the UUID:
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func (u UUID) String() string {
	buf := make([]byte, 36)

	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], u[10:])

	return string(buf)
}

// Format implements fmt.Formatter for UUID values.
//
// The behavior is as follows:
// The 'x' and 'X' verbs output only the hex digits of the UUID, using a-f for 'x' and A-F for 'X'.
// The 'v', '+v', 's' and 'q' verbs return the canonical RFC-4122 string representation.
// The 'S' verb returns the RFC-4122 format, but with capital hex digits.
// The '#v' verb returns the "Go syntax" representation, which is a 16 byte array initializer.
// All other verbs not handled directly by the fmt package (like '%p') are unsupported and will return
// "%!verb(uuid.UUID=value)" as recommended by the fmt package.
func (u UUID) Format(f fmt.State, c rune) {
	switch c {
	case 'x', 'X':
		s := hex.EncodeToString(u.Bytes())
		if c == 'X' {
			s = strings.Map(toCapitalHexDigits, s)
		}
		_, _ = io.WriteString(f, s)
	case 'v':
		var s string
		if f.Flag('#') {
			s = fmt.Sprintf("%#v", [Size]byte(u))
		} else {
			s = u.String()
		}
		_, _ = io.WriteString(f, s)
	case 's', 'S':
		s := u.String()
		if c == 'S' {
			s = strings.Map(toCapitalHexDigits, s)
		}
		_, _ = io.WriteString(f, s)
	case 'q':
		_, _ = io.WriteString(f, `"`+u.String()+`"`)
	default:
		// invalid/unsupported format verb
		fmt.Fprintf(f, "%%!%c(uuid.UUID=%s)", c, u.String())
	}
}

func toCapitalHexDigits(ch rune) rune {
	// convert a-f hex digits to A-F
	switch ch {
	case 'a':
		return 'A'
	case 'b':
		return 'B'
	case 'c':
		return 'C'
	case 'd':
		return 'D'
	case 'e':
		return 'E'
	case 'f':
		return 'F'
	default:
		return ch
	}
}

// setVersion sets the version bits.
func (u *UUID) SetVersion(v byte) {
	u[6] = (u[6] & 0x0f) | (v << 4)
}

// setVariant sets the variant bits.
func (u *UUID) setVariant(v byte) {
	switch v {
	case variantNCS:
		u[8] = (u[8]&(0xff>>1) | (0x00 << 7))
	case variantRFC4122:
		u[8] = (u[8]&(0xff>>2) | (0x02 << 6))
	case variantMicrosoft:
		u[8] = (u[8]&(0xff>>3) | (0x06 << 5))
	case variantFuture:
		fallthrough
	default:
		u[8] = (u[8]&(0xff>>3) | (0x07 << 5))
	}
}

// Must is a helper that wraps a call to a function returning (UUID, error)
// and panics if the error is non-nil. It is intended for use in variable
// initializations such as
//  var packageUUID = uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426655440000"))
func Must(u UUID, err error) UUID {
	if err != nil {
		panic(err)
	}
	return u
}
