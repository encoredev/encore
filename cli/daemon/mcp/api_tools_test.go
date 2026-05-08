package mcp

import (
	"reflect"
	"testing"
)

func TestStringifyBody_ConvertsBytesToString(t *testing.T) {
	in := map[string]any{
		"status": 200,
		"body":   []byte(`{"hello":"world"}`),
	}
	got := stringifyBody(in)
	want := map[string]any{
		"status": 200,
		"body":   `{"hello":"world"}`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestStringifyBody_PassesThroughOtherTypes(t *testing.T) {
	in := map[string]any{"body": "already-a-string"}
	got := stringifyBody(in)
	if got["body"].(string) != "already-a-string" {
		t.Fatalf("unexpected mutation: %+v", got)
	}
}
