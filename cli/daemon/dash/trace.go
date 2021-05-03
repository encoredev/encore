package dash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"encr.dev/cli/daemon/internal/env"
	"encr.dev/cli/daemon/runtime/trace"
	"encr.dev/cli/internal/dedent"
	tracepb "encr.dev/proto/encore/engine/trace"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/jsonpb"
)

type Trace struct {
	ID        uuid.UUID `json:"id"`
	Date      time.Time `json:"date"`
	StartTime int64     `json:"start_time"`
	EndTime   int64     `json:"end_time"`
	Root      *Request  `json:"root"`
	Auth      *Request  `json:"auth"`
	UID       *string   `json:"uid"`
	UserData  []byte    `json:"user_data"`

	Locations map[int32]json.RawMessage `json:"locations"`
}

type Request struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	ParentID  *string    `json:"parent_id"`
	Goid      uint32     `json:"goid"`
	StartTime int64      `json:"start_time"`
	EndTime   *int64     `json:"end_time,omitempty"`
	CallLoc   *int32     `json:"call_loc"`
	DefLoc    int32      `json:"def_loc"`
	Inputs    [][]byte   `json:"inputs"`
	Outputs   [][]byte   `json:"outputs"`
	Err       []byte     `json:"err"`
	ErrStack  *Stack     `json:"err_stack"`
	Events    []Event    `json:"events"`
	Children  []*Request `json:"children"`
}

type Goroutine struct {
	Type      string `json:"type"`
	Goid      uint32 `json:"goid"`
	CallLoc   int32  `json:"call_loc"`
	StartTime int64  `json:"start_time"`
	EndTime   *int64 `json:"end_time,omitempty"`
}

type DBTransaction struct {
	Type           string     `json:"type"`
	Goid           uint32     `json:"goid"`
	Txid           uint32     `json:"txid"`
	StartLoc       int32      `json:"start_loc"`
	EndLoc         int32      `json:"end_loc"`
	StartTime      int64      `json:"start_time"`
	EndTime        *int64     `json:"end_time,omitempty"`
	Err            []byte     `json:"err"`
	CompletionType string     `json:"completion_type"`
	Queries        []*DBQuery `json:"queries"`
	BeginStack     Stack      `json:"begin_stack"`
	EndStack       *Stack     `json:"end_stack"`
}

type DBQuery struct {
	Type      string  `json:"type"`
	Goid      uint32  `json:"goid"`
	Txid      *uint32 `json:"txid"`
	CallLoc   int32   `json:"call_loc"`
	StartTime int64   `json:"start_time"`
	EndTime   *int64  `json:"end_time,omitempty"`
	Query     []byte  `json:"query"`
	HTMLQuery []byte  `json:"html_query"`
	Err       []byte  `json:"err"`
	Stack     Stack   `json:"stack"`
}

type RPCCall struct {
	Type      string `json:"type"`
	Goid      uint32 `json:"goid"`
	ReqID     string `json:"req_id"`
	CallLoc   int32  `json:"call_loc"`
	DefLoc    int32  `json:"def_loc"`
	StartTime int64  `json:"start_time"`
	EndTime   *int64 `json:"end_time,omitempty"`
	Err       []byte `json:"err"`
	Stack     Stack  `json:"stack"`
}

type HTTPCall struct {
	Type       string          `json:"type"`
	Goid       uint32          `json:"goid"`
	ReqID      string          `json:"req_id"`
	StartTime  int64           `json:"start_time"`
	EndTime    *int64          `json:"end_time,omitempty"`
	Method     string          `json:"method"`
	Host       string          `json:"host"`
	Path       string          `json:"path"`
	URL        string          `json:"url"`
	StatusCode int             `json:"status_code"`
	Err        []byte          `json:"err"`
	Metrics    HTTPCallMetrics `json:"metrics"`
}

type HTTPCallMetrics struct {
	// Times are all 0 if not set
	GotConn           *int64 `json:"got_conn,omitempty"`
	ConnReused        bool   `json:"conn_reused,omitempty"`
	DNSDone           *int64 `json:"dns_done,omitempty"`
	TLSHandshakeDone  *int64 `json:"tls_handshake_done,omitempty"`
	WroteHeaders      *int64 `json:"wrote_headers,omitempty"`
	WroteRequest      *int64 `json:"wrote_request,omitempty"`
	FirstResponseByte *int64 `json:"first_response,omitempty"`
	BodyClosed        *int64 `json:"body_closed,omitempty"`
}

type LogMessage struct {
	Type    string     `json:"type"` // "LogMessage"
	Goid    uint32     `json:"goid"`
	Time    int64      `json:"time"`
	Level   string     `json:"level"` // "DEBUG", "INFO", or "ERROR"
	Message string     `json:"msg"`
	Fields  []LogField `json:"fields"`
	Stack   Stack      `json:"stack"`
}

type Stack struct {
	Frames []StackFrame `json:"frames"`
}

type StackFrame struct {
	FullFile  string `json:"full_file"`
	ShortFile string `json:"short_file"`
	Func      string `json:"func"`
	Line      int    `json:"line"`
}

type LogField struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	Stack *Stack      `json:"stack"`
}

type Event interface {
	traceEvent()
}

func (Goroutine) traceEvent()     {}
func (DBTransaction) traceEvent() {}
func (DBQuery) traceEvent()       {}
func (RPCCall) traceEvent()       {}
func (HTTPCall) traceEvent()      {}
func (LogMessage) traceEvent()    {}

func TransformTrace(ct *trace.TraceMeta) (*Trace, error) {
	traceID := traceUUID(ct.ID)
	tr := &Trace{
		ID:   traceID,
		Date: ct.Date,
	}

	tp := &traceParser{meta: ct}
	reqMap := make(map[string]*Request)
	for _, req := range ct.Reqs {
		if tp.startTime == 0 {
			tp.startTime = int64(req.StartTime / 1000)
		}
		r, err := tp.parseReq(req)
		if err != nil {
			return nil, fmt.Errorf("parsing request: %v", err)
		}
		reqMap[r.ID] = r

		switch {
		case req.Type == tracepb.Request_AUTH:
			if tr.Auth != nil {
				return nil, fmt.Errorf("got multiple auth calls in trace")
			}
			tr.Auth = r
		case r.ParentID == nil:
			if tr.Root != nil {
				return nil, fmt.Errorf("got multiple root requests (%v and %v)", tr.Root.ID, r.ID)
			}
			tr.Root = r
		default:
			parent, ok := reqMap[*r.ParentID]
			if !ok {
				return nil, fmt.Errorf("could not find parent request: %v", *r.ParentID)
			}
			parent.Children = append(parent.Children, r)
		}
	}

	if tr.Root == nil && tr.Auth == nil {
		return nil, fmt.Errorf("could not find a root request")
	}

	// Copy certain properties to the trace from the root request
	for _, req := range ct.Reqs {
		if t := tp.time(req.StartTime); t < tr.StartTime {
			tr.StartTime = t
		}
		if t := tp.time(req.EndTime); t > tr.EndTime {
			tr.EndTime = t
		}
	}

	locs := make(map[int32]json.RawMessage)
	m := &jsonpb.Marshaler{OrigName: true, EmitDefaults: true}
	for _, pkg := range ct.Meta.Pkgs {
		for _, e := range pkg.TraceNodes {
			s, err := m.MarshalToString(e)
			if err != nil {
				return nil, err
			}
			locs[e.Id] = json.RawMessage(s)
		}
	}
	tr.Locations = locs
	return tr, nil
}

type traceParser struct {
	startTime int64
	txCounter uint32
	meta      *trace.TraceMeta
}

func (tp *traceParser) parseReq(req *tracepb.Request) (*Request, error) {
	// Prevent marshalling as null
	inputs, outputs := req.Inputs, req.Outputs
	if inputs == nil {
		inputs = [][]byte{}
	}
	if outputs == nil {
		outputs = [][]byte{}
	}

	r := &Request{
		Type:      req.Type.String(),
		ID:        strconv.FormatUint(req.SpanId, 10),
		ParentID:  nullIntStr(req.ParentSpanId),
		Goid:      req.Goid,
		StartTime: tp.time(req.StartTime),
		EndTime:   tp.maybeTime(req.EndTime),
		CallLoc:   nullInt32(req.CallLoc),
		DefLoc:    req.DefLoc,
		Inputs:    inputs,
		Outputs:   outputs,
		Err:       nullBytes(req.Err),
		Events:    []Event{},    // prevent marshalling as null
		Children:  []*Request{}, // prevent marshalling as null
		ErrStack:  tp.maybeStack(req.ErrStack),
	}

	for _, ev := range req.Events {
		switch e := ev.Data.(type) {
		case *tracepb.Event_Tx:
			tx, err := tp.parseTx(e.Tx)
			if err != nil {
				return nil, fmt.Errorf("parsing db transaction event: %v", err)
			}
			r.Events = append(r.Events, tx)

		case *tracepb.Event_Query:
			r.Events = append(r.Events, tp.parseQuery(e.Query, 0))

		case *tracepb.Event_Rpc:
			r.Events = append(r.Events, tp.parseCall(e.Rpc))

		case *tracepb.Event_Http:
			r.Events = append(r.Events, tp.parseHTTP(e.Http))

		case *tracepb.Event_Goroutine:
			r.Events = append(r.Events, tp.parseGoroutine(e.Goroutine))

		case *tracepb.Event_Log:
			r.Events = append(r.Events, tp.parseLog(e.Log))
		}
	}

	return r, nil
}

func (tp *traceParser) parseGoroutine(g *tracepb.Goroutine) *Goroutine {
	return &Goroutine{
		Type:      "Goroutine",
		Goid:      g.Goid,
		CallLoc:   g.CallLoc,
		StartTime: tp.time(g.StartTime),
		EndTime:   tp.maybeTime(g.EndTime),
	}
}

func (tp *traceParser) parseLog(l *tracepb.LogMessage) *LogMessage {
	msg := &LogMessage{
		Type:    "LogMessage",
		Goid:    l.Goid,
		Time:    tp.time(l.Time),
		Level:   l.Level.String(),
		Message: l.Msg,
		Fields:  []LogField{},
		Stack:   tp.stack(l.Stack),
	}
	for _, f := range l.Fields {
		field := LogField{Key: f.Key}
		switch v := f.Value.(type) {
		case *tracepb.LogField_Error:
			field.Value = v.Error.Error
			if s := v.Error.Stack; s != nil {
				st := tp.stack(s)
				field.Stack = &st
			}
		case *tracepb.LogField_Str:
			field.Value = v.Str
		case *tracepb.LogField_Bool:
			field.Value = v.Bool
		case *tracepb.LogField_Time:
			field.Value = v.Time.AsTime()
		case *tracepb.LogField_Dur:
			field.Value = time.Duration(v.Dur).String()
		case *tracepb.LogField_Uuid:
			field.Value = uuid.FromBytesOrNil(v.Uuid).String()
		case *tracepb.LogField_Json:
			field.Value = json.RawMessage(v.Json)
		case *tracepb.LogField_Int:
			field.Value = v.Int
		case *tracepb.LogField_Uint:
			field.Value = v.Uint
		case *tracepb.LogField_Float32:
			field.Value = v.Float32
		case *tracepb.LogField_Float64:
			field.Value = v.Float64
		}
		msg.Fields = append(msg.Fields, field)
	}
	return msg
}

func (tp *traceParser) parseTx(tx *tracepb.DBTransaction) (*DBTransaction, error) {
	tp.txCounter++
	txid := tp.txCounter
	t := &DBTransaction{
		Type:       "DBTransaction",
		Goid:       tx.Goid,
		Txid:       txid,
		StartLoc:   tx.StartLoc,
		EndLoc:     tx.EndLoc,
		StartTime:  tp.time(tx.StartTime),
		EndTime:    tp.maybeTime(tx.EndTime),
		Err:        nullBytes(tx.Err),
		Queries:    []*DBQuery{}, // prevent marshalling as null
		BeginStack: tp.stack(tx.BeginStack),
		EndStack:   tp.maybeStack(tx.EndStack),
	}
	switch tx.Completion {
	case tracepb.DBTransaction_COMMIT:
		t.CompletionType = "COMMIT"
	case tracepb.DBTransaction_ROLLBACK:
		t.CompletionType = "ROLLBACK"
	default:
		return nil, fmt.Errorf("unknown completion type %v", tx.Completion)
	}
	for _, q := range tx.Queries {
		t.Queries = append(t.Queries, tp.parseQuery(q, txid))
	}
	return t, nil
}

func (tp *traceParser) parseQuery(q *tracepb.DBQuery, txid uint32) *DBQuery {
	query := dedent.Bytes(q.Query)
	lexer := lexers.Get("postgres")
	iterator, err := lexer.Tokenise(nil, string(query))
	var htmlQuery []byte
	if err == nil {
		var buf bytes.Buffer
		formatter := html.New()
		style := styles.VisualStudio
		if err = formatter.Format(&buf, style, iterator); err == nil {
			htmlQuery = buf.Bytes()
		}
	}

	return &DBQuery{
		Type:      "DBQuery",
		Goid:      q.Goid,
		Txid:      nullUint32(txid),
		CallLoc:   q.CallLoc,
		StartTime: tp.time(q.StartTime),
		EndTime:   tp.maybeTime(q.EndTime),
		Query:     dedent.Bytes(q.Query),
		HTMLQuery: htmlQuery,
		Err:       nullBytes(q.Err),
		Stack:     tp.stack(q.Stack),
	}
}

func (tp *traceParser) parseCall(c *tracepb.RPCCall) *RPCCall {
	return &RPCCall{
		Type:      "RPCCall",
		Goid:      c.Goid,
		ReqID:     strconv.FormatUint(c.SpanId, 10),
		CallLoc:   c.CallLoc,
		DefLoc:    c.DefLoc,
		StartTime: tp.time(c.StartTime),
		EndTime:   tp.maybeTime(c.EndTime),
		Err:       nullBytes(c.Err),
		Stack:     tp.stack(c.Stack),
	}
}

func (tp *traceParser) parseHTTP(c *tracepb.HTTPCall) *HTTPCall {
	host := ""
	path := ""
	if u, err := url.Parse(c.Url); err == nil {
		host = u.Host
		path = u.Path
	}

	call := &HTTPCall{
		Type:       "HTTPCall",
		Goid:       c.Goid,
		ReqID:      strconv.FormatUint(c.SpanId, 10),
		Method:     c.Method,
		Host:       host,
		Path:       path,
		URL:        c.Url,
		StatusCode: int(c.StatusCode),
		StartTime:  tp.time(c.StartTime),
		EndTime:    tp.maybeTime(c.EndTime),
		Err:        nullBytes(c.Err),
		Metrics: HTTPCallMetrics{
			BodyClosed: tp.maybeTime(c.BodyClosedTime),
		},
	}
	m := &call.Metrics
	for _, ev := range c.Events {
		switch ev.Code {
		case tracepb.HTTPTraceEventCode_GOT_CONN:
			m.GotConn = tp.maybeTime(ev.Time)
			m.ConnReused = ev.GetGotConn().Reused
		case tracepb.HTTPTraceEventCode_DNS_DONE:
			m.DNSDone = tp.maybeTime(ev.Time)
		case tracepb.HTTPTraceEventCode_TLS_HANDSHAKE_DONE:
			m.TLSHandshakeDone = tp.maybeTime(ev.Time)
		case tracepb.HTTPTraceEventCode_WROTE_HEADERS:
			m.WroteHeaders = tp.maybeTime(ev.Time)
		case tracepb.HTTPTraceEventCode_WROTE_REQUEST:
			m.WroteRequest = tp.maybeTime(ev.Time)
		case tracepb.HTTPTraceEventCode_GOT_FIRST_RESPONSE_BYTE:
			m.FirstResponseByte = tp.maybeTime(ev.Time)
		}
	}
	return call
}

func (tp *traceParser) time(ns uint64) int64 {
	if ns == 0 {
		return -1
	}
	t := int64(ns/1000) - tp.startTime
	return t
}

func (tp *traceParser) maybeTime(ns uint64) *int64 {
	if ns == 0 {
		return nil
	}
	t := int64(ns/1000) - tp.startTime
	return &t
}

func nullIntStr(n uint64) *string {
	if n == 0 {
		return nil
	}
	s := strconv.FormatUint(n, 10)
	return &s
}

func nullInt32(n int32) *int32 {
	if n == 0 {
		return nil
	}
	return &n
}

func nullUint32(n uint32) *uint32 {
	if n == 0 {
		return nil
	}
	return &n
}

func nullBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}

func traceUUID(traceID trace.ID) uuid.UUID {
	return uuid.UUID(traceID)
}

func (tp *traceParser) stack(s *tracepb.StackTrace) Stack {
	if s == nil {
		return Stack{Frames: []StackFrame{}}
	}
	st := Stack{
		Frames: make([]StackFrame, 0, len(s.Frames)),
	}
	for _, f := range s.Frames {
		if f.Func == "runtime.goexit" {
			continue
		}
		st.Frames = append(st.Frames, StackFrame{
			FullFile:  f.Filename,
			ShortFile: shortenFilename(tp.meta.AppRoot, f.Filename, f.Func),
			Func:      shortenFunc(f.Func),
			Line:      int(f.Line),
		})
	}
	return st
}

func (tp *traceParser) maybeStack(s *tracepb.StackTrace) *Stack {
	if st := tp.stack(s); len(st.Frames) > 0 {
		return &st
	}
	return nil
}

func shortenFilename(appRoot, file, fn string) string {
	if rel, err := filepath.Rel(appRoot, file); err == nil && !strings.HasPrefix(rel, "..") {
		return "./" + filepath.ToSlash(rel)
	} else if fn != "" {
		// Use the package import path
		pkgPath, remainder := "", fn
		if idx := strings.LastIndexByte(fn, '/'); idx >= 0 {
			pkgPath = fn[:idx+1]   // "import.path/foo/"
			remainder = fn[idx+1:] // "bar.(*Foo).Baz"
		}
		if idx := strings.IndexByte(remainder, '.'); idx >= 0 {
			pkgName := remainder[:idx] // "bar"
			pkgPath += pkgName
		}
		return pkgPath // "import.path/foo/bar"
	}

	// Standard library?
	if idx := strings.Index(file, "/src/pkg/"); idx >= 0 {
		return file[idx+len("/src/pkg/"):]
	}
	// Encore runtime?
	rtPath := env.EncoreRuntimePath()
	if idx := strings.Index(file, rtPath); idx >= 0 {
		return file[idx+len(rtPath):]
	}

	prefixes := [...]string{
		"github.com/",
		"encore.dev/",
		"bitbucket.org/",
		"gopkg.in/",
	}
	for _, p := range prefixes {
		if idx := strings.Index(file, p); idx >= 0 {
			return file[idx:]
		}
	}
	return file
}

func shortenFunc(fn string) string {
	// Cut import path
	if idx := strings.LastIndexByte(fn, '/'); idx >= 0 {
		fn = fn[idx+1:]
	}
	// Cut package name
	if idx := strings.IndexByte(fn, '.'); idx >= 0 {
		fn = fn[idx+1:]
	}
	return fn
}
