package cors

import (
	"net/http"
	"sort"

	"github.com/rs/cors"

	"encore.dev/runtime/config"
)

func Wrap(cfg *config.CORS, handler http.Handler) http.Handler {
	c := cors.New(Options(cfg))
	return c.Handler(handler)
}

func Options(cfg *config.CORS) cors.Options {
	// Sort origins to allow for binary search
	originsCreds := sortedSliceCopy(cfg.AllowOriginsWithCredentials)
	originsWithoutCreds := sortedSliceCopy(cfg.AllowOriginsWithoutCredentials)

	// Determine if we have a wildcard origin for requests without credentials
	hasWildcardOriginWithoutCreds := cfg.AllowOriginsWithoutCredentials == nil || sortedSliceContains(originsWithoutCreds, "*")

	return cors.Options{
		AllowCredentials: !cfg.DisableCredentials,
		AllowedMethods:   []string{"GET", "POST", "PUT", "HEAD", "DELETE", "OPTIONS", "TRACE", "CONNECT"},
		AllowedHeaders:   []string{"Authorization"},
		AllowOriginRequestFunc: func(r *http.Request, origin string) bool {
			// If the request has credentials, look up origins in AllowOriginsWithCredentials.
			// Credentials are cookies, authorization headers, or TLS client certificates.
			hasCreds := len(r.Cookies()) > 0 || r.Header["Authorization"] != nil || (r.TLS != nil && len(r.TLS.PeerCertificates) > 0)
			if hasCreds {
				return sortedSliceContains(originsCreds, origin)
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
