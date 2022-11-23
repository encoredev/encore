package cors

import (
	"net/http"
	"sort"

	"github.com/rs/cors"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/config"
)

func Wrap(cfg *config.CORS, handler http.Handler) http.Handler {
	c := cors.New(Options(cfg))
	if cfg.Debug {
		logger := log.With().Str("subsystem", "cors").Logger()
		logger.Debug().Msg("CORS system running in debug mode. All requests will be logged.")
		c.Log = &logger
	}
	return c.Handler(handler)
}

func Options(cfg *config.CORS) cors.Options {
	// Sort origins to allow for binary search
	originsCreds := sortedSliceCopy(cfg.AllowOriginsWithCredentials)
	originsWithoutCreds := sortedSliceCopy(cfg.AllowOriginsWithoutCredentials)

	// Determine if we have a wildcard origins
	hasWildcardOriginWithoutCreds := cfg.AllowOriginsWithoutCredentials == nil || sortedSliceContains(originsWithoutCreds, "*")
	hasUnsafeWildcardOriginWithCreds := sortedSliceContains(originsCreds, config.UnsafeAllOriginWithCredentials)

	allowedHeaders := append([]string{"Origin", "Authorization", "Content-Type"}, cfg.ExtraAllowedHeaders...)

	return cors.Options{
		Debug:               cfg.Debug,
		AllowCredentials:    !cfg.DisableCredentials,
		AllowedMethods:      []string{"GET", "POST", "PUT", "PATCH", "HEAD", "DELETE", "OPTIONS", "TRACE", "CONNECT"},
		AllowedHeaders:      allowedHeaders,
		AllowPrivateNetwork: cfg.AllowAccessWhenOnPrivateNetwork,
		AllowOriginRequestFunc: func(r *http.Request, origin string) bool {
			// If the request has credentials, look up origins in AllowOriginsWithCredentials.
			// Credentials are cookies, authorization headers, or TLS client certificates.
			hasCreds := len(r.Cookies()) > 0 || r.Header["Authorization"] != nil || (r.TLS != nil && len(r.TLS.PeerCertificates) > 0)
			if hasCreds {
				ok := hasUnsafeWildcardOriginWithCreds || sortedSliceContains(originsCreds, origin)
				return ok
			}
			// Post-condition: request is without credentials

			if hasWildcardOriginWithoutCreds {
				return true
			}
			return sortedSliceContains(originsWithoutCreds, origin)
		},
	}
}

func sortedSliceContains(haystack []string, needle string) bool {
	idx := sort.SearchStrings(haystack, needle)
	return idx < len(haystack) && haystack[idx] == needle
}

func sortedSliceCopy(src []string) []string {
	if src == nil {
		return nil
	}

	dst := make([]string, len(src))
	copy(dst, src)
	sort.Strings(dst)
	return dst
}
