package cors

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	_ "unsafe"

	"github.com/rs/cors"

	"encore.dev/appruntime/exported/config"
)

func TestOptions(t *testing.T) {
	tests := []struct {
		name               string
		cfg                config.CORS
		credsGoodOrigins   []string
		credsBadOrigins    []string
		nocredsGoodOrigins []string
		nocredsBadOrigins  []string
		goodHeaders        []string
		badHeaders         []string
	}{
		{
			name:               "empty",
			cfg:                config.CORS{},
			credsGoodOrigins:   []string{},
			credsBadOrigins:    []string{"foo.com", "evil.com", "localhost"},
			nocredsGoodOrigins: []string{"foo.com", "localhost", "", "icanhazcheezburger.com"},
			nocredsBadOrigins:  []string{},
			goodHeaders:        []string{"Authorization", "Content-Type", "Origin"},
			badHeaders:         []string{"X-Requested-With", "X-Forwarded-For"},
		},
		{
			name: "allowed_creds",
			cfg: config.CORS{
				AllowOriginsWithCredentials: []string{"localhost", "ok.org"},
			},
			credsGoodOrigins:   []string{"localhost", "ok.org"},
			credsBadOrigins:    []string{"foo.com", "evil.com"},
			nocredsGoodOrigins: []string{"foo.com", "localhost", "", "icanhazcheezburger.com", "ok.org"},
			nocredsBadOrigins:  []string{},
		},
		{
			name: "allowed_glob_creds",
			cfg: config.CORS{
				AllowOriginsWithCredentials: []string{"https://*.example.com", "wss://ok1-*.example.com", "https://*-ok2.example.com"},
			},
			credsGoodOrigins: []string{"https://foo.example.com", "wss://ok1-foo.example.com", "https://foo-ok2.example.com"},
			credsBadOrigins: []string{
				"http://foo.example.com",   // Wrong scheme
				"htts://.example.com",      // No subdomain
				"ws://ok1-foo.example.com", // Wrong scheme
				".example.com",             // No scheme
				"https://evil.com",         // bad domain
			},
			nocredsGoodOrigins: []string{},
			nocredsBadOrigins:  []string{},
		},
		{
			name: "allowed_nocreds",
			cfg: config.CORS{
				AllowOriginsWithoutCredentials: []string{"localhost", "ok.org"},
			},
			credsGoodOrigins:   []string{},
			credsBadOrigins:    []string{"localhost", "ok.org", "foo.com", "evil.com"},
			nocredsGoodOrigins: []string{"localhost", "ok.org"},
			nocredsBadOrigins:  []string{"foo.com", "", "icanhazcheezburger.com"},
		},
		{
			name: "allowed_disjoint_sets",
			cfg: config.CORS{
				AllowOriginsWithCredentials:    []string{"foo.com"},
				AllowOriginsWithoutCredentials: []string{"bar.org"},
			},
			credsGoodOrigins:   []string{"foo.com"},
			credsBadOrigins:    []string{"bar.org", "", "localhost"},
			nocredsGoodOrigins: []string{"bar.org"},
			nocredsBadOrigins:  []string{"foo.com", "", "localhost"},
		},
		{
			name: "allowed_wildcard_without_creds",
			cfg: config.CORS{
				AllowOriginsWithoutCredentials: []string{"*"},
			},
			credsGoodOrigins:   []string{},
			credsBadOrigins:    []string{"bar.org", "", "localhost"},
			nocredsGoodOrigins: []string{"bar.org", "bar.com", "", "localhost"},
		},
		{
			name: "allowed_unsafe_wildcard_with_creds",
			cfg: config.CORS{
				AllowOriginsWithCredentials: []string{config.UnsafeAllOriginWithCredentials},
			},
			credsGoodOrigins: []string{"bar.org", "bar.com", "", "localhost", "unsafe.evil.com"},
		},
		{
			name: "extra_headers",
			cfg: config.CORS{
				ExtraAllowedHeaders: []string{"X-Forwarded-For", "X-Real-Ip"},
			},
			goodHeaders: []string{"Authorization", "Content-Type", "Origin", "X-Forwarded-For", "X-Real-Ip"},
			badHeaders:  []string{"X-Requested-With", "X-Evil-Header"},
		},
		{
			name: "extra_headers_wildcard",
			cfg: config.CORS{
				ExtraAllowedHeaders: []string{"X-Forwarded-For", "*", "X-Real-Ip"},
			},
			goodHeaders: []string{"Authorization", "Content-Type", "Origin", "X-Forwarded-For", "X-Real-Ip", "X-Requested-With", "X-Evil-Header"},
		},
		{
			name:        "static_headers",
			cfg:         config.CORS{},
			goodHeaders: []string{"Authorization", "Content-Type", "Origin", "X-Static-Test"},
		},
	}

	checkOrigins := func(t *testing.T, c *cors.Cors, creds, good bool, origins []string) {
		for _, o := range origins {
			h := make(http.Header)
			h.Set("Origin", o)
			if creds {
				h.Set("Authorization", "dummy")
			}
			allowed := c.OriginAllowed(&http.Request{Header: h})
			if allowed != good {
				t.Fatalf("origin=%s creds=%v: got allowed=%v, want %v", o, creds, allowed, good)
			} else {
				t.Logf("origin=%s creds=%v: ok allowed=%v", o, creds, allowed)
			}
		}
	}

	checkHeaders := func(t *testing.T, c *cors.Cors, allowedHeaders, exposedHeaders []string, wantOK bool) {
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "https://example.org")
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", strings.Join(allowedHeaders, ", "))
		w := httptest.NewRecorder()
		c.ServeHTTP(w, req, nil)

		if w.Code != http.StatusNoContent {
			t.Fatalf("got OPTIONS response code %d, want 204", w.Code)
		}

		// Check allowed headers.
		{
			rawAllowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
			allowHeaders := strings.Split(rawAllowedHeaders, ", ")
			allowed := make(map[string]bool)
			for _, val := range allowHeaders {
				allowed[strings.TrimSpace(val)] = true
			}

			if wantOK {
				for _, val := range allowedHeaders {
					if !allowed[val] {
						t.Fatalf("want header %q to be allowed, got false; resp header=%q", val, rawAllowedHeaders)
					}
				}
			} else {
				if rawAllowedHeaders != "" {
					t.Fatalf("want headers not to be allowed, got %q", rawAllowedHeaders)
				}
			}
		}

		// Check exposed headers.
		{
			rawAllowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
			allowHeaders := strings.Split(rawAllowedHeaders, ", ")
			allowed := make(map[string]bool)
			for _, val := range allowHeaders {
				allowed[strings.TrimSpace(val)] = true
			}

			if wantOK {
				for _, val := range allowedHeaders {
					if !allowed[val] {
						t.Fatalf("want header %q to be allowed, got false; resp header=%q", val, rawAllowedHeaders)
					}
				}
			} else {
				if rawAllowedHeaders != "" {
					t.Fatalf("want headers not to be allowed, got %q", rawAllowedHeaders)
				}
			}
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Options(&tt.cfg, []string{"X-Static-Test"}, []string{"X-Exposed-Test"})
			got.Debug = true
			c := cors.New(got)
			c.Log = log.New(os.Stdout, "cors: ", 0)

			checkOrigins(t, c, true, true, tt.credsGoodOrigins)
			checkOrigins(t, c, true, false, tt.credsBadOrigins)
			checkOrigins(t, c, false, true, tt.nocredsGoodOrigins)
			checkOrigins(t, c, false, false, tt.nocredsBadOrigins)

			// Only good headers should always be ok
			checkHeaders(t, c, tt.goodHeaders, nil, true)

			// Make sure all the bad headers are invalid, one by one
			for _, vad := range tt.badHeaders {
				headers := append(tt.goodHeaders, vad)
				checkHeaders(t, c, headers, nil, false)
			}
		})
	}
}
