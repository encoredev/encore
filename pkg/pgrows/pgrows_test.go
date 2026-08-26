package pgrows

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestOrderedRowsMarshalJSON(t *testing.T) {
	c := qt.New(t)

	// Column names deliberately not in alphabetical order: a map[string]any
	// would serialize them sorted, losing the result set's column order.
	res := orderedRows{
		columns: []string{"name", "id", "created_at"},
		rows: [][]any{
			{"foo <bar>", 1, nil},
			{"baz", 2, "2026-08-26"},
		},
	}

	data, err := json.Marshal(res)
	c.Assert(err, qt.IsNil)
	// Values are HTML-escaped, matching what encoding/json does for the
	// response as a whole.
	c.Assert(string(data), qt.Equals, `[`+
		`{"name":"foo \u003cbar\u003e","id":1,"created_at":null},`+
		`{"name":"baz","id":2,"created_at":"2026-08-26"}`+
		`]`)
}

func TestOrderedRowsMarshalJSONEmpty(t *testing.T) {
	c := qt.New(t)

	// A result set with no rows must still be an empty array, not null, so
	// clients can treat it uniformly.
	data, err := json.Marshal(orderedRows{columns: []string{"id"}, rows: [][]any{}})
	c.Assert(err, qt.IsNil)
	c.Assert(string(data), qt.Equals, `[]`)
}
