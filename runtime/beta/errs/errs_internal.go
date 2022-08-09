package errs

import (
	"bytes"
	"encoding/gob"
	"log"
	"net/http"

	"encore.dev/internal/stack"
)

var statusToCode = map[int]ErrCode{
	200: OK,
	499: Canceled,
	500: Internal,
	400: InvalidArgument,
	401: Unauthenticated,
	403: PermissionDenied,
	404: NotFound,
	409: AlreadyExists,
	429: ResourceExhausted,
	501: Unimplemented,
	503: Unavailable,
	504: DeadlineExceeded,
}

func HTTPStatusToCode(status int) ErrCode {
	if c, ok := statusToCode[status]; ok {
		return c
	}
	return Unknown
}

func Stack(err error) stack.Stack {
	if e, ok := err.(*Error); ok {
		return e.stack
	}
	return stack.Stack{}
}

func DropStackFrame(err error) error {
	if e, ok := err.(*Error); ok && len(e.stack.Frames) > 0 {
		e.stack.Frames = e.stack.Frames[1:]
	}
	return err
}

// RoundTrip copies an error, returning an equivalent error
// for replicating across RPC boundaries.
func RoundTrip(err error) error {
	if err == nil {
		return nil
	} else if e, ok := err.(*Error); ok {
		e2 := &Error{
			Code:    e.Code,
			Message: e.Message,
			stack:   stack.Build(3), // skip caller of RoundTrip as well
		}

		// Copy details
		if e.Details != nil {
			var buf bytes.Buffer
			gob.Register(e.Details)
			enc := gob.NewEncoder(&buf)
			if err := enc.Encode(struct{ Details ErrDetails }{Details: e.Details}); err != nil {
				log.Printf("failed to encode error details: %v", err)
			} else {
				dec := gob.NewDecoder(&buf)
				var dst struct{ Details ErrDetails }
				if err := dec.Decode(&dst); err != nil {
					log.Printf("failed to decode error details: %v", err)
				} else {
					e2.Details = dst.Details
				}
			}
		}

		// Copy meta
		if e.Meta != nil {
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			if err := enc.Encode(e.Meta); err != nil {
				log.Printf("failed to encode error metadata: %v", err)
			} else {
				dec := gob.NewDecoder(&buf)
				if err := dec.Decode(&e2.Meta); err != nil {
					log.Printf("failed to decode error metadata: %v", err)
				}
			}
		}

		return e2
	} else {
		return &Error{
			Code:    Unknown,
			Message: err.Error(),
			stack:   stack.Build(3), // skip caller of RoundTrip as well
		}
	}
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

// HTTPErrorWithCode writes structured error information to w using JSON encoding.
// The given status code is used if it is non-zero, and otherwise
// it is computed with HTTPStatus.
//
// If err is nil it writes:
//     {"code": "ok", "message": "", "details": null}
func HTTPErrorWithCode(w http.ResponseWriter, err error, code int) {
	if code == 0 {
		code = HTTPStatus(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err == nil {
		w.WriteHeader(code)
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
	w.WriteHeader(code)
	w.Write(data)
}
