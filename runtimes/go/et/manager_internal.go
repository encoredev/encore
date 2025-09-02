package et

import (
	"encore.dev/appruntime/apisdk/api"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
	"encore.dev/storage/sqldb"
)

//publicapigen:drop
type Manager struct {
	static  *config.Static
	runtime *config.Runtime
	rt      *reqtrack.RequestTracker
	testMgr *testsupport.Manager
	server  *api.Server
	db      *sqldb.Manager
}

//publicapigen:drop
func NewManager(static *config.Static, runtime *config.Runtime, rt *reqtrack.RequestTracker, testMgr *testsupport.Manager, server *api.Server, db *sqldb.Manager) *Manager {
	if runtime.EnvType != "test" {
		panic("et: cannot create manager in non-test environment")
	}

	return &Manager{static, runtime, rt, testMgr, server, db}
}
