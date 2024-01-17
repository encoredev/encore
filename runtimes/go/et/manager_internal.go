package et

import (
	"encore.dev/appruntime/apisdk/api"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
)

//publicapigen:drop
type Manager struct {
	static  *config.Static
	rt      *reqtrack.RequestTracker
	testMgr *testsupport.Manager
	server  *api.Server
}

//publicapigen:drop
func NewManager(static *config.Static, rt *reqtrack.RequestTracker, testMgr *testsupport.Manager, server *api.Server) *Manager {
	return &Manager{static, rt, testMgr, server}
}
