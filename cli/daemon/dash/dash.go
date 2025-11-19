// Package dash serves the Encore Developer Dashboard.
package dash

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/dash/ai"
	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/browser"
	"encr.dev/cli/internal/jsonrpc2"
	"encr.dev/cli/internal/onboarding"
	"encr.dev/cli/internal/telemetry"
	"encr.dev/internal/version"
	"encr.dev/parser/encoding"
	"encr.dev/pkg/editors"
	"encr.dev/pkg/errlist"
	"encr.dev/pkg/jsonext"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type handler struct {
	rpc  jsonrpc2.Conn
	apps *apps.Manager
	run  *run.Manager
	ns   *namespace.Manager
	ai   *ai.Manager
	tr   trace2.Store
}

func (h *handler) GetMeta(appID string) (*meta.Data, error) {
	runInstance := h.run.FindRunByAppID(appID)
	var md *meta.Data
	if runInstance != nil && runInstance.ProcGroup() != nil {
		md = runInstance.ProcGroup().Meta
	} else {
		app, err := h.apps.FindLatestByPlatformOrLocalID(appID)
		if err != nil {
			return nil, err
		}
		md, err = app.CachedMetadata()
		if err != nil {
			return nil, err
		} else if md == nil {
			return nil, err
		}
	}
	return md, nil
}

func (h *handler) GetNamespace(ctx context.Context, appID string) (*namespace.Namespace, error) {
	runInstance := h.run.FindRunByAppID(appID)
	if runInstance != nil && runInstance.ProcGroup() != nil {
		return runInstance.NS, nil
	} else {
		app, err := h.apps.FindLatestByPlatformOrLocalID(appID)
		if err != nil {
			return nil, err
		}
		ns, err := h.ns.GetActive(ctx, app)
		if err != nil {
			return nil, err
		}
		return ns, nil
	}
}

func (h *handler) Handle(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	reply = makeProtoReplier(reply)

	unmarshal := func(dst interface{}) error {
		if r.Params() == nil {
			return fmt.Errorf("missing params")
		}
		return json.Unmarshal([]byte(r.Params()), dst)
	}

	switch r.Method() {
	case "db/query":
		var p QueryRequest
		if err := unmarshal(&p); err != nil {
			return reply(ctx, nil, err)
		}
		res, err := h.Query(ctx, p)
		return reply(ctx, res, err)
	case "db/transaction":
		var p TransactionRequest
		if err := unmarshal(&p); err != nil {
			return reply(ctx, nil, err)
		}
		res, err := h.Transaction(ctx, p)
		return reply(ctx, res, err)
	case "onboarding/get":
		state, err := onboarding.Load()
		if err != nil {
			return reply(ctx, nil, err)
		}
		resp := map[string]time.Time{}
		for key, val := range state.EventMap {
			if val.IsSet() {
				resp[key] = val.UTC()
			}
		}
		return reply(ctx, resp, nil)
	case "onboarding/set":
		type params struct {
			Properties []string `json:"properties"`
		}
		var p params
		if err := unmarshal(&p); err != nil {
			return reply(ctx, nil, err)
		}
		state, err := onboarding.Load()
		if err != nil {
			return reply(ctx, nil, err)
		}
		for _, prop := range p.Properties {
			state.Property(prop).Set()
		}
		err = state.Write()
		if err != nil {
			return reply(ctx, nil, err)
		}
		return reply(ctx, nil, nil)
	case "telemetry":
		type params struct {
			Event      string                 `json:"event"`
			Properties map[string]interface{} `json:"properties"`
			Once       bool                   `json:"once,omitempty"`
		}
		var p params
		if err := unmarshal(&p); err != nil {
			return reply(ctx, nil, err)
		}
		if p.Once {
			telemetry.SendOnce(p.Event, p.Properties)
		} else {
			telemetry.Send(p.Event, p.Properties)
		}
		return reply(ctx, "ok", nil)
	case "version":
		type versionResp struct {
			Version string `json:"version"`
			Channel string `json:"channel"`
		}

		rtn := versionResp{
			Version: version.Version,
			Channel: string(version.Channel),
		}

		return reply(ctx, rtn, nil)

	case "list-apps":
		type app struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			AppRoot string `json:"app_root"`
			Offline bool   `json:"offline,omitempty"`
		}

		apps := []app{} // prevent marshalling as null

		// Load all the apps we know about
		allApp, err := h.apps.List()
		if err != nil {
			return reply(ctx, nil, err)
		}
		for _, instance := range allApp {
			data := app{
				ID:      instance.PlatformOrLocalID(),
				Name:    instance.Name(),
				AppRoot: instance.Root(),
				Offline: true,
			}

			if run := h.run.FindRunByAppID(instance.PlatformOrLocalID()); run != nil {
				data.Offline = false
			}

			apps = append(apps, data)
		}

		// Sort the apps by offline status, then by name
		slices.SortStableFunc(apps, func(a, b app) int {
			if a.Offline == b.Offline {
				return strings.Compare(a.Name, b.Name)
			}
			if a.Offline {
				return 1
			}
			return -1
		})

		return reply(ctx, apps, nil)
	case "traces/clear":
		telemetry.Send("traces.clear")
		var params struct {
			AppID string `json:"app_id"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		err := h.tr.Clear(ctx, params.AppID)
		return reply(ctx, "ok", err)
	case "traces/list":
		telemetry.Send("traces.list")
		var params struct {
			AppID      string `json:"app_id"`
			MessageID  string `json:"message_id"`
			TestTraces *bool  `json:"test_traces,omitempty"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}

		query := &trace2.Query{
			AppID:      params.AppID,
			TestFilter: params.TestTraces,
			MessageID:  params.MessageID,
			Limit:      100,
		}
		var list []*tracepb2.SpanSummary
		iter := func(s *tracepb2.SpanSummary) bool {
			list = append(list, s)
			return true
		}
		err := h.tr.List(ctx, query, iter)
		if err != nil {
			log.Error().Err(err).Msg("dash: could not list traces")
		}
		return reply(ctx, list, err)

	case "traces/get":
		telemetry.Send("traces.get")
		var params struct {
			AppID   string `json:"app_id"`
			TraceID string `json:"trace_id"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}

		var events []*tracepb2.TraceEvent
		iter := func(ev *tracepb2.TraceEvent) bool {
			events = append(events, ev)
			return true
		}
		err := h.tr.Get(ctx, params.AppID, params.TraceID, iter)
		if err != nil {
			log.Error().Err(err).Msg("dash: could not list trace events")
		}
		return reply(ctx, events, err)

	case "status":
		var params struct {
			AppID string
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}

		// Find the latest app by platform ID or local ID.
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			if errors.Is(err, apps.ErrNotFound) {
				return reply(ctx, map[string]interface{}{"running": false}, nil)
			} else {
				return reply(ctx, nil, err)
			}
		}

		// Now find the running instance(s)
		runInstance := h.run.FindRunByAppID(params.AppID)
		status, err := buildAppStatus(app, runInstance)
		if err != nil {
			log.Error().Err(err).Msg("dash: could not build app status")
			return reply(ctx, nil, err)
		}

		return reply(ctx, status, nil)
	case "db-migration-status":
		var params struct {
			AppID string
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}

		// Find the latest app by platform ID or local ID.
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}

		appMeta, err := h.GetMeta(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}

		namespace, err := h.GetNamespace(ctx, params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}

		clusterType := sqldb.Run
		cluster, ok := h.run.ClusterMgr.Get(sqldb.GetClusterID(app, clusterType, namespace))
		if !ok {
			return reply(ctx, []dbMigrationHistory{}, nil)
		}

		status := buildDbMigrationStatus(ctx, appMeta, cluster)

		return reply(ctx, status, nil)
	case "api-call":
		telemetry.Send("api.call")
		var params run.ApiCallParams
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		res, err := run.CallAPI(ctx, h.run.FindRunByAppID(params.AppID), &params)
		return reply(ctx, res, err)

	case "editors/list":
		var resp struct {
			Editors []string `json:"editors"`
		}

		found, err := editors.Resolve(ctx)
		if err != nil {
			log.Err(err).Msg("dash: could not list editors")
			return reply(ctx, nil, err)
		}

		for _, e := range found {
			resp.Editors = append(resp.Editors, string(e.Editor))
		}
		return reply(ctx, resp, nil)
	case "ai/propose-system-design":
		telemetry.Send("ai.propose")
		log.Debug().Msg("dash: propose-system-design")
		var params struct {
			AppID  string `json:"app_id"`
			Prompt string `json:"prompt"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		md, err := h.GetMeta(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		sessionCh := make(chan *ai.AINotification)
		defer close(sessionCh)
		idResp := sync.Once{}
		task, err := h.ai.ProposeSystemDesign(ctx, params.AppID, params.Prompt, md, func(ctx context.Context, msg *ai.AINotification) error {
			if _, ok := msg.Value.(ai.SessionUpdate); ok || msg.Error != nil {
				idResp.Do(func() {
					sessionCh <- msg
				})
				if ok {
					return nil
				}
			}
			return h.rpc.Notify(ctx, r.Method()+"/stream", msg)
		})
		if err != nil {
			return reply(ctx, nil, err)
		}

		select {
		case msg := <-sessionCh:
			su, ok := msg.Value.(ai.SessionUpdate)
			if !ok || msg.Error != nil {
				if msg.Error != nil {
					err = jsonrpc2.NewError(ai.ErrorCodeMap[msg.Error.Code], msg.Error.Message)
				} else {
					err = jsonrpc2.NewError(1, "missing session_id")
				}
				return reply(ctx, nil, err)
			}
			return reply(ctx, map[string]string{
				"session_id":      string(su.Id),
				"subscription_id": task.SubscriptionID,
			}, nil)
		case <-ctx.Done():
			return reply(ctx, nil, ctx.Err())
		case <-time.NewTimer(10 * time.Second).C:
			_ = task.Stop()
			return reply(ctx, nil, errors.New("timed out waiting for response"))
		}

	case "ai/modify-system-design":
		telemetry.Send("ai.modify")
		log.Debug().Msg("dash: modify-system-design")
		var params struct {
			AppID          string         `json:"app_id"`
			SessionID      ai.AISessionID `json:"session_id"`
			OriginalPrompt string         `json:"original_prompt"`
			Prompt         string         `json:"prompt"`
			Proposed       []ai.Service   `json:"proposed"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		md, err := h.GetMeta(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		task, err := h.ai.ModifySystemDesign(ctx, params.AppID, params.SessionID, params.OriginalPrompt, params.Proposed, params.Prompt, md, func(ctx context.Context, msg *ai.AINotification) error {
			return h.rpc.Notify(ctx, r.Method()+"/stream", msg)
		})
		return reply(ctx, task.SubscriptionID, err)
	case "ai/define-endpoints":
		telemetry.Send("ai.details")
		log.Debug().Msg("dash: define-endpoints")
		log.Debug().Msg("dash: define-endpoints")
		var params struct {
			AppID     string         `json:"app_id"`
			SessionID ai.AISessionID `json:"session_id"`
			Prompt    string         `json:"prompt"`
			Proposed  []ai.Service   `json:"proposed"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		md, err := h.GetMeta(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		task, err := h.ai.DefineEndpoints(ctx, params.AppID, params.SessionID, params.Prompt, md, params.Proposed, func(ctx context.Context, msg *ai.AINotification) error {
			return h.rpc.Notify(ctx, r.Method()+"/stream", msg)
		})
		return reply(ctx, task.SubscriptionID, err)
	case "ai/parse-code":
		log.Debug().Msg("dash: parse-code")
		var params struct {
			AppID    string       `json:"app_id"`
			Services []ai.Service `json:"services"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		results, err := h.ai.ParseCode(ctx, params.Services, app)
		return reply(ctx, results, err)
	case "ai/update-code":
		log.Debug().Msg("dash: update-code")
		var params struct {
			AppID     string       `json:"app_id"`
			Services  []ai.Service `json:"services"`
			Overwrite bool         `json:"overwrite"` // Ovwerwrite any existing endpoint code
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		results, err := h.ai.UpdateCode(ctx, params.Services, app, params.Overwrite)
		return reply(ctx, results, err)
	case "ai/preview-files":
		telemetry.Send("ai.preview")
		log.Debug().Msg("dash: preview-files")
		var params struct {
			AppID    string       `json:"app_id"`
			Services []ai.Service `json:"services"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		result, err := h.ai.PreviewFiles(ctx, params.Services, app)
		return reply(ctx, result, err)
	case "ai/write-files":
		telemetry.Send("ai.write")
		log.Debug().Msg("dash: write-files")
		var params struct {
			AppID    string       `json:"app_id"`
			Services []ai.Service `json:"services"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		result, err := h.ai.WriteFiles(ctx, params.Services, app)
		return reply(ctx, result, err)
	case "ai/parse-sql-schema":
		var params struct {
			AppID string `json:"app_id"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		md, err := h.GetMeta(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		for _, db := range md.SqlDatabases {
			_, err := ai.ParseSQLSchema(app, *db.MigrationRelPath)
			if err != nil {
				return reply(ctx, nil, err)
			}
		}
		return reply(ctx, true, err)
	case "editors/open":
		telemetry.Send("editors.open")
		var params struct {
			AppID     string             `json:"app_id"`
			Editor    editors.EditorName `json:"editor"`
			File      string             `json:"file"`
			StartLine int                `json:"start_line,omitempty"`
			StartCol  int                `json:"start_col,omitempty"`
			EndLine   int                `json:"end_line,omitempty"`
			EndCol    int                `json:"end_col,omitempty"`
		}
		if err := unmarshal(&params); err != nil {
			log.Warn().Err(err).Msg("dash: could not parse open command")
			return reply(ctx, nil, err)
		}

		editor, err := editors.Find(ctx, params.Editor)
		if err != nil {
			log.Err(err).Str("editor", string(params.Editor)).Msg("dash: could not find editor")
			return reply(ctx, nil, err)
		}

		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			if errors.Is(err, apps.ErrNotFound) {
				return reply(ctx, nil, fmt.Errorf("app not found, try running encore run"))
			}
			log.Err(err).Str("app_id", params.AppID).Msg("dash: could not find app")
			return reply(ctx, nil, err)
		}

		if !filepath.IsLocal(params.File) {
			log.Warn().Str("file", params.File).Msg("dash: file was not local to the repo")
			return reply(ctx, nil, errors.New("file path must be local"))
		}
		params.File = filepath.Join(app.Root(), params.File)

		if err := editors.LaunchExternalEditor(params.File, params.StartLine, params.StartCol, editor); err != nil {
			log.Err(err).Str("editor", string(params.Editor)).Msg("dash: could not open file")
			return reply(ctx, nil, err)
		}

		type openResp struct{}
		return reply(ctx, openResp{}, nil)
	}

	return jsonrpc2.MethodNotFound(ctx, reply, r)
}

type sourceContextResponse struct {
	Lines []string `json:"lines"`
	Start int      `json:"start"`
}

func (h *handler) listenNotify(ctx context.Context, ch <-chan *notification) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-ch:
			if err := h.rpc.Notify(ctx, r.Method, r.Params); err != nil {
				return
			}
		}
	}
}

func (s *Server) listenTraces() {
	for sp := range s.traceCh {
		// Only marshal the trace if someone's listening.
		s.mu.Lock()
		hasClients := len(s.clients) > 0
		s.mu.Unlock()
		if !hasClients {
			continue
		}

		data, err := jsonext.ProtoEncoder.Marshal(sp.Span)
		if err != nil {
			log.Error().Err(err).Msg("dash: could not marshal trace")
			continue
		}

		s.notify(&notification{
			Method: "trace/new",
			Params: map[string]any{
				"app_id":     sp.AppID,
				"test_trace": sp.TestTrace,
				"span":       json.RawMessage(data),
			},
		})
	}
}

var _ run.EventListener = (*Server)(nil)

// OnStart notifies active websocket clients about the started run.
func (s *Server) OnStart(r *run.Run) {
	status, err := buildAppStatus(r.App, r)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not build app status")
		return
	}

	// Open the browser if needed.
	browserMode := r.Params.Browser
	if browserMode == run.BrowserModeAlways || (browserMode == run.BrowserModeAuto && !s.hasClients()) {
		u := fmt.Sprintf("http://localhost:%d/%s", s.dashPort, r.App.PlatformOrLocalID())
		browser.Open(u)
	}

	s.notify(&notification{
		Method: "process/start",
		Params: status,
	})
}

func (s *Server) OnCompileStart(r *run.Run) {
	status, err := buildAppStatus(r.App, r)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not build app status")
		return
	}

	status.Compiling = true

	s.notify(&notification{
		Method: "process/compile-start",
		Params: status,
	})
}

// OnReload notifies active websocket clients about the reloaded run.
func (s *Server) OnReload(r *run.Run) {
	status, err := buildAppStatus(r.App, r)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not build app status")
		return
	}

	s.notify(&notification{
		Method: "process/reload",
		Params: status,
	})
}

// OnStop notifies active websocket clients about the stopped run.
func (s *Server) OnStop(r *run.Run) {
	status, err := buildAppStatus(r.App, nil)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not build app status")
		return
	}

	s.notify(&notification{
		Method: "process/stop",
		Params: status,
	})
}

// OnStdout forwards the output to active websocket clients.
func (s *Server) OnStdout(r *run.Run, out []byte) {
	s.onOutput(r, out)
}

// OnStderr forwards the output to active websocket clients.
func (s *Server) OnStderr(r *run.Run, out []byte) {
	s.onOutput(r, out)
}

func (s *Server) OnError(r *run.Run, err *errlist.List) {
	if err == nil {
		return
	}

	status, statusErr := buildAppStatus(r.App, nil)
	if statusErr != nil {
		log.Error().Err(statusErr).Msg("dash: could not build app status")
		return
	}

	err.MakeRelative(r.App.Root(), "")

	status.CompileError = err.Error()

	s.notify(&notification{
		Method: "process/compile-error",
		Params: status,
	})
}

func (s *Server) onOutput(r *run.Run, out []byte) {
	// Copy to a new slice since we cannot retain it after the call ends, and notify is async.
	out2 := make([]byte, len(out))
	copy(out2, out)
	s.notify(&notification{
		Method: "process/output",
		Params: map[string]interface{}{
			"appID":  r.App.PlatformOrLocalID(),
			"pid":    r.ID,
			"output": out2,
		},
	})
}

// protoReplier is a jsonrpc2.Replier that wraps another replier and serializes
// any protobuf message with protojson.
func makeProtoReplier(rep jsonrpc2.Replier) jsonrpc2.Replier {
	return func(ctx context.Context, result any, err error) error {
		if err != nil {
			return rep(ctx, nil, err)
		}
		jsonData, err := jsonext.ProtoEncoder.Marshal(result)
		return rep(ctx, json.RawMessage(jsonData), err)
	}
}

// appStatus is the the shared data structure to communicate app status to the client.
//
// It is mirrored in the frontend at src/lib/client/dev-dash-client.ts as `AppStatus`.
type appStatus struct {
	Running      bool                  `json:"running"`
	Tutorial     string                `json:"tutorial,omitempty"`
	AppID        string                `json:"appID"`
	PlatformID   string                `json:"platformID,omitempty"`
	AppRoot      string                `json:"appRoot"`
	PID          string                `json:"pid,omitempty"`
	Meta         json.RawMessage       `json:"meta,omitempty"`
	Addr         string                `json:"addr,omitempty"`
	APIEncoding  *encoding.APIEncoding `json:"apiEncoding,omitempty"`
	Compiling    bool                  `json:"compiling"`
	CompileError string                `json:"compileError,omitempty"`
}

type dbMigrationHistory struct {
	DatabaseName string        `json:"databaseName"`
	Migrations   []dbMigration `json:"migrations"`
}

type dbMigration struct {
	Filename    string `json:"filename"`
	Number      uint64 `json:"number"`
	Description string `json:"description"`
	Applied     bool   `json:"applied"`
}

func buildAppStatus(app *apps.Instance, runInstance *run.Run) (s appStatus, err error) {
	// Now try and grab latest metadata for the app
	var md *meta.Data
	if runInstance != nil {
		proc := runInstance.ProcGroup()
		if proc != nil {
			md = proc.Meta
		}
	}

	if md == nil {
		md, err = app.CachedMetadata()
		if err != nil {
			return appStatus{}, err
		}
	}

	// Convert the metadata into a format we can send to the client
	mdStr := "null"
	var apiEnc *encoding.APIEncoding
	if md != nil {
		m := &jsonpb.Marshaler{OrigName: true, EmitDefaults: true}

		mdStr, err = m.MarshalToString(md)
		if err != nil {
			return appStatus{}, err
		}

		apiEnc = encoding.DescribeAPI(md)
	}

	// Build the response
	resp := appStatus{
		Running:     false,
		Tutorial:    app.Tutorial(),
		AppID:       app.PlatformOrLocalID(),
		PlatformID:  app.PlatformID(),
		Meta:        json.RawMessage(mdStr),
		AppRoot:     app.Root(),
		APIEncoding: apiEnc,
	}
	if runInstance != nil {
		resp.Running = true
		resp.PID = runInstance.ID
		resp.Addr = runInstance.ListenAddr
	}

	return resp, nil
}

func buildDbMigrationStatus(ctx context.Context, appMeta *meta.Data, cluster *sqldb.Cluster) []dbMigrationHistory {
	var statuses []dbMigrationHistory
	for _, dbMeta := range appMeta.SqlDatabases {
		db, ok := cluster.GetDB(dbMeta.Name)
		if !ok {
			// Remote database migration status are not supported yet
			continue
		}
		appliedVersions, err := db.ListAppliedMigrations(ctx)
		if err != nil {
			log.Error().Msgf("failed to list applied migrations for database %s: %v", dbMeta.Name, err)
			continue
		}
		statuses = append(statuses, buildMigrationHistory(dbMeta, appliedVersions))
	}
	return statuses
}

func buildMigrationHistory(dbMeta *meta.SQLDatabase, appliedVersions map[uint64]bool) dbMigrationHistory {
	history := dbMigrationHistory{
		DatabaseName: dbMeta.Name,
		Migrations:   []dbMigration{},
	}
	// Go over migrations from latest to earliest
	sortedMigrations := make([]*meta.DBMigration, len(dbMeta.Migrations))
	copy(sortedMigrations, dbMeta.Migrations)
	slices.SortStableFunc(sortedMigrations, func(a, b *meta.DBMigration) int {
		return int(b.Number - a.Number)
	})
	implicitlyApplied := false
	for _, migration := range sortedMigrations {
		dirty, attempted := appliedVersions[migration.Number]
		applied := attempted && !dirty
		// If the database doesn't allow non-sequential migrations,
		// then any migrations before the last applied will also have
		// been applied even if we don't see them in the database.
		if !dbMeta.AllowNonSequentialMigrations && applied {
			implicitlyApplied = true
		}

		status := dbMigration{
			Filename:    migration.Filename,
			Number:      migration.Number,
			Description: migration.Description,
			Applied:     applied || implicitlyApplied,
		}
		history.Migrations = append(history.Migrations, status)
	}
	return history
}
