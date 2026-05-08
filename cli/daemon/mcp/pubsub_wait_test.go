package mcp

import "testing"

func TestMatchPayload_TopLevelEquality(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		match   map[string]any
		want    bool
	}{
		{
			name:    "no match map matches anything",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   nil,
			want:    true,
		},
		{
			name:    "empty match map matches anything",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   map[string]any{},
			want:    true,
		},
		{
			name:    "single key matches",
			payload: []byte(`{"customerID":"cust_42","amount":10}`),
			match:   map[string]any{"customerID": "cust_42"},
			want:    true,
		},
		{
			name:    "single key mismatches",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   map[string]any{"customerID": "cust_99"},
			want:    false,
		},
		{
			name:    "missing key mismatches",
			payload: []byte(`{"customerID":"cust_42"}`),
			match:   map[string]any{"orderID": 7},
			want:    false,
		},
		{
			name:    "number equality with json.Number-like decoding",
			payload: []byte(`{"orderID":7}`),
			match:   map[string]any{"orderID": float64(7)},
			want:    true,
		},
		{
			name:    "all keys must match",
			payload: []byte(`{"a":1,"b":2}`),
			match:   map[string]any{"a": float64(1), "b": float64(3)},
			want:    false,
		},
		{
			name:    "non-JSON payload never matches a non-empty filter",
			payload: []byte("not json"),
			match:   map[string]any{"a": float64(1)},
			want:    false,
		},
		{
			name:    "non-JSON payload matches an empty filter",
			payload: []byte("not json"),
			match:   map[string]any{},
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchPayload(tc.payload, tc.match)
			if got != tc.want {
				t.Fatalf("matchPayload() = %v, want %v", got, tc.want)
			}
		})
	}
}
