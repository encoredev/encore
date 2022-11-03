package sqldb

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/reqtrack"
)

// Manager manages database connections.
type Manager struct {
	rt  *reqtrack.RequestTracker
	cfg *config.Config

	mu  sync.RWMutex
	dbs map[string]*Database

	// Accessed atomically
	txidCtr  uint64
	queryCtr uint64
}

func NewManager(cfg *config.Config, rt *reqtrack.RequestTracker) *Manager {
	return &Manager{
		rt:  rt,
		cfg: cfg,
		dbs: make(map[string]*Database),
	}
}

// GetCurrentDB gets the database for the current request.
func (mgr *Manager) GetCurrentDB() *Database {
	var dbName string
	if curr := mgr.rt.Current(); curr.Req != nil {
		dbName = curr.Req.Service()
	} else if testSvc := mgr.cfg.Static.TestService; testSvc != "" {
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
	db = &Database{
		name: dbName,
		mgr:  mgr,
		pool: mgr.getPool(dbName),
	}
	mgr.dbs[dbName] = db
	return db
}

// getPool returns a database connection pool for the given database name.
// Each time it's called it returns a new pool.
func (mgr *Manager) getPool(dbName string) *pgxpool.Pool {
	var db *config.SQLDatabase
	for _, d := range mgr.cfg.Runtime.SQLDatabases {
		if d.EncoreName == dbName {
			db = d
			break
		}
	}
	if db == nil {
		panic("sqldb: unknown database: " + dbName)
	}

	srv := mgr.cfg.Runtime.SQLServers[db.ServerID]
	cfg, err := dbConf(srv, db)
	if err != nil {
		panic("sqldb: " + err.Error())
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), cfg)
	if err != nil {
		panic("sqldb: setup db: " + err.Error())
	}

	return pool
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
