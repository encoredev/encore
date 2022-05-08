// Package serde implements Encore's serialization and deserialization support.
package serde

import (
	"io"
	"net/http"
	"net/url"

	jsoniter "github.com/json-iterator/go"
)

var itercfg = jsoniter.ConfigCompatibleWithStandardLibrary

type RequestSpec struct {
	Headers http.Header // nil if no headers
	Query   url.Values  // nil if no query params
	Payload io.Reader   // nil if no payload
}

// NewDecoder returns a Decoder that decodes the given request spec.
func NewDecoder(spec RequestSpec) *Decoder {
	return &Decoder{
		iter: jsoniter.Parse(itercfg, spec.Payload, 512),
	}
}

// Decoder is a deserialization context.
type Decoder struct {
	// iter is the underlying JSON iterator used to decode requests.
	iter *jsoniter.Iterator
}

// Header returns the value of the given header name.
// It returns "", false if the header was not provided.
func (d *Decoder) Header(name string) (string, bool) {
	// TODO(andre) implement
	return "", false
}

// Query returns the value of the given query string name.
// It returns "", false if the query string was not provided.
func (d *Decoder) Query(name string) (string, bool) {
	// TODO(andre) implement
	return "", false
}

// ObjectCB decodes an object, calling fn for each key-value pair.
// It assumes the keys are valid struct field names, meaning they are
// ASCII only without escape codes.
// If fn reports false it means an error was encountered and decoding is aborted.
// To decode maps with arbitrary unicode keys, where all key-value pairs of the same type, use Map.
func (d *Decoder) ObjectCB(fn func(d *Decoder, field string) bool) bool {
	return d.iter.ReadObjectCB(func(_ *jsoniter.Iterator, field string) bool {
		return fn(d, field)
	})
}

// MapCB decodes a map, calling fn for each key-value pair.
func (d *Decoder) MapCB(fn func(d *Decoder, key string) bool) bool {
	return d.iter.ReadMapCB(func(_ *jsoniter.Iterator, key string) bool {
		return fn(d, key)
	})
}

// Null checks if the next JSON value in the stream is null,
// and if so consumes it and reports true. Otherwise it reports false
// and leaves the stream unmodified.
func (d *Decoder) Null() bool { return d.iter.ReadNil() }

// Skip skips the next JSON value.
func (d *Decoder) Skip() {
	d.iter.Skip()
}

// Err reports any error encountered during decoding.
func (d *Decoder) Err() error {
	return d.iter.Error
}

// Value readers. If the JSON value is null they all report the zero value.

func (d *Decoder) String() string   { return d.iter.ReadString() }
func (d *Decoder) Int() int         { return d.iter.ReadInt() }
func (d *Decoder) Any(dst any) bool { d.iter.ReadVal(dst); return d.iter.Error == nil }

// A RequestDecoder is a type that knows how to decode requests into itself.
type RequestDecoder interface {
	DecodeRequest(d *Decoder) bool
}

// MakeMap makes a new map of type M.
// It's a convenience method to simplify allocating maps of arbitrary types.
func MakeMap[M ~map[K]V, K comparable, V any](dst *M) {
	*dst = make(M)
}

// AllocMapValue allocates a zero value for a map of type M.
// The parameter exists to facilitate type inference in generated code,
// and is not otherwise used; it can be nil.
func AllocMapValue[M ~map[K]V, K comparable, V any](_ M) V {
	var zero V
	return zero
}

// NewPtrValue allocates a zero value of type V into dst.
// It's a convenience method to simplify initializing pointers to types.
func NewPtrValue[V any](dst **V) {
	obj := new(V)
	*dst = obj
}
