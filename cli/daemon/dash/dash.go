// Package dash serves the Encore Developer Dashboard.
package dash

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/rs/zerolog/log"
	"github.com/tailscale/hujson"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/dash/ai"
	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/internal/browser"
	"encr.dev/cli/internal/jsonrpc2"
	"encr.dev/internal/version"
	"encr.dev/parser/encoding"
	"encr.dev/pkg/editors"
	"encr.dev/pkg/errlist"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type handler struct {
	rpc  jsonrpc2.Conn
	apps *apps.Manager
	run  *run.Manager
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

func (h *handler) Handle(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	reply = makeProtoReplier(reply)

	unmarshal := func(dst interface{}) error {
		if r.Params() == nil {
			return fmt.Errorf("missing params")
		}
		return json.Unmarshal([]byte(r.Params()), dst)
	}

	switch r.Method() {
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

	case "traces/list":
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

	case "api-call":
		var params apiCallParams
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		return h.apiCall(ctx, reply, &params)

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
		subID, err := h.ai.ProposeSystemDesign(ctx, params.AppID, params.Prompt, md, func(ctx context.Context, msg *ai.WSNotification) error {
			return h.rpc.Notify(ctx, r.Method()+"/stream", msg)
		})
		return reply(ctx, subID, err)
	case "ai/define-endpoints":
		var params struct {
			AppID    string            `json:"app_id"`
			Prompt   string            `json:"prompt"`
			Proposed []ai.ServiceInput `json:"proposed"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		md, err := h.GetMeta(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		subID, err := h.ai.DefineEndpoints(ctx, params.AppID, params.Prompt, md, params.Proposed, func(ctx context.Context, msg *ai.WSNotification) error {
			return h.rpc.Notify(ctx, r.Method()+"/stream", msg)
		})
		return reply(ctx, subID, err)
	case "ai/generate-code":
		var params struct {
			AppID    string            `json:"app_id"`
			Services []ai.ServiceInput `json:"services"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		err = ai.GenerateCode(params.Services, app)
		return reply(ctx, true, err)
	case "ai/validate-code":
		var params struct {
			AppID    string            `json:"app_id"`
			Services []ai.ServiceInput `json:"services"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		app, err := h.apps.FindLatestByPlatformOrLocalID(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		results, err := ai.ValidateCode(params.Services, app)
		return reply(ctx, results, err)
	case "ai/modify-system-design":
		var params struct {
			AppID          string            `json:"app_id"`
			OriginalPrompt string            `json:"original_prompt"`
			Prompt         string            `json:"prompt"`
			Proposed       []ai.ServiceInput `json:"proposed"`
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		md, err := h.GetMeta(params.AppID)
		if err != nil {
			return reply(ctx, nil, err)
		}
		subID, err := h.ai.ModifySystemDesign(ctx, params.AppID, params.OriginalPrompt, params.Proposed, params.Prompt, md, func(ctx context.Context, msg *ai.WSNotification) error {
			return h.rpc.Notify(ctx, r.Method()+"/stream", msg)
		})
		return reply(ctx, subID, err)
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
			err := ai.ParseSQLSchema(app, *db.MigrationRelPath)
			if err != nil {
				return reply(ctx, nil, err)
			}
		}
		return reply(ctx, true, err)
	case "editors/open":
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

type apiCallParams struct {
	AppID         string
	Service       string
	Endpoint      string
	Path          string
	Method        string
	Payload       []byte
	AuthPayload   []byte `json:"auth_payload,omitempty"`
	AuthToken     string `json:"auth_token,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func (h *handler) apiCall(ctx context.Context, reply jsonrpc2.Replier, p *apiCallParams) error {
	log := log.With().Str("app_id", p.AppID).Str("path", p.Path).Str("service", p.Service).Str("endpoint", p.Endpoint).Logger()
	run := h.run.FindRunByAppID(p.AppID)
	if run == nil {
		log.Error().Str("app_id", p.AppID).Msg("dash: cannot make api call: app not running")
		return reply(ctx, nil, fmt.Errorf("app not running"))
	}
	proc := run.ProcGroup()
	if proc == nil {
		log.Error().Str("app_id", p.AppID).Msg("dash: cannot make api call: app not running")
		return reply(ctx, nil, fmt.Errorf("app not running"))
	}

	baseURL := "http://" + run.ListenAddr
	req, err := prepareRequest(ctx, baseURL, proc.Meta, p)
	if err != nil {
		log.Error().Err(err).Msg("dash: unable to prepare request")
		return reply(ctx, nil, err)
	}

	if p.CorrelationID != "" {
		req.Header.Set("X-Correlation-ID", p.CorrelationID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("dash: api call failed")
		return reply(ctx, nil, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Encode the body back into a Go style struct
	if resp.StatusCode == http.StatusOK {
		body = handleResponse(proc.Meta, p, resp.Header, body)
	}

	log.Info().Int("status", resp.StatusCode).Msg("dash: api call completed")
	return reply(ctx, map[string]interface{}{
		"status":      resp.Status,
		"status_code": resp.StatusCode,
		"body":        body,
		"trace_id":    resp.Header.Get("X-Encore-Trace-Id"),
	}, nil)
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

		data, err := protoEncoder.Marshal(sp.Span)
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

// findRPC finds the RPC with the given service and endpoint name.
// If it cannot be found it reports nil.
func findRPC(md *meta.Data, service, endpoint string) *meta.RPC {
	for _, svc := range md.Svcs {
		if svc.Name == service {
			for _, rpc := range svc.Rpcs {
				if rpc.Name == endpoint {
					return rpc
				}
			}
			break
		}
	}
	return nil
}

// prepareRequest prepares a request for sending based on the given apiCallParams.
func prepareRequest(ctx context.Context, baseURL string, md *meta.Data, p *apiCallParams) (*http.Request, error) {
	reqSpec := newHTTPRequestSpec()
	rpc := findRPC(md, p.Service, p.Endpoint)
	if rpc == nil {
		return nil, fmt.Errorf("unknown service/endpoint: %s/%s", p.Service, p.Endpoint)
	}

	rpcEncoding, err := encoding.DescribeRPC(md, rpc, nil)
	if err != nil {
		return nil, fmt.Errorf("describe rpc: %v", err)
	}

	// Add request encoding
	{
		reqEnc := rpcEncoding.RequestEncodingForMethod(p.Method)
		if reqEnc == nil {
			return nil, fmt.Errorf("unsupported method: %s (supports: %s)", p.Method, strings.Join(rpc.HttpMethods, ","))
		}
		if len(p.Payload) > 0 {
			if err := addToRequest(reqSpec, p.Payload, reqEnc.ParameterEncodingMapByName()); err != nil {
				return nil, fmt.Errorf("encode request params: %v", err)
			}
		}
	}

	// Add auth encoding, if any
	if h := md.AuthHandler; h != nil {
		auth, err := encoding.DescribeAuth(md, h.Params, nil)
		if err != nil {
			return nil, fmt.Errorf("describe auth: %v", err)
		}
		if auth.LegacyTokenFormat {
			reqSpec.Header.Set("Authorization", "Bearer "+p.AuthToken)
		} else {
			if err := addToRequest(reqSpec, p.AuthPayload, auth.ParameterEncodingMapByName()); err != nil {
				return nil, fmt.Errorf("encode auth params: %v", err)
			}
		}
	}

	var body io.Reader = nil
	if reqSpec.Body != nil {
		data, _ := json.Marshal(reqSpec.Body)
		body = bytes.NewReader(data)
		if reqSpec.Header["Content-Type"] == nil {
			reqSpec.Header.Set("Content-Type", "application/json")
		}
	}

	reqURL := baseURL + p.Path
	if len(reqSpec.Query) > 0 {
		reqURL += "?" + reqSpec.Query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, p.Method, reqURL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range reqSpec.Header {
		req.Header[k] = v
	}
	for _, c := range reqSpec.Cookies {
		req.AddCookie(c)
	}
	return req, nil
}

func handleResponse(md *meta.Data, p *apiCallParams, headers http.Header, body []byte) []byte {
	rpc := findRPC(md, p.Service, p.Endpoint)
	if rpc == nil {
		return body
	}

	encodingOptions := &encoding.Options{}
	rpcEncoding, err := encoding.DescribeRPC(md, rpc, encodingOptions)
	if err != nil {
		return body
	}

	decoded := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body
	}

	members := make([]hujson.ObjectMember, 0)
	if rpcEncoding.ResponseEncoding != nil {
		for i, m := range rpcEncoding.ResponseEncoding.HeaderParameters {
			value := headers.Get(m.Name)

			var beforeExtra []byte
			if i == 0 {
				beforeExtra = []byte("\n    // HTTP Headers\n    ")
			}

			members = append(members, hujson.ObjectMember{
				Name:  hujson.Value{Value: hujson.String(m.Name), BeforeExtra: beforeExtra},
				Value: hujson.Value{Value: hujson.String(value)},
			})
		}

		for i, m := range rpcEncoding.ResponseEncoding.BodyParameters {
			value, ok := decoded[m.Name]
			if !ok {
				value = []byte("null")
			}

			var beforeExtra []byte
			if i == 0 {
				if len(rpcEncoding.ResponseEncoding.HeaderParameters) > 0 {
					beforeExtra = []byte("\n\n    // JSON Payload\n    ")
				} else {
					beforeExtra = []byte("\n    ")
				}
			}

			// nosemgrep: trailofbits.go.invalid-usage-of-modified-variable.invalid-usage-of-modified-variable
			hValue, err := hujson.Parse(value)
			if err != nil {
				hValue = hujson.Value{Value: hujson.Literal(value)}
			}

			members = append(members, hujson.ObjectMember{
				Name:  hujson.Value{Value: hujson.String(m.Name), BeforeExtra: beforeExtra},
				Value: hValue,
			})
		}
	}

	value := hujson.Value{Value: &hujson.Object{Members: members}}
	value.Format()
	return value.Pack()
}

// httpRequestSpec specifies how the HTTP request should be generated.
type httpRequestSpec struct {
	// Body are the fields to encode as the JSON body.
	// If nil, no body is added.
	Body map[string]json.RawMessage

	// Header are the HTTP headers to set in the request.
	Header http.Header

	// Query are the query string fields to set.
	Query url.Values

	// Cookies are the cookies to send.
	Cookies []*http.Cookie
}

func newHTTPRequestSpec() *httpRequestSpec {
	return &httpRequestSpec{
		Body:   nil, // to distinguish between no body and "{}".
		Header: make(http.Header),
		Query:  make(url.Values),
	}
}

// addToRequest decodes rawPayload and adds it to the request according to the given parameter encodings.
// The body argument is where body parameters are added; other parameter locations are added
// directly to the request object itself.
func addToRequest(req *httpRequestSpec, rawPayload []byte, params map[string][]*encoding.ParameterEncoding) error {
	payload, err := hujson.Parse(rawPayload)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}
	vals, ok := payload.Value.(*hujson.Object)
	if !ok {
		return fmt.Errorf("invalid payload: expected JSON object, got %s", payload.Pack())
	}

	seenKeys := make(map[string]int)

	for _, kv := range vals.Members {
		lit, _ := kv.Name.Value.(hujson.Literal)
		key := lit.String()
		val := kv.Value
		val.Standardize()

		if matches := params[key]; len(matches) > 0 {
			// Get the index of this particular match, in case we have conflicts.
			idx := seenKeys[key]
			seenKeys[key]++
			if idx < len(matches) {
				param := matches[idx]
				switch param.Location {
				case encoding.Body:
					if req.Body == nil {
						req.Body = make(map[string]json.RawMessage)
					}
					req.Body[param.WireFormat] = val.Pack()

				case encoding.Query:
					switch v := val.Value.(type) {
					case hujson.Literal:
						req.Query.Add(param.WireFormat, v.String())
					case *hujson.Array:
						for _, elem := range v.Elements {
							if lit, ok := elem.Value.(hujson.Literal); ok {
								req.Query.Add(param.WireFormat, lit.String())
							} else {
								return fmt.Errorf("unsupported value type for query string array element: %T", elem.Value)
							}
						}
					default:
						return fmt.Errorf("unsupported value type for query string: %T", v)
					}

				case encoding.Header:
					switch v := val.Value.(type) {
					case hujson.Literal:
						req.Header.Add(param.WireFormat, v.String())
					default:
						return fmt.Errorf("unsupported value type for query string: %T", v)
					}

				case encoding.Cookie:
					switch v := val.Value.(type) {
					case hujson.Literal:
						// nosemgrep
						req.Cookies = append(req.Cookies, &http.Cookie{
							Name:  param.WireFormat,
							Value: v.String(),
						})
					default:
						return fmt.Errorf("unsupported value type for cookie: %T", v)
					}

				default:
					return fmt.Errorf("unsupported parameter location %v", param.Location)
				}
			}
		}
	}

	return nil
}

// protoReplier is a jsonrpc2.Replier that wraps another replier and serializes
// any protobuf message with protojson.
func makeProtoReplier(rep jsonrpc2.Replier) jsonrpc2.Replier {
	return func(ctx context.Context, result any, err error) error {
		if err != nil {
			return rep(ctx, nil, err)
		}
		jsonData, err := protoEncoder.Marshal(result)
		return rep(ctx, json.RawMessage(jsonData), err)
	}
}

// appStatus is the the shared data structure to communicate app status to the client.
//
// It is mirrored in the frontend at src/lib/client/dev-dash-client.ts as `AppStatus`.
type appStatus struct {
	Running      bool                  `json:"running"`
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
