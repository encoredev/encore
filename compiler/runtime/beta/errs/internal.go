package errs

import (
	"bytes"
	"encoding/gob"
	"log"
)

// RoundTrip copies an error, returning an equivalent error
// for replicating across RPC boundaries.
func RoundTrip(err error) error {
	if err == nil {
		return nil
	} else if e, ok := err.(*Error); ok {
		e2 := &Error{
			Code:    e.Code,
			Message: e.Message,
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
		}
	}
}
