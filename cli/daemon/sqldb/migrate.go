package sqldb

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4/source"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type src struct {
	appRoot    string
	svcRelPath string
	migrations []*meta.DBMigration
}

func (src *src) Open(url string) (source.Driver, error) {
	return nil, fmt.Errorf("driver.Open is not implemented")
}

func (src *src) Close() error {
	return nil
}

func (src *src) First() (version uint, err error) {
	if len(src.migrations) == 0 {
		return 0, os.ErrNotExist
	}
	return uint(src.migrations[0].Number), nil
}

func (src *src) Prev(version uint) (prevVersion uint, err error) {
	idx := src.verIdx(version, -1)
	if idx < 0 || idx >= len(src.migrations) {
		return 0, os.ErrNotExist
	}
	return uint(src.migrations[idx].Number), nil
}

func (src *src) Next(version uint) (nextVersion uint, err error) {
	idx := src.verIdx(version, +1)
	if idx >= len(src.migrations) {
		return 0, os.ErrNotExist
	}
	return uint(src.migrations[idx].Number), nil
}

func (src *src) ReadUp(version uint) (r io.ReadCloser, identifier string, err error) {
	idx := src.verIdx(version, 0)
	if idx < 0 || idx >= len(src.migrations) {
		return nil, "", os.ErrNotExist
	}
	m := src.migrations[idx]
	filepath := filepath.Join(src.appRoot, src.svcRelPath, "migrations", m.Filename)
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, "", err
	}
	return io.NopCloser(bytes.NewReader(data)), m.Description, nil
}

func (src *src) ReadDown(version uint) (r io.ReadCloser, identifier string, err error) {
	return nil, "", os.ErrNotExist
}

func (src) verIdx(version uint, offset int) int {
	return int(version) - 1 + offset
}
