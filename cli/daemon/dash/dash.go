// Package dash serves the Encore Developer Dashboard.
package dash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/jsonpb"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/engine/trace"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/internal/jsonrpc2"
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
		var params struct {
			AppID     string
			Path      string
			Method    string
			Payload   []byte
			AuthToken string
			Headers   map[string]interface{} `json:"headers,omitempty"`
		}

		if err := unmarshal(&params); err != nil {
			return reply(ctx, nil, err)
		}
		run := h.run.FindRunByAppID(params.AppID)
		if run == nil {
			log.Error().Str("appID", params.AppID).Msg("dash: cannot make api call: app not running")
			return reply(ctx, nil, fmt.Errorf("app not running"))
		}

		url := "http://" + run.ListenAddr + params.Path
		log := log.With().Str("appID", params.AppID).Str("path", params.Path).Logger()

		req, err := http.NewRequestWithContext(ctx, params.Method, url, bytes.NewReader(params.Payload))
		if err != nil {
			log.Err(err).Msg("dash: api call failed")
			return reply(ctx, nil, err)
		}
		if len(params.Headers) > 0 {
			for k, v := range params.Headers {
				req.Header.Set(k, fmt.Sprintf("%v", v))
			}
		}
		if tok := params.AuthToken; tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
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
