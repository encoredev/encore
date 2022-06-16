package cors

import (
	"net/http"
	"testing"
	_ "unsafe"

	"github.com/rs/cors"

	"encore.dev/runtime/config"
)

func TestOptions(t *testing.T) {
	tests := []struct {
		name               string
		cfg                config.CORS
		credsGoodOrigins   []string
		credsBadOrigins    []string
		nocredsGoodOrigins []string
		nocredsBadOrigins  []string
	}{
		{
			name:               "empty",
			cfg:                config.CORS{},
			credsGoodOrigins:   []string{},
			credsBadOrigins:    []string{"foo.com", "evil.com", "localhost"},
			nocredsGoodOrigins: []string{"foo.com", "localhost", "", "icanhazcheezburger.com"},
			nocredsBadOrigins:  []string{},
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Options(&tt.cfg)
			c := cors.New(got)

			checkOrigins(t, c, true, true, tt.credsGoodOrigins)
			checkOrigins(t, c, true, false, tt.credsBadOrigins)
			checkOrigins(t, c, false, true, tt.nocredsGoodOrigins)
			checkOrigins(t, c, false, false, tt.nocredsBadOrigins)
		})
	}
}

//go:linkname testConfig encore.dev/runtime/config.loadConfig
func testConfig() (*config.Config, error) {
	return nil, nil
}
