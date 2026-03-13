package cors

import (
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/rs/cors"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
)

func Wrap(cfg *config.CORS, staticAllowedHeaders, staticExposedHeaders []string, handler http.Handler, logger zerolog.Logger) http.Handler {
	c := cors.New(Options(cfg, staticAllowedHeaders, staticExposedHeaders))
	if cfg.Debug {
		logger := logger.With().Str("subsystem", "cors").Logger()
		logger.Debug().Msg("CORS system running in debug mode. All requests will be logged.")
		c.Log = &logger
	}
	return c.Handler(handler)
}

func Options(cfg *config.CORS, staticAllowedHeaders, staticExposedHeaders []string) cors.Options {
	// Sort origins to allow for binary search
	originsCreds := sortedSliceCopy(cfg.AllowOriginsWithCredentials)
	originsWithoutCreds := sortedSliceCopy(cfg.AllowOriginsWithoutCredentials)
	globCreds := getGlobOrigins(cfg.AllowOriginsWithCredentials)
	globWithoutCreds := getGlobOrigins(cfg.AllowOriginsWithoutCredentials)

	// Determine if we have a wildcard origins
	hasWildcardOriginWithoutCreds := cfg.AllowOriginsWithoutCredentials == nil || sortedSliceContains(originsWithoutCreds, "*")
	hasUnsafeWildcardOriginWithCreds := sortedSliceContains(originsCreds, config.UnsafeAllOriginWithCredentials)

	// allowedHeaders are the headers allowed through CORS.
	allowedHeaders := []string{
		"Authorization",
		"Content-Type",
		"User-Agent",
		"X-Request-ID",
		"X-Correlation-ID",
	}
	allowedHeaders = append(allowedHeaders, cfg.ExtraAllowedHeaders...)
	allowedHeaders = append(allowedHeaders, staticAllowedHeaders...)

	exposedHeaders := []string{
		"X-Request-ID",
		"X-Correlation-ID",
		"X-Encore-Trace-ID",
	}
	exposedHeaders = append(exposedHeaders, cfg.ExtraExposedHeaders...)
	exposedHeaders = append(exposedHeaders, staticExposedHeaders...)

	// Sort the slices so the output looks nicer.
	sort.Strings(allowedHeaders)
	sort.Strings(exposedHeaders)

	return cors.Options{
		Debug:               cfg.Debug,
		AllowCredentials:    !cfg.DisableCredentials,
		AllowedMethods:      []string{"GET", "POST", "PUT", "PATCH", "HEAD", "DELETE", "OPTIONS", "TRACE", "CONNECT"},
		AllowedHeaders:      allowedHeaders,
		ExposedHeaders:      exposedHeaders,
		AllowPrivateNetwork: cfg.AllowPrivateNetworkAccess,
		AllowOriginRequestFunc: func(r *http.Request, origin string) bool {
			// If the request has credentials, look up origins in AllowOriginsWithCredentials.
			// Credentials are cookies, authorization headers, or TLS client certificates.
			hasCreds := func(r *http.Request) bool {
				if len(r.Cookies()) > 0 || len(r.Header.Values("Authorization")) > 0 || (r.TLS != nil && len(r.TLS.PeerCertificates) > 0) {
					return true
				}

				if r.Method == http.MethodOptions {
					return slices.ContainsFunc(
						r.Header.Values("Access-Control-Request-Headers"),
						func(val string) bool {
							return slices.ContainsFunc(
								strings.Split(val, ","),
								func(val string) bool {
									val = strings.TrimSpace(val)
									return val == "authorization" || val == "cookie"
								},
							)
						},
					)
				}

				return false
			}
			if hasCreds(r) {
				ok := hasUnsafeWildcardOriginWithCreds || sortedSliceContains(originsCreds, origin)
				if !ok {
					// Not an exact match. Check any glob origins.
					ok = globCreds.Matches(origin)
				}
				return ok
			}
			// Post-condition: request is without credentials

			ok := hasWildcardOriginWithoutCreds || sortedSliceContains(originsWithoutCreds, origin)
			if !ok {
				// Not an exact match. Check any glob origins.
				ok = globWithoutCreds.Matches(origin)
			}
			return ok
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

// globOrigin represents a parsed origin pattern with glob support.
type globOrigin struct {
	scheme, hostname, port string
}

type globOriginSet []globOrigin

func (s globOriginSet) Matches(origin string) bool {
	o, ok := parseOrigin(origin)
	if !ok {
		return false
	}
	for _, pattern := range s {
		if pattern.matches(o) {
			return true
		}
	}
	return false
}

func (pattern globOrigin) matches(origin globOrigin) bool {
	if pattern.scheme != origin.scheme {
		return false
	}
	if matched, _ := filepath.Match(pattern.port, origin.port); !matched {
		return false
	}
	matched, _ := filepath.Match(pattern.hostname, origin.hostname)
	return matched
}

// parseOrigin splits an origin string into scheme, hostname, and port.
// The port is normalized to the default port for the scheme if not specified.
// See https://developer.mozilla.org/en-US/docs/Glossary/Origin.
func parseOrigin(origin string) (globOrigin, bool) {
	scheme, rest, ok := strings.Cut(origin, "://")
	if !ok {
		return globOrigin{}, false
	}
	// Strip any path component.
	if idx := strings.Index(rest, "/"); idx != -1 {
		rest = rest[:idx]
	}
	hostname, port, hasPort := strings.Cut(rest, ":")
	if !hasPort {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return globOrigin{scheme, hostname, port}, true
}

func getGlobOrigins(origins []string) globOriginSet {
	var globs []globOrigin
	for _, o := range origins {
		if o == "*" || !strings.Contains(o, "*") {
			continue
		}
		if parsed, ok := parseOrigin(o); ok {
			globs = append(globs, parsed)
		}
	}
	return globs
}
