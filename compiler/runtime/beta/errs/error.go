package errs

import (
	"fmt"
	"net/http"
	"strings"
	"unsafe"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/internal/stack"
)

var json = jsoniter.Config{
	EscapeHTML:             false,
	SortMapKeys:            false,
	ValidateJsonRawMessage: true,
}.Froze()

type Error struct {
	Code    ErrCode    `json:"code"`
	Message string     `json:"message"`
	Details ErrDetails `json:"details"`
	Meta    Metadata   `json:"-"` // not exposed to external clients

	// underlying is the underlying error,
	// for use with errors.Is and errors.As.
	// It is not propagated across RPC boundaries.
	underlying error

	stack stack.Stack
}

type Metadata map[string]interface{}

func Wrap(err error, msg string, metaPairs ...interface{}) error {
	if err == nil {
		return nil
	}

	e := &Error{Code: Unknown, Message: msg, underlying: err}
	if ee, ok := err.(*Error); ok {
		e.Details = ee.Details
		e.Code = ee.Code
		e.Meta = mergeMeta(ee.Meta, metaPairs)
		e.stack = ee.stack
	} else {
		e.Meta = mergeMeta(nil, metaPairs)
		e.stack = stack.Build(2)
	}
	return e
}

func WrapCode(err error, code ErrCode, msg string, metaPairs ...interface{}) error {
	if err == nil || code == OK {
		return nil
	}

	e := &Error{Code: code, Message: msg, underlying: err}
	if ee, ok := err.(*Error); ok {
		e.Details = ee.Details
		e.Code = ee.Code
		e.Meta = mergeMeta(ee.Meta, metaPairs)
		e.stack = ee.stack
	} else {
		e.Meta = mergeMeta(nil, metaPairs)
		e.stack = stack.Build(2)
	}
	return e
}

func Convert(err error) error {
	if err == nil {
		return nil
	} else if e, ok := err.(*Error); ok {
		return e
	}
	return &Error{
		Code:       Unknown,
		underlying: err,
		stack:      stack.Build(2),
	}
}

func Code(err error) ErrCode {
	if err == nil {
		return OK
	} else if e, ok := err.(*Error); ok {
		return e.Code
	}
	return Unknown
}

func Details(err error) ErrDetails {
	if e, ok := err.(*Error); ok {
		return e.Details
	}
	return nil
}

func Meta(err error) Metadata {
	if e, ok := err.(*Error); ok {
		return e.Meta
	}
	return nil
}

func (e *Error) Error() string {
	return e.Code.String() + ": " + e.ErrorMessage()
}

func (e *Error) ErrorMessage() string {
	if e.underlying == nil {
		return e.Message
	}

	var b strings.Builder
	b.WriteString(e.Message)

	var next error = e.underlying
	for next != nil {
		var msg string
		if e, ok := next.(*Error); ok {
			msg = e.Message
			next = e.underlying
		} else {
			msg = next.Error()
			next = nil
		}
		if b.Len() > 0 && msg != "" {
			b.WriteString(": ")
		}
		b.WriteString(msg)
	}
	return b.String()
}

func (e *Error) Unwrap() error {
	return e.underlying
}

func HTTPError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err == nil {
		w.WriteHeader(200)
		w.Write([]byte(`{
  "code": "ok",
  "message": "",
  "details": null
}
`))
		return
	}

	e := Convert(err).(*Error)
	data, err2 := json.MarshalIndent(e, "", "  ")
	if err2 != nil {
		// Must be the details; drop them
		e2 := &Error{Code: e.Code, Message: e.Message}
		data, _ = json.MarshalIndent(e2, "", "  ")
	}
	w.WriteHeader(e.Code.HTTPStatus())
	w.Write(data)
}

func HTTPStatus(err error) int {
	code := Code(err)
	switch code {
	case OK:
		return 200
	case Canceled:
		return 499
	case Unknown:
		return 500
	case InvalidArgument:
		return 400
	case DeadlineExceeded:
		return 504
	case NotFound:
		return 404
	case AlreadyExists:
		return 409
	case PermissionDenied:
		return 403
	case ResourceExhausted:
		return 429
	case FailedPrecondition:
		return 400
	case Aborted:
		return 409
	case OutOfRange:
		return 400
	case Unimplemented:
		return 501
	case Internal:
		return 500
	case Unavailable:
		return 503
	case DataLoss:
		return 500
	case Unauthenticated:
		return 401
	default:
		return 500
	}
}

func mergeMeta(md Metadata, pairs []interface{}) Metadata {
	n := len(pairs)
	if n%2 != 0 {
		panic(fmt.Sprintf("got uneven number (%d) of metadata key-values", n))
	}
	if md == nil && n > 0 {
		md = make(Metadata, n/2)
	}
	for i := 0; i < n; i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			panic(fmt.Sprintf("metadata key-value pair #%d key is not a string (is %T)", i/2, pairs[i]))
		}
		md[key] = pairs[i+1]
	}
	return md
}

func init() {
	jsoniter.RegisterTypeEncoderFunc("errs.Error", func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
		e := (*Error)(ptr)
		stream.WriteObjectStart()
		stream.WriteObjectField("code")
		stream.WriteString(e.Code.String())
		stream.WriteMore()
		stream.WriteObjectField("message")
		stream.WriteString(e.ErrorMessage())
		stream.WriteMore()
		stream.WriteObjectField("details")
		stream.WriteVal(e.Details)
		stream.WriteObjectEnd()
	}, nil)

}
