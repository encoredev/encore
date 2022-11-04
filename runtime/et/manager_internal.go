package et

import (
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/reqtrack"
)

//publicapigen:drop
type Manager struct {
	cfg *config.Config
	rt  *reqtrack.RequestTracker
}

//publicapigen:drop
func NewManager(cfg *config.Config, rt *reqtrack.RequestTracker) *Manager {
	return &Manager{cfg, rt}
}
