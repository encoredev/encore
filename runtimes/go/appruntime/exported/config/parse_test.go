package config

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	qt "github.com/frankban/quicktest"
)

func gzipData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func TestGZippedContent(t *testing.T) {
	var tests = map[string]struct {
		GZip          bool
		Config        *Runtime
		ProcessConfig *ProcessConfig
		MergedConfig  *Runtime
	}{
		"zipped": {
			GZip: true,
			Config: &Runtime{
				AppSlug: "no-env-ref",
				PubsubTopics: map[string]*PubsubTopic{
					"one": {
						EncoreName: "testTopic1",
					},
				},
			},
		},
		"unzipped": {
			GZip: false,
			Config: &Runtime{
				AppSlug: "test",
				PubsubTopics: map[string]*PubsubTopic{
					"one": {
						EncoreName: "testTopic1",
					},
					"two": {
						EncoreName: "testTopic2",
					},
				},
			},
		},
		"process-config-wo-gw": {
			GZip: false,
			Config: &Runtime{
				AppSlug:        "test",
				HostedServices: []string{"one", "two", "three"},
				Gateways: []Gateway{{
					Name: "test",
				}},
			},
			ProcessConfig: &ProcessConfig{
				HostedServices: []string{"one"},
			},
			MergedConfig: &Runtime{
				AppSlug:        "test",
				HostedServices: []string{"one"},
			},
		},
		"process-config-w-gw": {
			GZip: false,
			Config: &Runtime{
				AppSlug:        "test",
				HostedServices: []string{"one", "two", "three"},
				Gateways: []Gateway{{
					Name: "test",
					Host: "test",
				}},
			},
			ProcessConfig: &ProcessConfig{
				HostedGateways: []string{"test"},
			},
			MergedConfig: &Runtime{
				AppSlug: "test",
				Gateways: []Gateway{{
					Name: "test",
					Host: "test",
				}},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rawData, err := json.Marshal(test.Config)
			if err != nil {
				t.Fatalf("could not marshal test data: %v", err)
			}
			var cfgString string
			if test.GZip {
				data, err := gzipData(rawData)
				if err != nil {
					t.Fatalf("could not gzip data: %v", err)
				}
				cfgString = "gzip:" + base64.StdEncoding.EncodeToString(data)
			} else {
				cfgString = base64.StdEncoding.EncodeToString(rawData)
			}

			expected := test.Config
			procCfg := ""
			if test.ProcessConfig != nil {
				expected = test.MergedConfig
				rawData, err := json.Marshal(test.ProcessConfig)
				if err != nil {
					t.Fatalf("could not marshal process config: %v", err)
				}
				procCfg = base64.StdEncoding.EncodeToString(rawData)
			}
			resp := ParseRuntime(cfgString, "", procCfg, "", "")
			if !reflect.DeepEqual(resp, expected) {
				t.Fatalf("expected %+v, got %+v", test.Config, resp)
			}
		})
	}
}

func TestParseInfraConfigEnv(t *testing.T) {
	c := qt.New(t)

	// Parse the infra config using parseInfraConfigEnv
	parsedRuntime := parseInfraConfigEnv("infra/testdata/infra.config.json")

	// Read the runtime test data file
	runtimeData, err := os.ReadFile("infra/testdata/runtime.json")
	c.Assert(err, qt.IsNil)

	// Unmarshal the runtime JSON data into Runtime
	var expectedRuntime Runtime
	err = json.Unmarshal(runtimeData, &expectedRuntime)
	c.Assert(err, qt.IsNil)

	// Compare the parsed runtime with the expected runtime
	c.Assert(parsedRuntime, qt.DeepEquals, &expectedRuntime)
}

func TestParseInfraConfigEnv_MultipleProviders(t *testing.T) {
	c := qt.New(t)

	infraCfg := `{
  "pubsub": [
    {
      "type": "nsq",
      "hosts": "nsqd:4150",
      "topics": {
        "user-events": {
          "name": "user-events",
          "subscriptions": {"user-event-logger": {"name": "user-event-logger"}}
        }
      }
    },
    {
      "type": "nsq",
      "hosts": "nsqd-log:4150",
      "topics": {
        "login-log-events": {
          "name": "login-log-events",
          "subscriptions": {"login-log-writer": {"name": "login-log-writer"}}
        }
      }
    }
  ],
  "object_storage": [
    {"type": "gcs", "buckets": {"avatars": {"name": "avatars-gcs"}}},
    {"type": "s3", "region": "eu-north-1", "buckets": {"uploads": {"name": "uploads-s3"}}}
  ]
}`
	path := filepath.Join(t.TempDir(), "infra.config.json")
	c.Assert(os.WriteFile(path, []byte(infraCfg), 0o644), qt.IsNil)

	cfg := parseInfraConfigEnv(path)

	// Topics from every provider must be kept, each pointing at its own provider.
	c.Assert(cfg.PubsubProviders, qt.HasLen, 2)
	c.Assert(cfg.PubsubTopics, qt.HasLen, 2)
	c.Assert(cfg.PubsubTopics["user-events"].ProviderID, qt.Equals, 0)
	c.Assert(cfg.PubsubTopics["user-events"].Subscriptions, qt.HasLen, 1)
	c.Assert(cfg.PubsubTopics["login-log-events"].ProviderID, qt.Equals, 1)
	c.Assert(cfg.PubsubTopics["login-log-events"].Subscriptions, qt.HasLen, 1)

	// The same applies to buckets across object storage providers.
	c.Assert(cfg.BucketProviders, qt.HasLen, 2)
	c.Assert(cfg.Buckets, qt.HasLen, 2)
	c.Assert(cfg.Buckets["avatars"].ProviderID, qt.Equals, 0)
	c.Assert(cfg.Buckets["uploads"].ProviderID, qt.Equals, 1)
}
