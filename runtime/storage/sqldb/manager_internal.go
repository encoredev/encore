package sqldb

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
)

// Manager manages database connections.
type Manager struct {
	runtime *config.Runtime
	rt      *reqtrack.RequestTracker
	ts      *testsupport.Manager

	mu  sync.RWMutex
	dbs map[string]*Database
}

func NewManager(runtime *config.Runtime, rt *reqtrack.RequestTracker, ts *testsupport.Manager) *Manager {
	return &Manager{
		runtime: runtime,
		rt:      rt,
		ts:      ts,
		dbs:     make(map[string]*Database),
	}
}

// GetCurrentDB gets the database for the current request.
func (mgr *Manager) GetCurrentDB() *Database {
	var dbName string
	if curr := mgr.rt.Current(); curr.Req != nil {
		dbName = curr.Req.Service()
	} else if testSvc, _ := mgr.ts.TestService(); testSvc != "" {
		dbName = testSvc
	} else {
		panic("sqldb: no current request")
	}
	return mgr.GetDB(dbName)
}

func (mgr *Manager) GetDB(dbName string) *Database {
	mgr.mu.RLock()
	db, ok := mgr.dbs[dbName]
	mgr.mu.RUnlock()
	if ok {
		return db
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	// Check again now that we've re-acquired the mutex
	if db, ok := mgr.dbs[dbName]; ok {
		return db
	}
	pool, found := mgr.getPool(dbName)
	db = &Database{
		name:   dbName,
		mgr:    mgr,
		noopDB: !found,
		pool:   pool,
	}
	mgr.dbs[dbName] = db
	return db
}

// getPool returns a database connection pool for the given database name.
// Each time it's called it returns a new pool.
func (mgr *Manager) getPool(dbName string) (pool *pgxpool.Pool, found bool) {
	var db *config.SQLDatabase
	for _, d := range mgr.runtime.SQLDatabases {
		if d.EncoreName == dbName {
			db = d
			break
		}
	}
	if db == nil {
		return nil, false
	}

	srv := mgr.runtime.SQLServers[db.ServerID]
	cfg, err := dbConf(srv, db)
	if err != nil {
		panic("sqldb: " + err.Error())
	}

	cfg.ConnConfig.Tracer = &pgxTracer{mgr: mgr}
	pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		panic("sqldb: setup db: " + err.Error())
	}

	return pool, true
}

func (mgr *Manager) Shutdown(force context.Context) {
	var wg sync.WaitGroup
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	wg.Add(len(mgr.dbs))
	for _, db := range mgr.dbs {
		db := db
		go func() {
			defer wg.Done()
			db.shutdown(force)
		}()
	}
	wg.Wait()
}

func (mgr *Manager) Named(name string) *Database {
	return mgr.GetDB(name)
}
