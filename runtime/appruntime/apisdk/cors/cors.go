package cors

import (
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/cors"
	"github.com/rs/zerolog/log"

	"encore.dev/appruntime/exported/config"
)

func Wrap(cfg *config.CORS, staticAllowedHeaders, staticExposedHeaders []string, handler http.Handler) http.Handler {
	c := cors.New(Options(cfg, staticAllowedHeaders, staticExposedHeaders))
	if cfg.Debug {
		logger := log.With().Str("subsystem", "cors").Logger()
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
			hasCreds := len(r.Cookies()) > 0 || r.Header["Authorization"] != nil || (r.TLS != nil && len(r.TLS.PeerCertificates) > 0)
			if hasCreds {
				ok := hasUnsafeWildcardOriginWithCreds || sortedSliceContains(originsCreds, origin)
				if !ok {
					// Not an exact match. Check any glob origins.
					ok = globCreds.Matches(origin) || globWithoutCreds.Matches(origin)
				}
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

type globOriginSet []*url.URL

func (s globOriginSet) Matches(origin string) bool {
	if u, err := url.Parse(origin); err == nil {
		for _, allow := range s {
			if globMatch(allow, u) {
				return true
			}
		}
	}
	return false
}

func globMatch(pattern, origin *url.URL) bool {
	// For it to be a match, the schemes, hostname and port must all match.
	if pattern.Scheme != origin.Scheme {
		return false
	}

	// Make sure the port matches. If there is no port, the port is
	// the standard port for the scheme. See https://developer.mozilla.org/en-US/docs/Glossary/Origin.
	normalizedPort := func(u *url.URL) string {
		if u.Port() == "" {
			switch u.Scheme {
			case "http":
				return "80"
			case "https":
				return "443"
			default:
				return ""
			}
		}
		return u.Port()
	}
	if normalizedPort(pattern) != normalizedPort(origin) {
		return false
	}

	// Make sure the hostname matches. Since we are only checking the hostname,
	// it's fine to use filepath.Match
	matched, err := filepath.Match(pattern.Hostname(), origin.Hostname())
	return matched && err == nil
}

func getGlobOrigins(origins []string) globOriginSet {
	var globs []*url.URL
	for _, o := range origins {
		if o == "*" {
			// The literal "*" string means something else and is not
			// covered by the glob-matching support.
			continue
		}

		if o != "*" && strings.Contains(o, "*") {
			if u, err := url.Parse(o); err == nil {
				globs = append(globs, u)
			}
		}
	}
	return globs
}
