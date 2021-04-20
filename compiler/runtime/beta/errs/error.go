package errs

import (
	"fmt"
	"net/http"
	"strings"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
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

	// underlying is the underlying error,
	// for use with errors.Is and errors.As.
	// It is not propagated across RPC boundaries.
	underlying error

	// TODO stack, params, retryable, ...
}

// New creates a new Error without wrapping another underlying error.
// If code == OK it reports nil.
func New(code ErrCode, msg string, details ErrDetails) error {
	if code == OK {
		return nil
	}
	return &Error{Code: code, Message: msg, Details: details}
}

// Wrap wraps the err, adding additional error information.
// If err is nil it returns nil.
//
// If err is already an *Error its code, message, and details
// are copied over to the new error.
//
// The fields are used to update the corresponding field of the error:
// Passing in an ErrCode updates the Code field.
// Passing in a string adds context to the error message.
// Passing in a type that implements ErrDetails updates the Details field,
// and passing in untyped nil sets Details to nil.
//
// Passing in another type causes Wrap to panic.
func Wrap(err error, fields ...interface{}) error {
	if err == nil {
		return nil
	}

	e := &Error{Code: Unknown, underlying: err}
	if ee, ok := err.(*Error); ok {
		e.Details = ee.Details
		e.Code = ee.Code
		// Note: not setting message from ee because it's already
		// returned by ErrorMessage() from the underlying err.
	}

	for _, f := range fields {
		switch f := f.(type) {
		case ErrDetails:
			e.Details = f
		case ErrCode:
			e.Code = f
		case string:
			e.Message = f
		case nil:
			e.Details = nil
		default:
			panic(fmt.Sprintf("errs.Wrap: unsupported field type %T", f))
		}
	}

	return e
}

func Code(err error) ErrCode {
	if err == nil {
		return OK
	} else if e, ok := err.(*Error); ok {
		return e.Code
	}
	return Unknown
}

// Details reports the error details included in the error.
// If err is nil or the error lacks details it reports nil.
func Details(err error) ErrDetails {
	if e, ok := err.(*Error); !ok {
		return nil
	} else {
		return e.Details
	}
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

	e := Wrap(err).(*Error)
	data, err2 := json.MarshalIndent(e, "", "  ")
	if err2 != nil {
		// Must be the details; drop them
		e2 := &Error{Code: e.Code, Message: e.Message}
		data, _ = json.MarshalIndent(e2, "", "  ")
	}
	w.WriteHeader(e.Code.HTTPStatus())
	w.Write(data)
}

// HTTPStatus reports a suitable HTTP status code for a code.
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
