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
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
)

func TestOptions(t *testing.T) {
	t.Parallel()

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
			goodHeaders:        []string{"Content-Type", "Origin"},
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
				ExtraAllowedHeaders: []string{"Not-Authorization", "X-Forwarded-For", "X-Real-Ip"},
			},
			goodHeaders: []string{"Not-Authorization", "Content-Type", "Origin", "X-Forwarded-For", "X-Real-Ip"},
			badHeaders:  []string{"X-Requested-With", "X-Evil-Header", "Authorization"},
		},
		{
			name: "extra_headers_wildcard",
			cfg: config.CORS{
				ExtraAllowedHeaders: []string{"X-Forwarded-For", "*", "X-Real-Ip"},
			},
			goodHeaders: []string{"Not-Authorization", "Content-Type", "Origin", "X-Forwarded-For", "X-Real-Ip", "X-Requested-With", "X-Evil-Header"},
			badHeaders:  []string{"Authorization"},
		},
		{
			name:        "static_headers",
			cfg:         config.CORS{},
			goodHeaders: []string{"Content-Type", "Origin", "X-Static-Test"},
		},
		{
			name: "wildcard_port_creds",
			cfg: config.CORS{
				AllowOriginsWithCredentials: []string{"http://localhost:*"},
			},
			credsGoodOrigins:   []string{"http://localhost:3000", "http://localhost:8080", "http://localhost:80", "http://localhost"},
			credsBadOrigins:    []string{"https://localhost:3000", "http://evil.com:3000"},
			nocredsGoodOrigins: []string{},
			nocredsBadOrigins:  []string{},
		},
		{
			name: "wildcard_port_without_creds",
			cfg: config.CORS{
				AllowOriginsWithoutCredentials: []string{"http://localhost:*"},
			},
			credsGoodOrigins:   []string{},
			credsBadOrigins:    []string{"http://localhost:3000"},
			nocredsGoodOrigins: []string{"http://localhost:3000", "http://localhost:8080", "http://localhost"},
			nocredsBadOrigins:  []string{"https://localhost:3000"},
		},
		{
			name: "wildcard_without_creds",
			cfg: config.CORS{
				AllowOriginsWithCredentials:    []string{"https://vercel.app"},
				AllowOriginsWithoutCredentials: []string{"https://*-foo.vercel.app"},
			},
			credsBadOrigins:    []string{"https://blah-foo.vercel.app"},
			nocredsGoodOrigins: []string{"https://blah-foo.vercel.app"},
		},
	}

	checkOrigins := func(t *testing.T, c *cors.Cors, creds, good bool, origins []string) {
		for _, o := range origins {
			h := make(http.Header)
			h.Set("Origin", o)
			if creds {
				h.Set("Access-Control-Request-Headers", "authorization")
			}
			allowed := c.OriginAllowed(&http.Request{Header: h, Method: "OPTIONS"})
			if allowed != good {
				t.Fatalf("origin=%s creds=%v: got allowed=%v, want %v", o, creds, allowed, good)
			} else {
				t.Logf("origin=%s creds=%v: ok allowed=%v", o, creds, allowed)
			}
		}
	}

	checkHeaders := func(t *testing.T, c *cors.Cors, allowedHeaders []string, wantOK bool) {
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "https://example.org")
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", strings.ToLower(strings.Join(allowedHeaders, ",")))
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Options(&tt.cfg, []string{"X-Static-Test"}, []string{"X-Exposed-Test"})
			got.Debug = true
			c := cors.New(got)
			c.Log = log.New(os.Stdout, "cors: ", 0)

			checkOrigins(t, c, true, true, tt.credsGoodOrigins)
			checkOrigins(t, c, true, false, tt.credsBadOrigins)
			checkOrigins(t, c, false, true, tt.nocredsGoodOrigins)
			checkOrigins(t, c, false, false, tt.nocredsBadOrigins)

			// Only good headers should always be ok
			checkHeaders(t, c, tt.goodHeaders, true)

			// Make sure all the bad headers are invalid, one by one
			for _, vad := range tt.badHeaders {
				headers := append(tt.goodHeaders, vad)
				checkHeaders(t, c, headers, false)
			}
		})
	}
}

func TestWrapWildcardWithoutCredentials(t *testing.T) {
	t.Parallel()

	cfg := &config.CORS{
		AllowOriginsWithCredentials: []string{"https://trusted.example.com"},
	}
	// Inner handler explicitly writes status and body to commit headers on the wire,
	// verifying that CORS headers are set BEFORE the response is committed.
	handler := Wrap(cfg, nil, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}), zerolog.Nop())

	srv := httptest.NewServer(handler)
	defer srv.Close()
	client := srv.Client()

	// 1. Non-credentialed GET & OPTIONS preflight from unlisted origins match default wildcard:
	// Expect Access-Control-Allow-Origin: * and NO Access-Control-Allow-Credentials header on real wire response.
	for _, origin := range []string{"https://evil.example", "null"} {
		for _, method := range []string{http.MethodGet, http.MethodOptions} {
			req, err := http.NewRequest(method, srv.URL+"/public", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Origin", origin)
			if method == http.MethodOptions {
				req.Header.Set("Access-Control-Request-Method", "POST")
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()

			if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
				t.Fatalf("method=%s origin=%q without creds: got Access-Control-Allow-Origin=%q, want %q", method, origin, got, "*")
			}
			if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
				t.Fatalf("method=%s origin=%q without creds: got Access-Control-Allow-Credentials=%q, want empty", method, origin, got)
			}
		}
	}

	// 2. Credentialed request from unlisted origin is rejected (no CORS headers).
	{
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/public", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Authorization", "Bearer secret")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()

		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("unlisted origin with creds: got Access-Control-Allow-Origin=%q, want empty", got)
		}
		if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("unlisted origin with creds: got Access-Control-Allow-Credentials=%q, want empty", got)
		}
	}

	// 3. Credentialed request from trusted origin reflects Origin and includes credentials header.
	{
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/public", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://trusted.example.com")
		req.Header.Set("Authorization", "Bearer secret")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()

		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://trusted.example.com" {
			t.Fatalf("trusted origin with creds: got Access-Control-Allow-Origin=%q, want %q", got, "https://trusted.example.com")
		}
		if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("trusted origin with creds: got Access-Control-Allow-Credentials=%q, want %q", got, "true")
		}
	}
}
