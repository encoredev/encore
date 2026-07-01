package secrets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	qt "github.com/frankban/quicktest"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/secrets/provider"
	"encore.dev/appruntime/shared/encoreenv"
	"encore.dev/types/option"
)

type fakeProvider struct {
	calls atomic.Int32
	value string
	err   error
}

func (f *fakeProvider) Load(_ context.Context, _ provider.Ref) (string, error) {
	f.calls.Add(1)
	if f.err != nil {
		return "", f.err
	}
	return f.value, nil
}

func newTestManager(t *testing.T, secrets map[string]string, refs map[string]boundRef) *Manager {
	t.Helper()
	return &Manager{
		cfg:     &config.Runtime{EnvCloud: "local"},
		secrets: secrets,
		refs:    refs,
	}
}

func TestLoad_EnvMapTakesPrecedence(t *testing.T) {
	c := qt.New(t)
	fp := &fakeProvider{value: "from-provider"}
	m := newTestManager(t,
		map[string]string{"KEY": "from-env"},
		map[string]boundRef{"KEY": {provider: fp, ID: option.Some("x")}},
	)
	c.Assert(m.Load("KEY", ""), qt.Equals, "from-env")
	c.Assert(fp.calls.Load(), qt.Equals, int32(0))
}

func TestLoad_FallsBackToProvider(t *testing.T) {
	c := qt.New(t)
	fp := &fakeProvider{value: "secret-value"}
	m := newTestManager(t,
		map[string]string{},
		map[string]boundRef{"KEY": {provider: fp, ID: option.Some("x")}},
	)
	c.Assert(m.Load("KEY", ""), qt.Equals, "secret-value")
	c.Assert(fp.calls.Load(), qt.Equals, int32(1))
}

func TestLoad_ProviderResultIsCached(t *testing.T) {
	c := qt.New(t)
	fp := &fakeProvider{value: "secret-value"}
	m := newTestManager(t,
		nil,
		map[string]boundRef{"KEY": {provider: fp, ID: option.Some("x")}},
	)
	c.Assert(m.Load("KEY", ""), qt.Equals, "secret-value")
	c.Assert(m.Load("KEY", ""), qt.Equals, "secret-value")
	c.Assert(m.Load("KEY", ""), qt.Equals, "secret-value")
	c.Assert(fp.calls.Load(), qt.Equals, int32(1))
}

func TestLoad_MissingInLocalReturnsEmpty(t *testing.T) {
	c := qt.New(t)
	m := newTestManager(t, nil, nil)
	c.Assert(m.Load("UNKNOWN", ""), qt.Equals, "")
}

func TestLoad_ProviderErrorInLocalReturnsEmpty(t *testing.T) {
	c := qt.New(t)
	fp := &fakeProvider{err: errors.New("boom")}
	m := newTestManager(t,
		nil,
		map[string]boundRef{"KEY": {provider: fp, ID: option.Some("x")}},
	)
	c.Assert(m.Load("KEY", ""), qt.Equals, "")
}

func TestExpandEnvRef(t *testing.T) {
	c := qt.New(t)

	// No prefix: returned unchanged.
	c.Assert(expandEnvRef("plain-value"), qt.Equals, "plain-value")

	// Chunks are concatenated in order, whitespace around names is tolerated,
	// and a trailing comma produces an empty (skipped) part.
	encoreenv.Set("SPLIT_0", "hel")
	encoreenv.Set("SPLIT_1", "lo")
	c.Assert(expandEnvRef("envref:SPLIT_0, SPLIT_1,"), qt.Equals, "hello")
}

func TestParse_EnvRef(t *testing.T) {
	c := qt.New(t)

	// The whole ENCORE_APP_SECRETS blob is split across env vars and
	// reassembled before parsing: "KEY=" + base64url("world").
	full := "KEY=" + base64.RawURLEncoding.EncodeToString([]byte("world"))
	encoreenv.Set("CHUNK_0", full[:5])
	encoreenv.Set("CHUNK_1", full[5:])

	m := parse("envref:CHUNK_0,CHUNK_1")
	c.Assert(m["KEY"], qt.Equals, "world")
}

func TestParseProviders_Empty(t *testing.T) {
	c := qt.New(t)
	refs := parseProviders("")
	c.Assert(refs, qt.IsNil)
}

func TestParseProviders_RegistersTypes(t *testing.T) {
	c := qt.New(t)

	const typeName = "test_provider_parse"
	fp := &fakeProvider{value: "ok"}
	provider.Register(typeName, func(raw json.RawMessage) (provider.Provider, error) {
		return fp, nil
	})

	cfg := `{
		"providers": {
			"p": {
				"type": "test_provider_parse",
				"refs": {"KEY": {"id": "abc"}}
			}
		}
	}`
	refs := parseProviders(cfg)
	c.Assert(refs["KEY"].ID.MustGet(), qt.Equals, "abc")
	c.Assert(refs["KEY"].provider, qt.Equals, provider.Provider(fp))
}
