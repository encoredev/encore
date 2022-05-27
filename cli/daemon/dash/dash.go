// Package dash serves the Encore Developer Dashboard.
package dash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/jsonpb"
	"github.com/rs/zerolog/log"
	"github.com/tailscale/hujson"

	"encr.dev/cli/daemon/engine/trace"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/internal/jsonrpc2"
	"encr.dev/parser/encoding"
	v1 "encr.dev/proto/encore/parser/meta/v1"
)

type handler struct {
	rpc jsonrpc2.Conn
	run *run.Manager
	tr  *trace.Store
}

func (h *handler) Handle(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	unmarshal := func(dst interface{}) error {
		if r.Params() == nil {
			return fmt.Errorf("missing params")
		}
		return json.Unmarshal([]byte(r.Params()), dst)
	}

	switch r.Method() {
	case "list-apps":
		type app struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		runs := h.run.ListRuns()
		apps := []app{} // prevent marshalling as null
		seen := make(map[string]bool)
		for _, r := range runs {
			id := r.AppID
			name := r.AppSlug
			if name == "" {
				name = filepath.Base(r.Root)
			}
			if !seen[id] {
				seen[id] = true
				apps = append(apps, app{ID: id, Name: name})
			}
		}
		return reply(ctx, apps, nil)

	case "list-traces":
		var params struct {
			AppID string
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		traces := h.tr.List(params.AppID)
		tr := make([]*Trace, len(traces))
		for i, t := range traces {
			tt, err := TransformTrace(t)
			if err != nil {
				return reply(ctx, nil, err)
			}
			tr[i] = tt
		}
		return reply(ctx, tr, nil)

	case "status":
		var params struct {
			AppID string
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}

		run := h.run.FindRunByAppID(params.AppID)
		if run == nil {
			return reply(ctx, map[string]interface{}{"running": false}, nil)
		}
		proc := run.Proc()
		if proc == nil {
			return reply(ctx, map[string]interface{}{"running": false}, nil)
		}

		m := &jsonpb.Marshaler{OrigName: true, EmitDefaults: true}
		str, err := m.MarshalToString(proc.Meta)
		if err != nil {
			log.Error().Err(err).Msg("dash: could not marshal app metadata")
			return reply(ctx, nil, err)
		}

		return reply(ctx, map[string]interface{}{
			"running": true,
			"appID":   run.AppID,
			"pid":     run.ID,
			"meta":    json.RawMessage(str),
			"addr":    run.ListenAddr,
		}, nil)

	case "api-call":
		var params apiCallParams
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		return h.apiCall(ctx, reply, &params)

	case "source-context":
		var params struct {
			AppID string
			File  string
			Line  int
		}
		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		f, err := os.Open(params.File)
		if err != nil {
			return reply(ctx, nil, err)
		}
		defer f.Close()
		lines, start, err := sourceContext(f, params.Line, 5)
		if err != nil {
			return reply(ctx, nil, err)
		}
		return reply(ctx, sourceContextResponse{Lines: lines, Start: start}, nil)
	}

	return jsonrpc2.MethodNotFound(ctx, reply, r)
}

type apiCallParams struct {
	AppID       string
	Service     string
	Endpoint    string
	Path        string
	Method      string
	Payload     []byte
	AuthPayload []byte `json:"auth_payload,omitempty"`
	AuthToken   string `json:"auth_token,omitempty"`
}

func (h *handler) apiCall(ctx context.Context, reply jsonrpc2.Replier, p *apiCallParams) error {
	log := log.With().Str("appID", p.AppID).Str("path", p.Path).Str("service", p.Service).Str("endpoint", p.Endpoint).Logger()
	run := h.run.FindRunByAppID(p.AppID)
	if run == nil {
		log.Error().Str("appID", p.AppID).Msg("dash: cannot make api call: app not running")
		return reply(ctx, nil, fmt.Errorf("app not running"))
	}
	proc := run.Proc()
	if proc == nil {
		log.Error().Str("appID", p.AppID).Msg("dash: cannot make api call: app not running")
		return reply(ctx, nil, fmt.Errorf("app not running"))
	}

	baseURL := "http://" + run.ListenAddr
	req, err := prepareRequest(ctx, baseURL, proc.Meta, p)
	if err != nil {
		log.Error().Err(err).Msg("dash: unable to prepare request")
		return reply(ctx, nil, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("dash: api call failed")
		return reply(ctx, nil, err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	log.Info().Int("status", resp.StatusCode).Msg("dash: api call completed")
	return reply(ctx, map[string]interface{}{
		"status":      resp.Status,
		"status_code": resp.StatusCode,
		"body":        body,
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
	for tt := range s.traceCh {
		// Transforming a trace is fairly expensive, so only do it
		// if somebody is listening.
		s.mu.Lock()
		hasClients := len(s.clients) > 0
		s.mu.Unlock()
		if !hasClients {
			continue
		}

		tr, err := TransformTrace(tt)
		if err != nil {
			log.Error().Err(err).Msg("dash: could not process trace")
			continue
		}
		s.notify(&notification{
			Method: "trace/new",
			Params: tr,
		})
	}
}

var _ run.EventListener = (*Server)(nil)

// OnStart notifies active websocket clients about the started run.
func (s *Server) OnStart(r *run.Run) {
	m := &jsonpb.Marshaler{OrigName: true, EmitDefaults: true}
	proc := r.Proc()
	str, err := m.MarshalToString(proc.Meta)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not marshal app meta")
		return
	}

	s.notify(&notification{
		Method: "process/start",
		Params: map[string]interface{}{
			"appID": r.AppID,
			"pid":   r.ID,
			"addr":  r.ListenAddr,
			"meta":  json.RawMessage(str),
		},
	})
}

// OnReload notifies active websocket clients about the reloaded run.
func (s *Server) OnReload(r *run.Run) {
	m := &jsonpb.Marshaler{OrigName: true, EmitDefaults: true}
	proc := r.Proc()
	str, err := m.MarshalToString(proc.Meta)
	if err != nil {
		log.Error().Err(err).Msg("dash: could not marshal app meta")
		return
	}
	s.notify(&notification{
		Method: "process/reload",
		Params: map[string]interface{}{
			"appID": r.AppID,
			"pid":   r.ID,
			"meta":  json.RawMessage(str),
		},
	})
}

// OnStop notifies active websocket clients about the stopped run.
func (s *Server) OnStop(r *run.Run) {
	s.notify(&notification{
		Method: "process/stop",
		Params: map[string]interface{}{
			"appID": r.AppID,
			"pid":   r.ID,
		},
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

func (s *Server) onOutput(r *run.Run, out []byte) {
	// Copy to a new slice since we cannot retain it after the call ends, and notify is async.
	out2 := make([]byte, len(out))
	copy(out2, out)
	s.notify(&notification{
		Method: "process/output",
		Params: map[string]interface{}{
			"appID":  r.AppID,
			"pid":    r.ID,
			"output": out2,
		},
	})
}

// findRPC finds the RPC with the given service and endpoint name.
// If it cannot be found it reports nil.
func findRPC(md *v1.Data, service, endpoint string) *v1.RPC {
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
func prepareRequest(ctx context.Context, baseURL string, md *v1.Data, p *apiCallParams) (*http.Request, error) {
	reqSpec := newHTTPRequestSpec()
	rpc := findRPC(md, p.Service, p.Endpoint)
	if rpc == nil {
		return nil, fmt.Errorf("unknown service/endpoint: %s/%s", p.Service, p.Endpoint)
	}

	encodingOptions := &encoding.Options{}
	rpcEncoding, err := encoding.DescribeRPC(md, rpc, encodingOptions)
	if err != nil {
		return nil, fmt.Errorf("describe rpc: %v", err)
	}

	// Add request encoding
	{
		reqEnc := rpcEncoding.RequestEncodingForMethod(p.Method)
		if reqEnc == nil {
			return nil, fmt.Errorf("unsupported method: %s", p.Method)
		}
		if err := addToRequest(reqSpec, p.Payload, reqEnc.ParameterEncodingMap()); err != nil {
			return nil, fmt.Errorf("encode request params: %v", err)
		}
	}

	// Add auth encoding, if any
	if h := md.AuthHandler; h != nil {
		auth, err := encoding.DescribeAuth(md, h.Params, encodingOptions)
		if err != nil {
			return nil, fmt.Errorf("describe auth: %v", err)
		}
		if auth.LegacyTokenFormat {
			reqSpec.Header.Set("Authorization", "Bearer "+p.AuthToken)
		} else {
			if err := addToRequest(reqSpec, p.AuthPayload, auth.ParameterEncodingMap()); err != nil {
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
	return req, nil
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
func addToRequest(req *httpRequestSpec, rawPayload []byte, params map[string]*encoding.ParameterEncoding) error {
	payload, err := hujson.Parse(rawPayload)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}
	vals, ok := payload.Value.(*hujson.Object)
	if !ok {
		return fmt.Errorf("invalid payload: expected JSON object, got %s", payload.Pack())
	}

	for _, kv := range vals.Members {
		lit, _ := kv.Name.Value.(hujson.Literal)
		key := lit.String()
		val := kv.Value
		val.Standardize()

		if param, ok := params[key]; ok {
			switch param.Location {
			case encoding.Body:
				if req.Body == nil {
					req.Body = make(map[string]json.RawMessage)
				}
				req.Body[param.Name] = val.Pack()

			case encoding.Query:
				switch v := val.Value.(type) {
				case hujson.Literal:
					req.Query.Add(param.Name, v.String())
				case *hujson.Array:
					for _, elem := range v.Elements {
						if lit, ok := elem.Value.(hujson.Literal); ok {
							req.Query.Add(param.Name, lit.String())
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
					req.Header.Add(param.Name, v.String())
				default:
					return fmt.Errorf("unsupported value type for query string: %T", v)
				}

			default:
				return fmt.Errorf("unsupported parameter location %v", param.Location)
			}
		}
	}

	return nil
}
