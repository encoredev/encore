package runtime

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"net/http"

	"encore.dev/runtime/config"
)

type encoreAuthenticateCtxKey string

const encoreAuthenticatedKey encoreAuthenticateCtxKey = "encoreAuthenticateCtxKey"

func withEncoreAuthentication(ctx context.Context) context.Context {
	return context.WithValue(ctx, encoreAuthenticatedKey, true)
}

// IsEncoreAuthenticatedRequest returns true if the current context is spawned from a request which was
// initiated by an Encore server.
func IsEncoreAuthenticatedRequest(ctx context.Context) bool {
	value := ctx.Value(encoreAuthenticatedKey)
	if value == nil {
		return false
	}

	v, ok := value.(bool)
	return ok && v
}

func (srv *Server) checkAuth(req *http.Request, macSig string) (bool, error) {
	macBytes, err := base64.RawStdEncoding.DecodeString(macSig)
	if err != nil {
		return false, nil
	}

	// Pull out key ID from hmac prefix
	const keyIDLen = 4
	if len(macBytes) < keyIDLen {
		return false, nil
	}

	keyID := binary.BigEndian.Uint32(macBytes[:keyIDLen])
	mac := macBytes[keyIDLen:]
	for _, k := range config.Cfg.Runtime.AuthKeys {
		if k.KeyID == keyID {
			return checkAuth(k, req, mac), nil
		}
	}

	return false, nil
}
