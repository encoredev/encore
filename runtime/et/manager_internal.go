package et

import (
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
)

//publicapigen:drop
type Manager struct {
	static *config.Static
	rt     *reqtrack.RequestTracker
}

//publicapigen:drop
func NewManager(static *config.Static, rt *reqtrack.RequestTracker) *Manager {
	return &Manager{static, rt}
}
