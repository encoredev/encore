package mcp

import "testing"

func TestPredicate_Status(t *testing.T) {
	cases := []struct {
		name   string
		pred   predicate
		status int
		body   string
		want   bool
	}{
		{name: "match", pred: predicate{Status: 200}, status: 200, body: "{}", want: true},
		{name: "mismatch", pred: predicate{Status: 200}, status: 404, body: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.pred.evaluate(tc.status, []byte(tc.body))
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestPredicate_BodyPath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		equals  any
		body    string
		want    bool
		wantErr bool
	}{
		{name: "top-level field", path: ".id", equals: float64(7), body: `{"id":7}`, want: true},
		{name: "nested field", path: ".user.name", equals: "alice", body: `{"user":{"name":"alice"}}`, want: true},
		{name: "array index", path: ".events.0.id", equals: float64(7), body: `{"events":[{"id":7}]}`, want: true},
		{name: "top-level array index", path: ".0.id", equals: float64(7), body: `[{"id":7}]`, want: true},
		{name: "missing key", path: ".missing", equals: nil, body: `{"id":7}`, want: false},
		{name: "value mismatch", path: ".id", equals: float64(8), body: `{"id":7}`, want: false},
		{name: "out-of-bounds index", path: ".events.5", equals: nil, body: `{"events":[]}`, want: false},
		{name: "non-JSON body", path: ".id", equals: float64(7), body: `not json`, want: false, wantErr: false},
		{name: "missing leading dot", path: "id", equals: float64(7), body: `{"id":7}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pred := predicate{Path: &pathPredicate{Path: tc.path, Equals: tc.equals}}
			got, err := pred.evaluate(200, []byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
