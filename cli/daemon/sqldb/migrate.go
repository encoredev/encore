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

	"github.com/cockroachdb/errors"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/hashicorp/go-multierror"
	"github.com/lib/pq"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type MetadataSource interface {
	source.Driver
	Migration(version uint, offset int) (*meta.DBMigration, error)
	FilePath(filename string) string
}

func NewMetadataSource(path string, migrations []*meta.DBMigration) MetadataSource {
	return &metadataSource{
		path:       path,
		migrations: migrations,
	}
}

type metadataSource struct {
	path       string
	migrations []*meta.DBMigration
}

func (src *metadataSource) FilePath(filename string) string {
	return filepath.Join(src.path, filename)
}

func (src *metadataSource) Open(url string) (source.Driver, error) {
	return nil, fmt.Errorf("driver.Open is not implemented")
}

func (src *metadataSource) Close() error {
	return nil
}

func (src *metadataSource) First() (version uint, err error) {
	if len(src.migrations) == 0 {
		return 0, os.ErrNotExist
	}
	return uint(src.migrations[0].Number), nil
}

func (src *metadataSource) Prev(version uint) (prevVersion uint, err error) {
	m, err := src.Migration(version, -1)
	if err != nil {
		return 0, err
	}
	return uint(m.Number), nil
}

func (src *metadataSource) Next(version uint) (nextVersion uint, err error) {
	m, err := src.Migration(version, +1)
	if err != nil {
		return 0, err
	}
	return uint(m.Number), nil
}

func (src *metadataSource) ReadUp(version uint) (r io.ReadCloser, identifier string, err error) {
	m, err := src.Migration(version, 0)
	if err != nil {
		return nil, "", err
	}
	fpath := src.FilePath(m.Filename)
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, "", err
	}
	return io.NopCloser(bytes.NewReader(data)), m.Description, nil
}

func (src *metadataSource) ReadDown(version uint) (r io.ReadCloser, identifier string, err error) {
	return nil, "", os.ErrNotExist
}

func (src *metadataSource) Migration(version uint, offset int) (*meta.DBMigration, error) {
	idx := slices.IndexFunc(src.migrations, func(m *meta.DBMigration) bool {
		return m.Number == uint64(version)
	}) + offset
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
	*metadataSource
	dbDriver *nonSequentialDbDriver
}

func NonSequentialMigrator(ctx context.Context, conn *sql.Conn, dir string, migrations []*meta.DBMigration) (database.Driver, MetadataSource, error) {
	src := &nonSequentialSource{
		metadataSource: &metadataSource{path: dir, migrations: migrations},
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
	if _, ok := src.dbDriver.appliedVersions[m.Number]; ok {
		return uint(m.Number), nil
	}
	return src.Prev(uint(m.Number))
}

func (src *nonSequentialSource) Next(version uint) (nextVersion uint, err error) {
	m, err := src.Migration(version, +1)
	if err != nil {
		return 0, err
	}
	if _, ok := src.dbDriver.appliedVersions[m.Number]; ok {
		return src.Next(uint(m.Number))
	}
	return uint(m.Number), nil
}
