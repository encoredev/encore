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

package uuid

import (
	"bytes"
	"fmt"
	"testing"
)

// Predefined namespace UUIDs.
var (
	namespaceDNS  = Must(FromString("6ba7b810-9dad-11d1-80b4-00c04fd430c8"))
	namespaceURL  = Must(FromString("6ba7b811-9dad-11d1-80b4-00c04fd430c8"))
	namespaceOID  = Must(FromString("6ba7b812-9dad-11d1-80b4-00c04fd430c8"))
	namespaceX500 = Must(FromString("6ba7b814-9dad-11d1-80b4-00c04fd430c8"))
)

func TestUUID(t *testing.T) {
	t.Run("Bytes", testUUIDBytes)
	t.Run("String", testUUIDString)
	t.Run("Version", testUUIDVersion)
	t.Run("Variant", testUUIDVariant)
	t.Run("SetVersion", testUUIDSetVersion)
	t.Run("SetVariant", testUUIDSetVariant)
	t.Run("Format", testUUIDFormat)
}

func testUUIDBytes(t *testing.T) {
	got := codecTestUUID.Bytes()
	want := codecTestData
	if !bytes.Equal(got, want) {
		t.Errorf("%v.Bytes() = %x, want %x", codecTestUUID, got, want)
	}
}

func testUUIDString(t *testing.T) {
	got := namespaceDNS.String()
	want := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	if got != want {
		t.Errorf("%v.String() = %q, want %q", namespaceDNS, got, want)
	}
}

func testUUIDVersion(t *testing.T) {
	u := UUID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if got, want := u.Version(), V1; got != want {
		t.Errorf("%v.Version() == %d, want %d", u, got, want)
	}
}

func testUUIDVariant(t *testing.T) {
	tests := []struct {
		u    UUID
		want byte
	}{
		{
			u:    UUID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: variantNCS,
		},
		{
			u:    UUID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: variantRFC4122,
		},
		{
			u:    UUID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: variantMicrosoft,
		},
		{
			u:    UUID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xe0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: variantFuture,
		},
	}
	for _, tt := range tests {
		if got := tt.u.variant(); got != tt.want {
			t.Errorf("%v.variant() == %d, want %d", tt.u, got, tt.want)
		}
	}
}

func testUUIDSetVersion(t *testing.T) {
	u := UUID{}
	want := V4
	u.SetVersion(want)
	if got := u.Version(); got != want {
		t.Errorf("%v.Version() == %d after setVersion(%d)", u, got, want)
	}
}

func testUUIDSetVariant(t *testing.T) {
	variants := []byte{
		variantNCS,
		variantRFC4122,
		variantMicrosoft,
		variantFuture,
	}
	for _, want := range variants {
		u := UUID{}
		u.setVariant(want)
		if got := u.variant(); got != want {
			t.Errorf("%v.variant() == %d after SetVariant(%d)", u, got, want)
		}
	}
}

func testUUIDFormat(t *testing.T) {
	val := Must(FromString("12345678-90ab-cdef-1234-567890abcdef"))
	tests := []struct {
		u    UUID
		f    string
		want string
	}{
		{u: val, f: "%s", want: "12345678-90ab-cdef-1234-567890abcdef"},
		{u: val, f: "%S", want: "12345678-90AB-CDEF-1234-567890ABCDEF"},
		{u: val, f: "%q", want: `"12345678-90ab-cdef-1234-567890abcdef"`},
		{u: val, f: "%x", want: "1234567890abcdef1234567890abcdef"},
		{u: val, f: "%X", want: "1234567890ABCDEF1234567890ABCDEF"},
		{u: val, f: "%v", want: "12345678-90ab-cdef-1234-567890abcdef"},
		{u: val, f: "%+v", want: "12345678-90ab-cdef-1234-567890abcdef"},
		{u: val, f: "%#v", want: "[16]uint8{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}"},
		{u: val, f: "%T", want: "uuid.UUID"},
		{u: val, f: "%t", want: "%!t(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%b", want: "%!b(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%c", want: "%!c(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%d", want: "%!d(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%e", want: "%!e(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%E", want: "%!E(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%f", want: "%!f(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%F", want: "%!F(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%g", want: "%!g(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%G", want: "%!G(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%o", want: "%!o(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
		{u: val, f: "%U", want: "%!U(uuid.UUID=12345678-90ab-cdef-1234-567890abcdef)"},
	}
	for _, tt := range tests {
		got := fmt.Sprintf(tt.f, tt.u)
		if tt.want != got {
			t.Errorf(`Format("%s") got %s, want %s`, tt.f, got, tt.want)
		}
	}
}

func TestMust(t *testing.T) {
	sentinel := fmt.Errorf("uuid: sentinel error")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("did not panic, want %v", sentinel)
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panicked with %T, want error (%v)", r, sentinel)
		}
		if err != sentinel {
			t.Fatalf("panicked with %v, want %v", err, sentinel)
		}
	}()
	fn := func() (UUID, error) {
		return Nil, sentinel
	}
	Must(fn())
}
