package sqldb

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"

	"encore.dev/storage/sqldb/internal/stdlibdriver"
)

// RegisterStdlibDriver returns a connection string that can be used with
// the standard library's sql.Open function to connect to the same db.
//
// The connection string should be used with the "encore" driver name:
//
//	connStr := sqldb.RegisterStdlibDriver(myDB)
//	db, err := sql.Open("encore", connStr)
//
// The main use case is to support libraries that expect to call sql.Open
// themselves without exposing the underlying database credentials.
func RegisterStdlibDriver(db *Database) string {
	if db == nil {
		panic("sqldb.StdlibDriver: received nil db")
	}

	// Initialize the standard library integration for this db.
	_ = db.Stdlib()

	return registeredAdapters.Register(db)
}

// combinedDriver combines driver.Driver and driver.DriverContext into a single interface,
// for drivers that support both interfaces.
type combinedDriver interface {
	driver.Driver
	driver.DriverContext
}

var (
	stdlibDriverOnce      sync.Once
	stdlibDriverSingleton combinedDriver
)

// stdlibDriver registers the stdlib driver.
// It uses a sync.Once so it's safe to call multiple times.
func registerStdlibDriver(mgr *Manager) combinedDriver {
	stdlibDriverOnce.Do(func() {
		d := &wrappedDriver{
			parent: stdlibdriver.GetDefaultDriver(),
			mw:     &interceptor{mgr: mgr},
		}
		sql.Register(stdlibDriverName, d)
		stdlibDriverSingleton = d
	})
	return stdlibDriverSingleton
}

const stdlibDriverName = "__encore_stdlib"

// registeredAdapters is the singleton registry of all databases that have been
// registered for stdlib adapters.
var registeredAdapters = &adapterRegistry{
	nameToID: make(map[string]string),
	idToDB:   make(map[string]*Database),
}

type adapterRegistry struct {
	mu       sync.Mutex
	nameToID map[string]string
	idToDB   map[string]*Database
}

func (r *adapterRegistry) Register(db *Database) string {
	registerAdapterDriver(db.mgr)
	r.mu.Lock()
	defer r.mu.Unlock()

	// If it's already registered, return the same identifier.
	if id, ok := r.nameToID[db.name]; ok {
		return id
	}

	ident := fmt.Sprintf("encore/stdlibdriver/%s", db.name)
	r.nameToID[db.name] = ident
	r.idToDB[ident] = db
	return ident
}

func (r *adapterRegistry) Get(ident string) (*Database, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	db, ok := r.idToDB[ident]
	return db, ok
}

const adapterDriverName = "encore"

type adapterDriver struct{}

func (adapterDriver) Open(ident string) (driver.Conn, error) {
	db, ok := registeredAdapters.Get(ident)
	if !ok {
		return nil, fmt.Errorf("sqldb: unknown database %q (did you register it with sqldb.StdlibDriver?)", ident)
	} else if db.noopDB {
		return nil, errNoopDB
	}
	return registerStdlibDriver(db.mgr).Open(db.connStr)
}

func (adapterDriver) OpenConnector(ident string) (driver.Connector, error) {
	db, ok := registeredAdapters.Get(ident)
	if !ok {
		return nil, fmt.Errorf("sqldb: unknown database %q (did you register it with sqldb.StdlibDriver?)", ident)
	} else if db.noopDB {
		return nil, errNoopDB
	}
	return registerStdlibDriver(db.mgr).OpenConnector(db.connStr)
}

var (
	adapterDriverOnce      sync.Once
	adapterDriverSingleton combinedDriver
)

// stdlibDriver registers the stdlib driver.
// It uses a sync.Once so it's safe to call multiple times.
func registerAdapterDriver(mgr *Manager) combinedDriver {
	adapterDriverOnce.Do(func() {
		d := &wrappedDriver{
			parent: adapterDriver{},
			mw:     &interceptor{mgr: mgr},
		}
		sql.Register(adapterDriverName, d)
		adapterDriverSingleton = d
	})
	return adapterDriverSingleton
}
