package runtime

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/runtime/trace"
	"github.com/rs/zerolog/log"
)

type server struct {
	runMgr *run.Manager
	ts     *trace.Store
}

func NewServer(runMgr *run.Manager, ts *trace.Store) http.Handler {
	s := &server{runMgr: runMgr, ts: ts}
	return s
}

// ServeHTTP implements http.Handler.
func (s *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/trace":
		s.RecordTrace(w, req)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (s *server) RecordTrace(w http.ResponseWriter, req *http.Request) {
	pid := req.Header.Get("X-Encore-Proc-ID")
	if pid == "" {
		http.Error(w, "missing X-Encore-Proc-ID header", http.StatusBadRequest)
		return
	}
	traceID, err := parseTraceID(req.Header.Get("X-Encore-Trace-ID"))
	if err != nil {
		http.Error(w, "invalid X-Encore-Trace-ID header: "+err.Error(), http.StatusBadRequest)
		return
	}

	proc := s.runMgr.FindProc(pid)
	if proc == nil {
		http.Error(w, "process "+pid+" not running", http.StatusBadRequest)
		return
	}

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reqs, err := trace.Parse(traceID, data, proc)
	if err != nil {
		log.Error().Err(err).Msg("runtime: could not parse trace")
		http.Error(w, "could not parse trace: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(reqs) == 0 {
		// Probably a 401 Unauthorized; drop it for now
		// since we can't visualize it nicely
		return
	}

	tm := &trace.TraceMeta{
		ID:      traceID,
		Reqs:    reqs,
		AppID:   proc.Run.AppID,
		AppRoot: proc.Run.Root,
		Date:    time.Now(),
		Meta:    proc.Meta,
	}

	err = s.ts.Store(req.Context(), tm)
	if err != nil {
		http.Error(w, "could not record trace:"+err.Error(), http.StatusInternalServerError)
		return
	}
}

func parseTraceID(s string) (id trace.ID, err error) {
	parsedID, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return id, err
	}
	if len(parsedID) != len(id) {
		return id, fmt.Errorf("bad length")
	}
	copy(id[:], parsedID)
	return id, nil
}
