package sqldb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

// HashMigrations hashes the name and contents of every migration file, or returns "" when there are none.
func HashMigrations(appRoot string, dbMeta *meta.SQLDatabase) (string, error) {
	if len(dbMeta.Migrations) == 0 || dbMeta.MigrationRelPath == nil {
		return "", nil
	}
	dir := filepath.Join(appRoot, *dbMeta.MigrationRelPath)
	h := sha256.New()
	for _, m := range dbMeta.Migrations {
		data, err := os.ReadFile(filepath.Join(dir, m.Filename))
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", m.Filename, len(data))
		_, _ = h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
