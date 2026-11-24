package jsonrpc2_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"encr.dev/cli/internal/jsonrpc2"
)

func TestEncodeNotification(t *testing.T) {
	tests := []struct {
		name   string
		params interface{}
	}{
		{name: "a struct holding raw JSON", params: struct {
			Meta json.RawMessage `json:"meta"`
			Name string          `json:"name"`
		}{Meta: json.RawMessage(`{ "svcs": [ {"name": "a<b"} ] }`), Name: "x&y"}},
		{name: "a map", params: map[string]any{"app_id": "app", "n": 1}},
		{name: "no params", params: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonrpc2.EncodeNotification("process/reload", tt.params)
			if err != nil {
				t.Fatal(err)
			}

			msg, err := jsonrpc2.NewNotification("process/reload", tt.params)
			if err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("got  %s\nwant %s", got, want)
			}
		})
	}
}
