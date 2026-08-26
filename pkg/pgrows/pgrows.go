// Package pgrows encodes Postgres result sets into the JSON shape the database
// browser dashboards consume.
//
// Both the local dev dashboard (served by the Encore daemon) and the cloud
// dashboard render query results with Drizzle Studio, so they must encode rows
// identically for the same query to render the same way in both.
package pgrows

import (
	"bytes"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Collect reads the remaining rows, ready to be marshaled as the response.
//
// In array mode that's a JSON array per row, holding the values in result set
// order. Otherwise it's a JSON object per row, keyed by column name — also in
// result set order, which is why the object form is marshaled by orderedRows
// rather than built as a map[string]any: encoding/json sorts map keys, so the
// dashboard would show columns alphabetically instead of as queried.
func Collect(rows pgx.Rows, arrayMode bool) (any, error) {
	// Read the column names up front: pgx only guarantees the field
	// descriptions stay valid until the rows are closed, which Next does once
	// it runs out of rows.
	columns := columnNames(rows.FieldDescriptions())

	collected := [][]any{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		collected = append(collected, values)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if arrayMode {
		return collected, nil
	}
	return orderedRows{columns: columns, rows: collected}, nil
}

// columnNames returns the name of each column, in result set order.
func columnNames(fields []pgconn.FieldDescription) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	return names
}

// orderedRows is a result set that marshals to an array of one JSON object per
// row, each keyed by column name in result set order. Holding the columns once
// for the whole result set keeps them out of the per-row representation.
type orderedRows struct {
	columns []string
	rows    [][]any
}

func (r orderedRows) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, row := range r.rows {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := r.marshalRow(&buf, row); err != nil {
			return nil, err
		}
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

func (r orderedRows) marshalRow(buf *bytes.Buffer, row []any) error {
	buf.WriteByte('{')
	for i, value := range row {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(r.columns[i])
		if err != nil {
			return err
		}
		buf.Write(key)
		buf.WriteByte(':')
		val, err := json.Marshal(value)
		if err != nil {
			return err
		}
		buf.Write(val)
	}
	buf.WriteByte('}')
	return nil
}
