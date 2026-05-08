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
