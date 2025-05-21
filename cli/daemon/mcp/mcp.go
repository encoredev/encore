package mcp

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/mark3labs/mcp-go/server"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/objects"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/sqldb"
)

type Manager struct {
	server  *server.MCPServer
	sse     *server.SSEServer
	cluster *sqldb.ClusterManager
	ns      *namespace.Manager
	traces  trace2.Store
	run     *run.Manager
	objects *objects.ClusterManager
	apps    *apps.Manager

	BaseURL string
}

type appContextKey struct{}

type appContext struct {
	AppID string
}

func WithAppID(ctx context.Context, appID string) context.Context {
	return context.WithValue(ctx, appContextKey{}, &appContext{AppID: appID})
}

func GetAppID(ctx context.Context) (string, bool) {
	if appCtx, ok := ctx.Value(appContextKey{}).(*appContext); ok {
		return appCtx.AppID, true
	}
	return "", false
}

func NewManager(apps *apps.Manager, cluster *sqldb.ClusterManager, ns *namespace.Manager, traces trace2.Store, runMgr *run.Manager, baseURL string) *Manager {
	// Create hooks for handling session registration
	hooks := &server.Hooks{}

	// Create a new MCP server
	s := server.NewMCPServer(
		"Encore MCP Server",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithHooks(hooks),
	)

	m := &Manager{
		server: s,
		sse: server.NewSSEServer(s,
			server.WithAppendQueryToMessageEndpoint(),
			server.WithKeepAlive(true),
			server.WithHTTPContextFunc(addAppToContext)),
		apps:    apps,
		ns:      ns,
		cluster: cluster,
		traces:  traces,
		run:     runMgr,
		BaseURL: baseURL,
	}

	m.registerDatabaseTools()
	m.registerTraceTools()
	m.registerAPITools()
	m.registerPubSubTools()
	m.registerSrcTools()
	m.registerBucketTools()
	m.registerCacheTools()
	m.registerMetricsTools()
	m.registerCronTools()
	m.registerSecretTools()
	m.registerDocsTools()

	m.registerTraceResources()
	return m
}

func addAppToContext(ctx context.Context, r *http.Request) context.Context {
	if appID := r.URL.Query().Get("app"); appID != "" {
		return WithAppID(ctx, appID)
	}
	return ctx
}

func (m *Manager) Serve(listener net.Listener) error {
	return http.Serve(listener, m.sse)
}

func (m *Manager) getApp(ctx context.Context) (*apps.Instance, error) {
	appID, ok := GetAppID(ctx)
	if !ok {
		return nil, fmt.Errorf("app not found in context")
	}
	inst, err := m.apps.FindLatestByPlatformOrLocalID(appID)
	if err != nil {
		return nil, fmt.Errorf("failed to find app: %w", err)
	}
	return inst, nil
}
