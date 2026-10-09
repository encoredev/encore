package mongodb

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/appruntime/shared/testsupport"
)

// Manager manages MongoDB database connections.
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
	_, _, found := mgr.lookup(dbName)
	db = &Database{
		name:   dbName,
		mgr:    mgr,
		noopDB: !found,
	}
	mgr.dbs[dbName] = db
	return db
}

// lookup finds the runtime configuration for the database with the given Encore name.
func (mgr *Manager) lookup(encoreName string) (srv *config.MongoServer, db *config.MongoDatabase, found bool) {
	for _, d := range mgr.runtime.MongoDatabases {
		if d.EncoreName == encoreName {
			return mgr.runtime.MongoServers[d.ServerID], d, true
		}
	}
	return nil, nil, false
}

func (mgr *Manager) Shutdown(p *shutdown.Process) error {
	// Wait for all user code to finish before shutting down databases.
	<-p.ServicesShutdownCompleted.Done()
	<-p.OutstandingTasks.Done()

	var wg sync.WaitGroup
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	wg.Add(len(mgr.dbs))
	for _, db := range mgr.dbs {
		db := db
		go func() {
			defer wg.Done()
			db.shutdown(context.Background())
		}()
	}
	wg.Wait()
	return nil
}

// maxTracedQueryLen is the longest query recorded in a trace event, in bytes.
const maxTracedQueryLen = 4096

// traceStart records the start of a MongoDB call in the current request's
// trace, if any, and returns a function that records its end.
func (db *Database) traceStart(collection, operation string, query any) (end func(error)) {
	curr := db.mgr.rt.Current()
	if curr.Trace == nil || curr.Req == nil {
		return func(error) {}
	}

	eventParams := trace2.EventParams{
		TraceID: curr.Req.TraceID,
		SpanID:  curr.Req.SpanID,
		Goid:    curr.Goctr,
	}
	var q string
	if query != nil {
		q = queryJSON(query)
	}
	startID := curr.Trace.MongoCallStart(trace2.MongoCallStartParams{
		EventParams: eventParams,
		Database:    db.name,
		Collection:  collection,
		Operation:   operation,
		Query:       q,
		Stack:       stack.Build(3),
	})

	return func(err error) {
		curr.Trace.MongoCallEnd(trace2.MongoCallEndParams{
			EventParams: eventParams,
			StartID:     model.TraceEventID(startID),
			Err:         err,
		})
	}
}

// queryJSON renders a filter, pipeline or document as relaxed Extended JSON, for traces.
func queryJSON(v any) string {
	// Wrap the value in a document, as only documents can be
	// marshalled on their own, and then unwrap it again.
	b, err := bson.MarshalExtJSON(bson.D{{Key: "q", Value: v}}, false, false)
	var s string
	if err == nil && len(b) > len(`{"q":}`) {
		s = strings.TrimSpace(string(b[len(`{"q":`) : len(b)-1]))
	} else {
		s = fmt.Sprintf("%v", v)
	}
	if len(s) > maxTracedQueryLen {
		s = s[:maxTracedQueryLen] + "..."
	}
	return s
}
