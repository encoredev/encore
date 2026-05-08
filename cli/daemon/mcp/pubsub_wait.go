package mcp

import (
	"encoding/json"
	"reflect"
)

// matchPayload reports whether the JSON-encoded payload satisfies the match
// filter. A nil or empty filter always matches. The filter compares top-level
// keys for deep equality only — no nested paths, no wildcards.
func matchPayload(payload []byte, match map[string]any) bool {
	if len(match) == 0 {
		return true
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return false
	}
	for k, want := range match {
		got, ok := decoded[k]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(got, want) {
			return false
		}
	}
	return true
}
