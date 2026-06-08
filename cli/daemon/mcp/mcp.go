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

// serverInstructions is returned to MCP clients in the `initialize` response.
// It tells the model when to reach for these tools. Some clients (e.g. Claude
// Code with tool search) truncate this at 2KB, so keep it concise and put the
// most important guidance first.
const serverInstructions = `Tools for the local Encore app the user is working on (the app served by ` + "`encore run`" + `). Reach for them when working in an Encore codebase: to exercise the live app, inspect its runtime state, or look up Encore's API surface.

RUNTIME — use these instead of shell equivalents; they see live data that files can't:
- call_endpoint: call any API endpoint (auto-starts the app if needed). Use instead of curl. Args: service, endpoint, method, path, payload (JSON body/query/headers/path params), optional auth_token/auth_payload/correlation_id; response includes a trace_id.
- query_database: run SQL against the app's databases. Beats psql round-trips.
- get_traces / get_trace_spans: search recent root traces — filter by service, endpoint, Pub/Sub topic/subscription, error, time range, duration, or parent_trace_id — then fetch full spans by trace_id for deep debugging (e.g. why a request failed).
- get_objects: list objects + metadata in storage buckets.

STATIC STRUCTURE — these read the app model, but for source-defined structure reading the source files directly is usually faster and more reliable: get_metadata, get_services, get_databases, get_pubsub, get_storage_buckets, get_cache_keyspaces, get_cronjobs, get_secrets, get_metrics, get_middleware, get_auth_handlers, get_src_files.

DOCS — when unsure how an Encore feature or API works: search_docs (Algolia-backed), then get_docs to fetch full pages by path.

Pub/Sub verification: a subscription handler runs in a SEPARATE trace from the publishing endpoint — it is NOT a child span of the publish. After call_endpoint, keep its trace_id (T1), then call get_traces with parent_trace_id=T1 (optionally narrowed by topic/subscription) to find the handler's trace directly, and get_trace_spans for full detail. The handler may not have started yet, so poll a few times (~250-500ms apart, cap ~10s).

Only sees the local app; for deployed/production environments, use the Encore Cloud MCP server.`

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
		server.WithInstructions(serverInstructions),
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
