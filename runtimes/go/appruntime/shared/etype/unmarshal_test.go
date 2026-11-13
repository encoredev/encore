package etype_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/shared/etype"
)

type bodyErrorDetail interface{ PublicDetail() string }

func parseJSON(t *testing.T, field, body string, dst any) error {
	t.Helper()
	var dec etype.Unmarshaller
	iter := jsoniter.ConfigCompatibleWithStandardLibrary.BorrowIterator([]byte(body))
	dec.ParseJSON(field, iter, dst)
	return dec.Error
}

func TestParseJSONDetail(t *testing.T) {
	var (
		s   string
		b   bool
		i   int
		f   float64
		sl  []string
		raw []byte
		m   map[string]string
		p   *string
		ts  time.Time
	)
	tests := []struct {
		name string
		body string
		dst  any
		want string
	}{
		{"string", `123`, &s, `invalid value for field "title": expected a string`},
		{"bool", `"x"`, &b, `invalid value for field "title": expected a boolean`},
		{"int", `"x"`, &i, `invalid value for field "title": expected an integer`},
		{"float", `"x"`, &f, `invalid value for field "title": expected a number`},
		{"array", `"x"`, &sl, `invalid value for field "title": expected an array`},
		{"bytes", `123`, &raw, `invalid value for field "title": expected a base64-encoded string`},
		{"object", `"x"`, &m, `invalid value for field "title": expected an object`},
		{"pointer", `123`, &p, `invalid value for field "title": expected a string`},
		{"custom unmarshaller", `123`, &ts, `invalid value for field "title"`},
		{"truncated", `"abc`, &s, `invalid value for field "title": expected a string`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseJSON(t, "title", tt.body, tt.dst)
			var d bodyErrorDetail
			if !errors.As(err, &d) {
				t.Fatalf("%v is not a body error", err)
			}
			if got := d.PublicDetail(); got != tt.want {
				t.Errorf("detail = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetailNamesNoInternals(t *testing.T) {
	var s string
	err := parseJSON(t, "title", `123`, &s)
	var d bodyErrorDetail
	errors.As(err, &d)
	for _, leak := range []string{"ReadString", "byte", "123", "Title"} {
		if strings.Contains(d.PublicDetail(), leak) {
			t.Errorf("detail %q contains %q", d.PublicDetail(), leak)
		}
	}
}

func TestReadBodyDetail(t *testing.T) {
	var dec etype.Unmarshaller
	dec.ReadBody(strings.NewReader(""))
	var d bodyErrorDetail
	if !errors.As(dec.Error, &d) || d.PublicDetail() != "missing request body" {
		t.Errorf("got %v, want a body error with detail %q", dec.Error, "missing request body")
	}
}

func TestReadBodyRejectsNonObject(t *testing.T) {
	for body, wantErr := range map[string]bool{`42`: true, ` "x"`: true, `[1]`: true, " ": true, ` {"a":1}`: false, `null`: false} {
		var dec etype.Unmarshaller
		dec.ReadBody(strings.NewReader(body))
		if got := dec.Error != nil; got != wantErr {
			t.Errorf("body %q: error = %v, want error %v", body, dec.Error, wantErr)
		}
	}
}

func TestParameterErrorsAreNotBodyErrors(t *testing.T) {
	var dec etype.Unmarshaller
	etype.UnmarshalOne(&dec, strconv.Atoi, "limit", "abc", true)
	if dec.Error == nil || errors.As(dec.Error, new(bodyErrorDetail)) {
		t.Errorf("%v must be an unmarked parameter error", dec.Error)
	}
}

func TestEarlierParameterErrorIsNotMarkedByLaterBodyError(t *testing.T) {
	var dec etype.Unmarshaller
	etype.UnmarshalOne(&dec, strconv.Atoi, "limit", "abc", true)
	dec.ReadBody(strings.NewReader(""))
	if errors.As(dec.Error, new(bodyErrorDetail)) {
		t.Errorf("%v: first error wins and it is a parameter error", dec.Error)
	}
}
