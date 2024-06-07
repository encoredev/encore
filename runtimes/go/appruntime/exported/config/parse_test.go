package config

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
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
		GZip   bool
		Config *Runtime
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
			resp := ParseRuntime(cfgString, "")
			if !reflect.DeepEqual(resp, test.Config) {
				t.Fatalf("expected %v, got %v", test.Config, resp)
			}
		})
	}
}
