package codegen

import (
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
