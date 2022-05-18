package codegen

import (
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
