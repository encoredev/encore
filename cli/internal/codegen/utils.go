package codegen

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"encr.dev/parser/encoding"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

func hasPublicRPC(svc *meta.Service) bool {
	for _, rpc := range svc.Rpcs {
		if rpc.AccessType != meta.RPC_PRIVATE {
			return true
		}
	}
	return false
}

func toFieldLists(fields []*encoding.ParameterEncoding) (header []*encoding.ParameterEncoding, query []*encoding.ParameterEncoding, body []*encoding.ParameterEncoding, err error) {
	for _, field := range fields {
		switch field.Location {
		case encoding.Header:
			header = append(header, field)
		case encoding.Query:
			query = append(query, field)
		case encoding.Body:
			body = append(body, field)
		default:
			err = errors.Newf("unexpected location: %+v", field.Location)
		}
	}
	return
}

type indentWriter struct {
	w                *bytes.Buffer
	depth            int
	indent           string
	firstWriteOnLine bool
}

func (w *indentWriter) Indent() *indentWriter {
	return &indentWriter{
		w:                w.w,
		depth:            w.depth + 1,
		indent:           w.indent,
		firstWriteOnLine: true,
	}
}

func (w *indentWriter) WriteString(s string) {
	parts := strings.Split(s, "\n")
	for i, part := range parts {
		if i > 0 {
			w.w.WriteString("\n")
			w.firstWriteOnLine = true
		}
		if part == "" {
			continue
		}

		if w.firstWriteOnLine {
			w.w.WriteString(strings.Repeat(w.indent, w.depth))
			w.firstWriteOnLine = false
		}

		w.w.WriteString(part)
	}
}

func (w *indentWriter) WriteStringf(s string, args ...interface{}) {
	w.WriteString(fmt.Sprintf(s, args...))
}
