package config

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
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
