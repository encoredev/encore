package sqldb

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/hashicorp/go-multierror"
	"github.com/lib/pq"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type MigrationReader interface {
	Read(*meta.DBMigration) (r io.ReadCloser, err error)
}

func NewOsReader(path string) *OsMigrationReader {
	return &OsMigrationReader{path: path}
}

type OsMigrationReader struct {
	path string
}

func (src *OsMigrationReader) Read(m *meta.DBMigration) (r io.ReadCloser, err error) {
	fpath := filepath.Join(src.path, m.Filename)
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func MultiReadCloser(r ...io.Reader) io.ReadCloser {
	return &multiReadCloser{
		readers:     r,
		multiReader: io.MultiReader(r...),
	}
}

type multiReadCloser struct {
	readers     []io.Reader
	multiReader io.Reader
}

func (m multiReadCloser) Read(p []byte) (n int, err error) {
	return m.multiReader.Read(p)
}

func (m multiReadCloser) Close() error {
	var errs []error
	for _, r := range m.readers {
		if c, ok := r.(io.Closer); !ok {
			continue
		} else if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

var _ io.ReadCloser = (*multiReadCloser)(nil)

func NewMetadataSource(reader MigrationReader, migrations []*meta.DBMigration) *MetadataSource {
	return &MetadataSource{
		MigrationReader: reader,
		migrations:      migrations,
	}
}

type MetadataSource struct {
	MigrationReader
	migrations []*meta.DBMigration
}

func (src *MetadataSource) ReadUp(version uint) (r io.ReadCloser, identifier string, err error) {
	m, err := src.Migration(version, 0)
	if err != nil {
		return nil, "", err
	}
	r, err = src.Read(m)
	if err != nil {
		return nil, "", err
	}
	// This is a hack to make sure that a migration is marked successful in the
	// same statement as it's run. Otherwise we may end up with a finished migration
	// which is marked dirty.
	statement := fmt.Sprintf(
		";\ninsert into schema_migrations (version, dirty) values (%d, false) ON CONFLICT (version) DO UPDATE SET dirty = false;",
		version)
	return MultiReadCloser(
		r,
		strings.NewReader(statement),
	), m.Description, nil

}

func (src *MetadataSource) Open(url string) (source.Driver, error) {
	return nil, fmt.Errorf("driver.Open is not implemented")
}

func (src *MetadataSource) Close() error {
	return nil
}

func (src *MetadataSource) First() (version uint, err error) {
	if len(src.migrations) == 0 {
		return 0, os.ErrNotExist
	}
	return uint(src.migrations[0].Number), nil
}

func (src *MetadataSource) Prev(version uint) (prevVersion uint, err error) {
	m, err := src.Migration(version, -1)
	if err != nil {
		return 0, err
	}
	return uint(m.Number), nil
}

func (src *MetadataSource) Next(version uint) (nextVersion uint, err error) {
	m, err := src.Migration(version, +1)
	if err != nil {
		return 0, err
	}
	return uint(m.Number), nil
}

func (src *MetadataSource) ReadDown(version uint) (r io.ReadCloser, identifier string, err error) {
	return nil, "", os.ErrNotExist
}

func (src *MetadataSource) Migration(version uint, offset int) (*meta.DBMigration, error) {
	idx := slices.IndexFunc(src.migrations, func(m *meta.DBMigration) bool {
		return m.Number == uint64(version)
	})
	if idx < 0 {
		return nil, os.ErrNotExist
	}
	idx += offset
	if idx < 0 || idx >= len(src.migrations) {
		return nil, os.ErrNotExist
	}
	return src.migrations[idx], nil
}

type nonSequentialDbDriver struct {
	*postgres.Postgres
	source          *nonSequentialSource
	schemaName      string
	migrationsTable string
	conn            *sql.Conn
	appliedVersions map[uint64]bool
}

type nonSequentialSource struct {
	*MetadataSource
	dbDriver *nonSequentialDbDriver
}

func NonSequentialMigrator(ctx context.Context, conn *sql.Conn, mdSource *MetadataSource) (database.Driver, source.Driver, error) {
	src := &nonSequentialSource{
		MetadataSource: mdSource,
	}
	db := &nonSequentialDbDriver{
		conn:            conn,
		migrationsTable: "schema_migrations",
		source:          src,
	}
	src.dbDriver = db
	query := `SELECT CURRENT_SCHEMA()`
	if err := conn.QueryRowContext(ctx, query).Scan(&db.schemaName); err != nil {
		return nil, nil, &database.Error{OrigErr: err, Query: []byte(query)}
	}

	if len(db.schemaName) == 0 {
		return nil, nil, postgres.ErrNoSchema
	}

	p, err := postgres.WithConnection(ctx, conn, &postgres.Config{
		MigrationsTable: db.migrationsTable,
		SchemaName:      db.schemaName,
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create migration instance")
	}
	db.Postgres = p
	if err := db.loadAppliedVersions(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to load applied versions")
	}
	return db, src, nil
}

func (p *nonSequentialDbDriver) Version() (version int, dirty bool, err error) {
	if len(p.appliedVersions) == 0 {
		return database.NilVersion, false, nil
	}
	var ok bool
	prevVersion := database.NilVersion
	for _, mg := range p.source.migrations {
		dirty, ok = p.appliedVersions[mg.Number]
		if !ok {
			return prevVersion, false, nil
		} else if dirty {
			return int(mg.Number), true, nil
		}
		prevVersion = int(mg.Number)
	}
	return prevVersion, false, nil
}

func (p *nonSequentialDbDriver) SetVersion(version int, dirty bool) error {
	if dirty {
		return nil
	}
	tx, err := p.conn.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return &database.Error{OrigErr: err, Err: "transaction start failed"}
	}

	if version >= 0 {
		query := `INSERT INTO ` + pq.QuoteIdentifier(p.schemaName) + `.` + pq.QuoteIdentifier(p.migrationsTable) + ` (version, dirty) VALUES ($1, $2) ON CONFLICT (version) DO UPDATE SET dirty = $2`
		if _, err := tx.Exec(query, version, dirty); err != nil {
			if errRollback := tx.Rollback(); errRollback != nil {
				err = multierror.Append(err, errRollback)
			}
			return &database.Error{OrigErr: err, Query: []byte(query)}
		}
	}

	if err := tx.Commit(); err != nil {
		return &database.Error{OrigErr: err, Err: "transaction commit failed"}
	}

	return nil
}

func (p *nonSequentialDbDriver) loadAppliedVersions() error {
	if p.appliedVersions != nil {
		return nil
	}
	p.appliedVersions = map[uint64]bool{}
	query := `SELECT version, dirty FROM ` + pq.QuoteIdentifier(p.schemaName) + `.` + pq.QuoteIdentifier(p.migrationsTable) + ` ORDER BY version`
	rows, err := p.conn.QueryContext(context.Background(), query)
	if err != nil {
		if e, ok := err.(*pq.Error); ok {
			if e.Code.Name() == "undefined_table" {
				return nil
			}
		}
		return &database.Error{OrigErr: err, Query: []byte(query)}
	}
	defer rows.Close()
	var version uint64
	var dirty bool
	for rows.Next() {
		err := rows.Scan(&version, &dirty)
		if err != nil {
			return &database.Error{OrigErr: err, Query: []byte(query)}
		}
		p.appliedVersions[version] = dirty
	}
	return nil
}

func (src *nonSequentialSource) Prev(version uint) (prevVersion uint, err error) {
	m, err := src.Migration(version, -1)
	if err != nil {
		return 0, err
	}
	// If the migration is applied, return this version
	if _, ok := src.dbDriver.appliedVersions[m.Number]; ok {
		return uint(m.Number), nil
	}
	// Otherwise skip to the previous version
	return src.Prev(uint(m.Number))
}

func (src *nonSequentialSource) Next(version uint) (nextVersion uint, err error) {
	m, err := src.Migration(version, +1)
	if err != nil {
		return 0, err
	}
	// If the migration is applied, return the next version
	if _, ok := src.dbDriver.appliedVersions[m.Number]; ok {
		return src.Next(uint(m.Number))
	}
	// Otherwise, return this version
	return uint(m.Number), nil
}
