package runtime

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cockroachdb/errors"

	tracemodel "encore.dev/appruntime/exported/trace2"
	"encr.dev/cli/daemon/engine/trace2"
	"encr.dev/cli/daemon/run"
)

type server struct {
	runMgr *run.Manager
	rec    *trace2.Recorder
}

func NewServer(runMgr *run.Manager, rec *trace2.Recorder) http.Handler {
	s := &server{runMgr: runMgr, rec: rec}
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
	data, err := s.parseTraceData(req)
	if err != nil {
		http.Error(w, "unable to parse trace header: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = s.rec.RecordTrace(data)
	if err != nil {
		http.Error(w, "unable to record trace: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *server) parseTraceData(req *http.Request) (d trace2.RecordData, err error) {
	// Parse trace version
	traceVersion := req.Header.Get("X-Encore-Trace-Version")
	version, err := strconv.Atoi(traceVersion)
	if err != nil || version <= 0 {
		return d, fmt.Errorf("bad trace protocol version %q", traceVersion)
	}
	d.TraceVersion = tracemodel.Version(version)

	// Look up app id
	pid := req.Header.Get("X-Encore-Env-ID")
	if pid == "" {
		return d, errors.New("missing X-Encore-Env-ID header")
	}
	proc := s.runMgr.FindProc(pid)
	if proc == nil {
		return d, errors.Newf("process %q is not running", pid)
	}
	d.Meta = &trace2.Meta{AppID: proc.Run.App.PlatformOrLocalID()}

	// Parse time anchor
	timeAnchor := req.Header.Get("X-Encore-Trace-TimeAnchor")
	if timeAnchor == "" {
		return d, errors.New("missing X-Encore-Trace-TimeAnchor header")
	}

	if err := d.Anchor.UnmarshalText([]byte(timeAnchor)); err != nil {
		return d, errors.Wrap(err, "unable to parse X-Encore-Trace-TimeAnchor header")
	}

	d.Buf = bufio.NewReader(req.Body)
	return d, nil
}
