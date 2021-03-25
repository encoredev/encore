package dash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

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
	EndTime   int64      `json:"end_time"`
	CallLoc   *int32     `json:"call_loc"`
	DefLoc    int32      `json:"def_loc"`
	Inputs    [][]byte   `json:"inputs"`
	Outputs   [][]byte   `json:"outputs"`
	Err       []byte     `json:"err"`
	Events    []Event    `json:"events"`
	Children  []*Request `json:"children"`
}

type Goroutine struct {
	Type      string `json:"type"`
	Goid      uint32 `json:"goid"`
	CallLoc   int32  `json:"call_loc"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

type DBTransaction struct {
	Type           string     `json:"type"`
	Goid           uint32     `json:"goid"`
	Txid           uint32     `json:"txid"`
	StartLoc       int32      `json:"start_loc"`
	EndLoc         int32      `json:"end_loc"`
	StartTime      int64      `json:"start_time"`
	EndTime        int64      `json:"end_time"`
	Err            []byte     `json:"err"`
	CompletionType string     `json:"completion_type"`
	Queries        []*DBQuery `json:"queries"`
}

type DBQuery struct {
	Type      string  `json:"type"`
	Goid      uint32  `json:"goid"`
	Txid      *uint32 `json:"txid"`
	CallLoc   int32   `json:"call_loc"`
	StartTime int64   `json:"start_time"`
	EndTime   int64   `json:"end_time"`
	Query     []byte  `json:"query"`
	HTMLQuery []byte  `json:"html_query"`
	Err       []byte  `json:"err"`
}

type RPCCall struct {
	Type      string `json:"type"`
	Goid      uint32 `json:"goid"`
	ReqID     string `json:"req_id"`
	CallLoc   int32  `json:"call_loc"`
	DefLoc    int32  `json:"def_loc"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Err       []byte `json:"err"`
}

type HTTPCall struct {
	Type       string `json:"type"`
	Goid       uint32 `json:"goid"`
	ReqID      string `json:"req_id"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	Method     string `json:"method"`
	Host       string `json:"host"`
	Path       string `json:"path"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Err        []byte `json:"err"`

	// May be -1
	BodyClosedTime int64 `json:"body_closed_time"`
}

type Event interface {
	traceEvent()
}

func (Goroutine) traceEvent()     {}
func (DBTransaction) traceEvent() {}
func (DBQuery) traceEvent()       {}
func (RPCCall) traceEvent()       {}
func (HTTPCall) traceEvent()      {}

func TransformTrace(ct *trace.TraceMeta) (*Trace, error) {
	traceID := traceUUID(ct.ID)
	tr := &Trace{
		ID:   traceID,
		Date: ct.Date,
	}

	tp := &traceParser{}
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
		EndTime:   tp.time(req.EndTime),
		CallLoc:   nullInt32(req.CallLoc),
		DefLoc:    req.DefLoc,
		Inputs:    inputs,
		Outputs:   outputs,
		Err:       nullBytes(req.Err),
		Events:    []Event{},    // prevent marshalling as null
		Children:  []*Request{}, // prevent marshalling as null
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
		EndTime:   tp.time(g.EndTime),
	}
}

func (tp *traceParser) parseTx(tx *tracepb.DBTransaction) (*DBTransaction, error) {
	tp.txCounter++
	txid := tp.txCounter
	t := &DBTransaction{
		Type:      "DBTransaction",
		Goid:      tx.Goid,
		Txid:      txid,
		StartLoc:  tx.StartLoc,
		EndLoc:    tx.EndLoc,
		StartTime: tp.time(tx.StartTime),
		EndTime:   tp.time(tx.EndTime),
		Err:       nullBytes(tx.Err),
		Queries:   []*DBQuery{}, // prevent marshalling as null
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
		EndTime:   tp.time(q.EndTime),
		Query:     dedent.Bytes(q.Query),
		HTMLQuery: htmlQuery,
		Err:       nullBytes(q.Err),
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
		EndTime:   tp.time(c.EndTime),
		Err:       nullBytes(c.Err),
	}
}

func (tp *traceParser) parseHTTP(c *tracepb.HTTPCall) *HTTPCall {
	return &HTTPCall{
		Type:           "HTTPCall",
		Goid:           c.Goid,
		ReqID:          strconv.FormatUint(c.SpanId, 10),
		Method:         c.Method,
		Host:           c.Host,
		Path:           c.Path,
		URL:            c.Url,
		StatusCode:     int(c.StatusCode),
		StartTime:      tp.time(c.StartTime),
		EndTime:        tp.time(c.EndTime),
		BodyClosedTime: tp.time(c.BodyClosedTime),
		Err:            nullBytes(c.Err),
	}
}

func (tp *traceParser) time(ns uint64) int64 {
	if ns == 0 {
		return -1
	}
	return int64(ns/1000) - tp.startTime
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

func parseTime(ns uint64) time.Time {
	return time.Unix(0, int64(ns))
}

func traceUUID(traceID trace.ID) uuid.UUID {
	return uuid.UUID(traceID)
}
