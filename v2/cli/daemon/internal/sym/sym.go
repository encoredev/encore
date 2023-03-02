// Package sym parses symbol tables from Go binaries.
package sym

import (
	"fmt"
	"io"

	"encr.dev/v2/cli/internal/gosym"
)

type Table struct {
	*gosym.Table
	BaseOffset uint64
}

func Load(r io.ReaderAt) (*Table, error) {
	tbl, err := load(r)
	if err != nil {
		return nil, fmt.Errorf("sym.Load: %v", err)
	}
	return tbl, nil
}
