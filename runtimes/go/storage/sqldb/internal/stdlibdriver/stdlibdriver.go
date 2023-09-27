// Package stdlibdriver is a vendored version of github.com/jackc/pgx/v5/stdlib,
// skipping the stdlib driver registration since we use OpenDB and pass in a driver.
// Its license can be found in ./LICENSE.
package stdlibdriver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Only intrinsic types should be binary format with database/sql.
var databaseSQLResultFormats pgx.QueryResultFormatsByOID

var pgxDriver *Driver

func init() {
	pgxDriver = &Driver{
		configs: make(map[string]*pgx.ConnConfig),
	}

	databaseSQLResultFormats = pgx.QueryResultFormatsByOID{
		pgtype.BoolOID:        1,
		pgtype.ByteaOID:       1,
		pgtype.CIDOID:         1,
		pgtype.DateOID:        1,
		pgtype.Float4OID:      1,
		pgtype.Float8OID:      1,
		pgtype.Int2OID:        1,
		pgtype.Int4OID:        1,
		pgtype.Int8OID:        1,
		pgtype.OIDOID:         1,
		pgtype.TimestampOID:   1,
		pgtype.TimestamptzOID: 1,
		pgtype.XIDOID:         1,
	}
}

// GetDefaultDriver returns the driver initialized in the init function
// and used when the pgx driver is registered.
func GetDefaultDriver() driver.Driver {
	return pgxDriver
}

type Driver struct {
	configMutex sync.Mutex
	configs     map[string]*pgx.ConnConfig
	sequence    int
}

func (d *Driver) Open(name string) (driver.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Ensure eventual timeout
	defer cancel()

	connector, err := d.OpenConnector(name)
	if err != nil {
		return nil, err
	}
	return connector.Connect(ctx)
}

func (d *Driver) OpenConnector(name string) (driver.Connector, error) {
	return &driverConnector{driver: d, name: name}, nil
}

func (d *Driver) registerConnConfig(c *pgx.ConnConfig) string {
	d.configMutex.Lock()
	connStr := fmt.Sprintf("registeredConnConfig%d", d.sequence)
	d.sequence++
	d.configs[connStr] = c
	d.configMutex.Unlock()
	return connStr
}

func (d *Driver) unregisterConnConfig(connStr string) {
	d.configMutex.Lock()
	delete(d.configs, connStr)
	d.configMutex.Unlock()
}

type driverConnector struct {
	driver *Driver
	name   string
}

func (dc *driverConnector) Connect(ctx context.Context) (driver.Conn, error) {
	var connConfig *pgx.ConnConfig

	dc.driver.configMutex.Lock()
	connConfig = dc.driver.configs[dc.name]
	dc.driver.configMutex.Unlock()

	if connConfig == nil {
		var err error
		connConfig, err = pgx.ParseConfig(dc.name)
		if err != nil {
			return nil, err
		}
	}

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, err
	}

	c := &Conn{
		conn:             conn,
		driver:           dc.driver,
		connConfig:       *connConfig,
		resetSessionFunc: func(context.Context, *pgx.Conn) error { return nil },
	}

	return c, nil
}

func (dc *driverConnector) Driver() driver.Driver {
	return dc.driver
}

// RegisterConnConfig registers a ConnConfig and returns the connection string to use with Open.
func RegisterConnConfig(c *pgx.ConnConfig) string {
	return pgxDriver.registerConnConfig(c)
}

// UnregisterConnConfig removes the ConnConfig registration for connStr.
func UnregisterConnConfig(connStr string) {
	pgxDriver.unregisterConnConfig(connStr)
}

type Conn struct {
	conn                 *pgx.Conn
	psCount              int64 // Counter used for creating unique prepared statement names
	driver               *Driver
	connConfig           pgx.ConnConfig
	resetSessionFunc     func(context.Context, *pgx.Conn) error // Function is called before a connection is reused
	lastResetSessionTime time.Time
}

// Conn returns the underlying *pgx.Conn
func (c *Conn) Conn() *pgx.Conn {
	return c.conn
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if c.conn.IsClosed() {
		return nil, driver.ErrBadConn
	}

	name := fmt.Sprintf("pgx_%d", c.psCount)
	c.psCount++

	sd, err := c.conn.Prepare(ctx, name, query)
	if err != nil {
		return nil, err
	}

	return &Stmt{sd: sd, conn: c}, nil
}

func (c *Conn) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return c.conn.Close(ctx)
}

func (c *Conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.conn.IsClosed() {
		return nil, driver.ErrBadConn
	}

	var pgxOpts pgx.TxOptions
	switch sql.IsolationLevel(opts.Isolation) {
	case sql.LevelDefault:
	case sql.LevelReadUncommitted:
		pgxOpts.IsoLevel = pgx.ReadUncommitted
	case sql.LevelReadCommitted:
		pgxOpts.IsoLevel = pgx.ReadCommitted
	case sql.LevelRepeatableRead, sql.LevelSnapshot:
		pgxOpts.IsoLevel = pgx.RepeatableRead
	case sql.LevelSerializable:
		pgxOpts.IsoLevel = pgx.Serializable
	default:
		return nil, fmt.Errorf("unsupported isolation: %v", opts.Isolation)
	}

	if opts.ReadOnly {
		pgxOpts.AccessMode = pgx.ReadOnly
	}

	tx, err := c.conn.BeginTx(ctx, pgxOpts)
	if err != nil {
		return nil, err
	}

	return wrapTx{ctx: ctx, tx: tx}, nil
}

func (c *Conn) ExecContext(ctx context.Context, query string, argsV []driver.NamedValue) (driver.Result, error) {
	if c.conn.IsClosed() {
		return nil, driver.ErrBadConn
	}

	args := namedValueToInterface(argsV)

	commandTag, err := c.conn.Exec(ctx, query, args...)
	// if we got a network error before we had a chance to send the query, retry
	if err != nil {
		if pgconn.SafeToRetry(err) {
			return nil, driver.ErrBadConn
		}
	}
	return driver.RowsAffected(commandTag.RowsAffected()), err
}

func (c *Conn) QueryContext(ctx context.Context, query string, argsV []driver.NamedValue) (driver.Rows, error) {
	if c.conn.IsClosed() {
		return nil, driver.ErrBadConn
	}

	args := []any{databaseSQLResultFormats}
	args = append(args, namedValueToInterface(argsV)...)

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		if pgconn.SafeToRetry(err) {
			return nil, driver.ErrBadConn
		}
		return nil, err
	}

	// Preload first row because otherwise we won't know what columns are available when database/sql asks.
	more := rows.Next()
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	return &Rows{conn: c, rows: rows, skipNext: true, skipNextMore: more}, nil
}

func (c *Conn) Ping(ctx context.Context) error {
	if c.conn.IsClosed() {
		return driver.ErrBadConn
	}

	err := c.conn.Ping(ctx)
	if err != nil {
		// A Ping failure implies some sort of fatal state. The connection is almost certainly already closed by the
		// failure, but manually close it just to be sure.
		c.Close()
		return driver.ErrBadConn
	}

	return nil
}

func (c *Conn) CheckNamedValue(*driver.NamedValue) error {
	// Underlying pgx supports sql.Scanner and driver.Valuer interfaces natively. So everything can be passed through directly.
	return nil
}

func (c *Conn) ResetSession(ctx context.Context) error {
	if c.conn.IsClosed() {
		return driver.ErrBadConn
	}

	now := time.Now()
	if now.Sub(c.lastResetSessionTime) > time.Second {
		if err := c.conn.PgConn().CheckConn(); err != nil {
			return driver.ErrBadConn
		}
	}
	c.lastResetSessionTime = now

	return c.resetSessionFunc(ctx, c.conn)
}

type Stmt struct {
	sd   *pgconn.StatementDescription
	conn *Conn
}

func (s *Stmt) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return s.conn.conn.Deallocate(ctx, s.sd.Name)
}

func (s *Stmt) NumInput() int {
	return len(s.sd.ParamOIDs)
}

func (s *Stmt) Exec(argsV []driver.Value) (driver.Result, error) {
	return nil, errors.New("Stmt.Exec deprecated and not implemented")
}

func (s *Stmt) ExecContext(ctx context.Context, argsV []driver.NamedValue) (driver.Result, error) {
	return s.conn.ExecContext(ctx, s.sd.Name, argsV)
}

func (s *Stmt) Query(argsV []driver.Value) (driver.Rows, error) {
	return nil, errors.New("Stmt.Query deprecated and not implemented")
}

func (s *Stmt) QueryContext(ctx context.Context, argsV []driver.NamedValue) (driver.Rows, error) {
	return s.conn.QueryContext(ctx, s.sd.Name, argsV)
}

type rowValueFunc func(src []byte) (driver.Value, error)

type Rows struct {
	conn         *Conn
	rows         pgx.Rows
	valueFuncs   []rowValueFunc
	skipNext     bool
	skipNextMore bool

	columnNames []string
}

func (r *Rows) Columns() []string {
	if r.columnNames == nil {
		fields := r.rows.FieldDescriptions()
		r.columnNames = make([]string, len(fields))
		for i, fd := range fields {
			r.columnNames[i] = string(fd.Name)
		}
	}

	return r.columnNames
}

// ColumnTypeDatabaseTypeName returns the database system type name. If the name is unknown the OID is returned.
func (r *Rows) ColumnTypeDatabaseTypeName(index int) string {
	if dt, ok := r.conn.conn.TypeMap().TypeForOID(r.rows.FieldDescriptions()[index].DataTypeOID); ok {
		return strings.ToUpper(dt.Name)
	}

	return strconv.FormatInt(int64(r.rows.FieldDescriptions()[index].DataTypeOID), 10)
}

const varHeaderSize = 4

// ColumnTypeLength returns the length of the column type if the column is a
// variable length type. If the column is not a variable length type ok
// should return false.
func (r *Rows) ColumnTypeLength(index int) (int64, bool) {
	fd := r.rows.FieldDescriptions()[index]

	switch fd.DataTypeOID {
	case pgtype.TextOID, pgtype.ByteaOID:
		return math.MaxInt64, true
	case pgtype.VarcharOID, pgtype.BPCharArrayOID:
		return int64(fd.TypeModifier - varHeaderSize), true
	default:
		return 0, false
	}
}

// ColumnTypePrecisionScale should return the precision and scale for decimal
// types. If not applicable, ok should be false.
func (r *Rows) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	fd := r.rows.FieldDescriptions()[index]

	switch fd.DataTypeOID {
	case pgtype.NumericOID:
		mod := fd.TypeModifier - varHeaderSize
		precision = int64((mod >> 16) & 0xffff)
		scale = int64(mod & 0xffff)
		return precision, scale, true
	default:
		return 0, 0, false
	}
}

// ColumnTypeScanType returns the value type that can be used to scan types into.
func (r *Rows) ColumnTypeScanType(index int) reflect.Type {
	fd := r.rows.FieldDescriptions()[index]

	switch fd.DataTypeOID {
	case pgtype.Float8OID:
		return reflect.TypeOf(float64(0))
	case pgtype.Float4OID:
		return reflect.TypeOf(float32(0))
	case pgtype.Int8OID:
		return reflect.TypeOf(int64(0))
	case pgtype.Int4OID:
		return reflect.TypeOf(int32(0))
	case pgtype.Int2OID:
		return reflect.TypeOf(int16(0))
	case pgtype.BoolOID:
		return reflect.TypeOf(false)
	case pgtype.NumericOID:
		return reflect.TypeOf(float64(0))
	case pgtype.DateOID, pgtype.TimestampOID, pgtype.TimestamptzOID:
		return reflect.TypeOf(time.Time{})
	case pgtype.ByteaOID:
		return reflect.TypeOf([]byte(nil))
	default:
		return reflect.TypeOf("")
	}
}

func (r *Rows) Close() error {
	r.rows.Close()
	return r.rows.Err()
}

func (r *Rows) Next(dest []driver.Value) error {
	m := r.conn.conn.TypeMap()
	fieldDescriptions := r.rows.FieldDescriptions()

	if r.valueFuncs == nil {
		r.valueFuncs = make([]rowValueFunc, len(fieldDescriptions))

		for i, fd := range fieldDescriptions {
			dataTypeOID := fd.DataTypeOID
			format := fd.Format

			switch fd.DataTypeOID {
			case pgtype.BoolOID:
				var d bool
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return d, err
				}
			case pgtype.ByteaOID:
				var d []byte
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return d, err
				}
			case pgtype.CIDOID, pgtype.OIDOID, pgtype.XIDOID:
				var d pgtype.Uint32
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					if err != nil {
						return nil, err
					}
					return d.Value()
				}
			case pgtype.DateOID:
				var d pgtype.Date
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					if err != nil {
						return nil, err
					}
					return d.Value()
				}
			case pgtype.Float4OID:
				var d float32
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return float64(d), err
				}
			case pgtype.Float8OID:
				var d float64
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return d, err
				}
			case pgtype.Int2OID:
				var d int16
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return int64(d), err
				}
			case pgtype.Int4OID:
				var d int32
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return int64(d), err
				}
			case pgtype.Int8OID:
				var d int64
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return d, err
				}
			case pgtype.JSONOID, pgtype.JSONBOID:
				var d []byte
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					if err != nil {
						return nil, err
					}
					return d, nil
				}
			case pgtype.TimestampOID:
				var d pgtype.Timestamp
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					if err != nil {
						return nil, err
					}
					return d.Value()
				}
			case pgtype.TimestamptzOID:
				var d pgtype.Timestamptz
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					if err != nil {
						return nil, err
					}
					return d.Value()
				}
			default:
				var d string
				scanPlan := m.PlanScan(dataTypeOID, format, &d)
				r.valueFuncs[i] = func(src []byte) (driver.Value, error) {
					err := scanPlan.Scan(src, &d)
					return d, err
				}
			}
		}
	}

	var more bool
	if r.skipNext {
		more = r.skipNextMore
		r.skipNext = false
	} else {
		more = r.rows.Next()
	}

	if !more {
		if r.rows.Err() == nil {
			return io.EOF
		} else {
			return r.rows.Err()
		}
	}

	for i, rv := range r.rows.RawValues() {
		if rv != nil {
			var err error
			dest[i], err = r.valueFuncs[i](rv)
			if err != nil {
				return fmt.Errorf("convert field %d failed: %v", i, err)
			}
		} else {
			dest[i] = nil
		}
	}

	return nil
}

func namedValueToInterface(argsV []driver.NamedValue) []any {
	args := make([]any, 0, len(argsV))
	for _, v := range argsV {
		if v.Value != nil {
			args = append(args, v.Value.(any))
		} else {
			args = append(args, nil)
		}
	}
	return args
}

type wrapTx struct {
	ctx context.Context
	tx  pgx.Tx
}

func (wtx wrapTx) Commit() error { return wtx.tx.Commit(wtx.ctx) }

func (wtx wrapTx) Rollback() error { return wtx.tx.Rollback(wtx.ctx) }
