package trace

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/exported/trace"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/internal/sym"
	"encr.dev/pkg/eerror"
	tracepb "encr.dev/proto/encore/engine/trace"
	metapb "encr.dev/proto/encore/parser/meta/v1"
)

type ID [16]byte

type TraceMeta struct {
	ID    ID
	Reqs  []*tracepb.Request
	App   *apps.Instance
	EnvID string
	Date  time.Time
	Meta  *metapb.Data
}

// A Store stores traces received from running applications.
type Store struct {
	trmu             sync.Mutex
	traces           map[string][]*TraceMeta
	requestIDMapping map[string]*tracepb.Request // Trace ID -> Request

	lnmu sync.Mutex
	ln   map[chan<- *TraceMeta]struct{}
}

func NewStore() *Store {
	return &Store{
		traces:           make(map[string][]*TraceMeta),
		requestIDMapping: make(map[string]*tracepb.Request),
		ln:               make(map[chan<- *TraceMeta]struct{}),
	}
}

func (st *Store) Listen(ch chan<- *TraceMeta) {
	st.lnmu.Lock()
	st.ln[ch] = struct{}{}
	st.lnmu.Unlock()
}

func (st *Store) Store(ctx context.Context, tr *TraceMeta) error {
	appID := tr.App.PlatformOrLocalID()
	st.trmu.Lock()
	st.traces[appID] = append(st.traces[appID], tr)

	const limit = 100
	// Remove earlier traces if we exceed the limit.
	if n := len(st.traces[appID]); n > limit {
		st.traces[appID] = st.traces[appID][n-limit:]
	}

	for _, req := range tr.Reqs {
		st.requestIDMapping[req.TraceId.String()] = req
	}

	st.trmu.Unlock()

	st.lnmu.Lock()
	defer st.lnmu.Unlock()
	for ch := range st.ln {
		// Don't block trying to send
		select {
		case ch <- tr:
		default:
		}
	}
	return nil
}

func (st *Store) GetRootTrace(traceID *tracepb.TraceID) (rtn *tracepb.Request) {
	st.trmu.Lock()
	defer st.trmu.Unlock()

	next := st.requestIDMapping[traceID.String()]
	for next != nil {
		rtn = next
		next = st.requestIDMapping[rtn.ParentTraceId.String()]
	}

	return rtn
}

func (st *Store) List(appID string) []*TraceMeta {
	st.trmu.Lock()
	tr := st.traces[appID]
	st.trmu.Unlock()
	return tr
}

func Parse(log *zerolog.Logger, traceID ID, data []byte, version trace.Version, symTable SymTabler) ([]*tracepb.Request, error) {
	id := &tracepb.TraceID{
		Low:  bin.Uint64(traceID[:8]),
		High: bin.Uint64(traceID[8:]),
	}
	tp := &traceParser{
		log:          log,
		version:      version,
		traceReader:  traceReader{buf: data},
		symTable:     symTable,
		traceID:      id,
		reqMap:       make(map[uint64]*tracepb.Request),
		txMap:        make(map[uint64]*tracepb.DBTransaction),
		queryMap:     make(map[uint64]*tracepb.DBQuery),
		callMap:      make(map[uint64]interface{}),
		goMap:        make(map[goKey]*tracepb.Goroutine),
		httpMap:      make(map[uint64]*tracepb.HTTPCall),
		publishMap:   make(map[uint64]*tracepb.PubsubMsgPublished),
		serviceInits: make(map[uint64]*tracepb.ServiceInit),
		cacheMap:     make(map[uint64]*tracepb.CacheOp),
	}
	if err := tp.Parse(); err != nil {
		return nil, err
	}
	return tp.reqs, nil
}

type goKey struct {
	spanID uint64
	goid   uint32
}

type SymTabler interface {
	SymTable(ctx context.Context) (*sym.Table, error)
}

type traceParser struct {
	traceReader
	log          *zerolog.Logger
	version      trace.Version
	symTable     SymTabler
	traceID      *tracepb.TraceID
	reqs         []*tracepb.Request
	reqMap       map[uint64]*tracepb.Request
	txMap        map[uint64]*tracepb.DBTransaction
	queryMap     map[uint64]*tracepb.DBQuery
	callMap      map[uint64]interface{} // *RPCCall or *AuthCall
	httpMap      map[uint64]*tracepb.HTTPCall
	goMap        map[goKey]*tracepb.Goroutine
	publishMap   map[uint64]*tracepb.PubsubMsgPublished
	serviceInits map[uint64]*tracepb.ServiceInit
	cacheMap     map[uint64]*tracepb.CacheOp
}

func (tp *traceParser) Parse() error {
	for i := 0; !tp.Done(); i++ {
		ev := trace.EventType(tp.Byte())
		ts := tp.Uint64()
		size := int(tp.Uint32())
		startOff := tp.Offset()

		var err error
		if tp.version >= 3 {
			err = tp.parseEventV3(ev, ts, size)
		} else {
			err = tp.parseEventV1(byte(ev), ts, size)
		}

		if errors.Is(err, errUnknownEvent) {
			tp.log.Info().Msgf("trace: event #%d: unknown event type %s, skipping", i, ev.String())
			tp.Skip(size)
			err = nil
		} else if err != nil {
			return eerror.WithMeta(err, map[string]any{"event#": i, "event": ev.String()})
		}

		if tp.Overflow() {
			return eerror.New("trace_parser", "invalid trace format: reader overflow parsing event", map[string]any{"event#": i, "event": ev})
		} else if off, want := tp.Offset(), startOff+size; off < want {
			tp.log.Warn().Msgf("trace: event #%d: parsing event=%s ended before end of frame, skipping ahead %d bytes", i, ev, want-off)
			tp.Skip(want - off)
		} else if off > want {
			return eerror.New("trace_parser", "event exceed frame size", map[string]any{"event#": i, "event": ev.String(), "excess": off - want})
		}
	}

	return nil
}

var errUnknownEvent = errors.New("unknown event")

func (tp *traceParser) parseEventV3(ev trace.EventType, ts uint64, size int) error {
	switch ev {
	case trace.RequestStart:
		return tp.requestStart(ts)
	case trace.RequestEnd:
		return tp.requestEnd(ts)
	case trace.GoStart:
		return tp.goroutineStart(ts)
	case trace.GoEnd:
		return tp.goroutineEnd(ts)
	case trace.GoClear:
		return tp.goroutineClear(ts)
	case trace.TxStart:
		return tp.transactionStart(ts)
	case trace.TxEnd:
		return tp.transactionEnd(ts)
	case trace.QueryStart:
		return tp.queryStart(ts)
	case trace.QueryEnd:
		return tp.queryEnd(ts)
	case trace.CallStart:
		return tp.callStart(ts, size)
	case trace.CallEnd:
		return tp.callEnd(ts)
	case trace.AuthStart, trace.AuthEnd:
		// Skip these events for now
		tp.Skip(size)
		return nil

	case trace.HTTPCallStart:
		return tp.httpStart(ts)
	case trace.HTTPCallEnd:
		return tp.httpEnd(ts)
	case trace.HTTPCallBodyClosed:
		return tp.httpBodyClosed(ts)
	case trace.LogMessage:
		return tp.logMessage(ts)
	case trace.PublishStart:
		return tp.publishStart(ts)
	case trace.PublishEnd:
		return tp.publishEnd(ts)
	case trace.ServiceInitStart:
		return tp.serviceInitStart(ts)
	case trace.ServiceInitEnd:
		return tp.serviceInitEnd(ts)
	case trace.CacheOpStart:
		return tp.cacheOpStart(ts)
	case trace.CacheOpEnd:
		return tp.cacheOpEnd(ts)
	case trace.BodyStream:
		return tp.bodyStream(ts)
	default:
		return errUnknownEvent
	}
}

func (tp *traceParser) parseEventV1(ev byte, ts uint64, size int) error {
	switch ev {
	case 0x01:
		return tp.requestStart(ts)
	case 0x02:
		return tp.requestEnd(ts)
	case 0x03:
		return tp.goroutineStart(ts)
	case 0x04:
		return tp.goroutineEnd(ts)
	case 0x05:
		return tp.goroutineClear(ts)
	case 0x06:
		return tp.transactionStart(ts)
	case 0x07:
		return tp.transactionEnd(ts)
	case 0x08:
		return tp.queryStart(ts)
	case 0x09:
		return tp.queryEnd(ts)
	case 0x10:
		return tp.callStart(ts, size)
	case 0x11:
		return tp.callEnd(ts)
	case 0x12, 0x13:
		// Skip these events for now
		tp.Skip(size)
		return nil

	default:
		return errUnknownEvent
	}
}

func (tp *traceParser) requestStart(ts uint64) error {
	typ, err := tp.parseRequestType()
	if err != nil {
		return err
	}

	// Determine the absolute start time.
	var absStart time.Time
	if tp.version >= 6 {
		absStart = tp.Time()
	} else {
		// We don't have enough information to determine the exact start time,
		// but approximate it from the monotonic clock reading
		absStart = time.Unix(0, int64(ts))
	}

	// Set the trace ID
	traceID := tp.traceID
	if tp.version >= 11 {
		parsedTraceID := tp.parseTraceID()
		if parsedTraceID.Low != 0 || parsedTraceID.High != 0 {
			traceID = parsedTraceID
		}
	}
	var parentTraceID *tracepb.TraceID
	if tp.version >= 12 {
		parentTraceID = tp.parseTraceID()
	}

	spanID := tp.Uint64()
	parentSpanID := tp.Uint64()

	var service, endpoint string
	if tp.version < 6 {
		service, endpoint = "unknown", "Unknown"
	} else if tp.version < 9 {
		service = tp.String()
		endpoint = tp.String()
	}

	goid := uint32(tp.UVarint())
	if tp.version < 9 {
		_ = tp.UVarint() // skip CallLoc: no longer used
	}
	defLoc := int32(tp.UVarint())

	req := &tracepb.Request{
		TraceId:       traceID,
		ParentTraceId: parentTraceID,
		SpanId:        spanID,
		ParentSpanId:  parentSpanID,
		StartTime:     ts,
		ServiceName:   service,
		EndpointName:  endpoint,
		AbsStartTime:  uint64(absStart.UnixNano()),
		// EndTime not set yet
		DefLoc: defLoc,
		Goid:   goid,
		Type:   typ,
	}

	if tp.version < 9 {
		req.Uid = tp.String()

		for n, i := tp.UVarint(), uint64(0); i < n; i++ {
			size := tp.UVarint()
			if size > (10 << 20) {
				return eerror.New("trace_parser", "input too large", map[string]any{"size": size})
			}
			input := make([]byte, size)
			tp.Bytes(input)
			req.Inputs = append(req.Inputs, input)
		}
	}

	switch typ {
	case tracepb.Request_RPC:
		if tp.version >= 9 {
			isRaw := tp.Bool()
			req.ServiceName = tp.String()
			req.EndpointName = tp.String()
			req.HttpMethod = tp.String()
			req.Path = tp.String()

			numParams := tp.UVarint()
			req.PathParams = make([]string, numParams)
			for i := uint64(0); i < numParams; i++ {
				req.PathParams[i] = tp.String()
			}

			req.Uid = tp.String()

			if tp.version >= 11 {
				req.ExternalRequestId = tp.String()

				if tp.version >= 12 {
					req.ExternalCorrelationId = tp.String()
				}
			}

			if isRaw {
				req.RawRequestHeaders = tp.parseHTTPHeaders()
			} else {
				req.RequestPayload = tp.ByteString()
			}
		}

	case tracepb.Request_AUTH:
		if tp.version >= 9 {
			req.ServiceName = tp.String()
			req.EndpointName = tp.String()
			req.RequestPayload = tp.ByteString()
		}

	case tracepb.Request_PUBSUB_MSG:
		if tp.version >= 9 {
			req.ServiceName = tp.String()
		}

		req.TopicName = tp.String()
		req.SubscriptionName = tp.String()
		req.MessageId = tp.String()
		req.Attempt = tp.Uint32()
		req.PublishTime = uint64(tp.Time().UnixMilli())

		if tp.version >= 10 {
			req.RequestPayload = tp.ByteString()
		}
	}

	tp.reqs = append(tp.reqs, req)
	tp.reqMap[req.SpanId] = req
	return nil
}

func (tp *traceParser) bodyStream(ts uint64) error {
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}
	flags := tp.Byte()
	data := tp.ByteString()

	isResponse := (flags & 1) == 1
	overflowed := (flags & 2) == 2

	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_BodyStream{
			BodyStream: &tracepb.BodyStream{
				IsResponse: isResponse,
				Overflowed: overflowed,
				Data:       data,
			},
		},
	})

	return nil
}

func (tp *traceParser) requestEnd(ts uint64) error {
	var typ tracepb.Request_Type
	if tp.version >= 9 {
		var err error
		typ, err = tp.parseRequestType()
		if err != nil {
			return err
		}
	}

	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}
	if tp.version < 9 {
		// Not captured by the protocol for old versions,
		// so grab it from the request.
		typ = req.Type
	}

	// dur := ts - rd.startTs
	req.EndTime = ts

	if tp.version >= 9 {
		errMsg := tp.ByteString()
		if len(errMsg) > 0 {
			req.Err = errMsg

			req.ErrStack = tp.stack(filterNone)
			if tp.version >= 13 {
				req.PanicStack = tp.formattedStack()
			}
		}

		switch typ {
		case tracepb.Request_RPC:
			if isRaw := tp.Bool(); isRaw {
				req.RawResponseHeaders = tp.parseHTTPHeaders()
			} else {
				req.ResponsePayload = tp.ByteString()
			}
		case tracepb.Request_AUTH:
			req.Uid = tp.String()
			req.ResponsePayload = tp.ByteString()
		case tracepb.Request_PUBSUB_MSG:
			req.ResponsePayload = tp.ByteString()
		}
	} else {
		isErr := tp.Bool()
		if isErr {
			msg := tp.ByteString()
			if len(msg) == 0 {
				msg = []byte("unknown error")
			}
			if tp.version >= 5 {
				req.ErrStack = tp.stack(filterNone)
			}
		} else {
			for n, i := tp.UVarint(), uint64(0); i < n; i++ {
				size := tp.UVarint()
				if size > (10 << 20) {
					return eerror.New("trace_parser", "input too large", map[string]any{"size": size})
				}
				output := make([]byte, size)
				tp.Bytes(output)
				req.Outputs = append(req.Outputs, output)
			}
		}
	}

	return nil
}

func (tp *traceParser) goroutineStart(ts uint64) error {
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		// This is an expected error in certain situations like goroutines
		// living past the request end that then spawn additional goroutines.
		// Treat it as a warning but don't fail the parse.
		tp.log.Warn().Uint64("span_id", spanID).Msg("unknown request span")
		return nil
	}
	goid := tp.Uint32()
	g := &tracepb.Goroutine{
		Goid:      goid,
		StartTime: ts,
	}
	k := goKey{spanID: spanID, goid: goid}
	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_Goroutine{Goroutine: g},
	})
	tp.goMap[k] = g
	return nil
}

func (tp *traceParser) goroutineEnd(ts uint64) error {
	spanID := tp.Uint64()
	goid := tp.Uint32()
	k := goKey{spanID: spanID, goid: goid}
	g, ok := tp.goMap[k]
	if !ok {
		return eerror.New("trace_parser", "unknown goroutine id", map[string]any{"goid": goid})
	}
	g.EndTime = ts
	delete(tp.goMap, k)
	return nil
}

func (tp *traceParser) goroutineClear(ts uint64) error {
	spanID := tp.Uint64()
	goid := tp.Uint32()
	k := goKey{spanID: spanID, goid: goid}
	g, ok := tp.goMap[k]
	if !ok {
		return eerror.New("trace_parser", "unknown goroutine id", map[string]any{"spanID": spanID, "goid": goid})
	}
	g.EndTime = ts
	delete(tp.goMap, k)
	return nil
}

func (tp *traceParser) transactionStart(ts uint64) error {
	txid := tp.UVarint()
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}
	goid := uint32(tp.UVarint())

	if tp.version < 4 {
		_ = tp.UVarint() // StartLoc; no longer used
	}

	tx := &tracepb.DBTransaction{
		Goid:      goid,
		StartTime: ts,
	}
	if tp.version >= 5 {
		tx.BeginStack = tp.stack(filterDB)
	}
	tp.txMap[txid] = tx
	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_Tx{Tx: tx},
	})
	return nil
}

func (tp *traceParser) transactionEnd(ts uint64) error {
	txid := tp.UVarint()
	_ = tp.Uint64() // spanID
	tx, ok := tp.txMap[txid]
	if !ok {
		return eerror.New("trace_parser", "unknown transaction id", map[string]any{"txid": txid})
	}
	_ = uint32(tp.UVarint()) // goid
	compl := tp.Byte()
	if tp.version < 4 {
		_ = int32(tp.UVarint()) // EndLoc; no longer used
	}
	errMsg := tp.ByteString()

	var stack *tracepb.StackTrace
	if tp.version >= 5 {
		stack = tp.stack(filterDB)
	}

	// It's possible to get multiple transaction end events.
	// Ignore them for now; we will expose this information later.
	if tx.EndTime == 0 {
		tx.EndTime = ts
		tx.Err = errMsg
		tx.EndStack = stack
		switch compl {
		case 0:
			tx.Completion = tracepb.DBTransaction_ROLLBACK
		case 1:
			tx.Completion = tracepb.DBTransaction_COMMIT
		default:
			return eerror.New("trace_parser", "unknown completion type", map[string]any{"compl": compl})
		}
	}
	return nil
}

func (tp *traceParser) queryStart(ts uint64) error {
	qid := tp.UVarint()
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}
	txid := tp.UVarint()
	goid := uint32(tp.UVarint())

	if tp.version < 4 {
		_ = tp.UVarint() // CallLoc; no longer used
	}
	q := &tracepb.DBQuery{
		Goid:      goid,
		StartTime: ts,
		Query:     tp.ByteString(),
	}
	if tp.version >= 5 {
		q.Stack = tp.stack(filterDB)
	}
	tp.queryMap[qid] = q

	if txid != 0 {
		tx, ok := tp.txMap[txid]
		if !ok {
			return eerror.New("trace_parser", "unknown transaction id", map[string]any{"txid": txid})
		}
		tx.Queries = append(tx.Queries, q)
	} else {
		req.Events = append(req.Events, &tracepb.Event{
			Data: &tracepb.Event_Query{Query: q},
		})
	}

	return nil
}

func (tp *traceParser) queryEnd(ts uint64) error {
	qid := tp.UVarint()
	q, ok := tp.queryMap[qid]
	if !ok {
		return eerror.New("trace_parser", "unknown query id", map[string]any{"qid": qid})
	}
	q.EndTime = ts
	q.Err = tp.ByteString()
	return nil
}

func (tp *traceParser) callStart(ts uint64, size int) error {
	callID := tp.UVarint()
	spanID := tp.Uint64()
	// TODO(eandre) We currently (Dec 2, 2020) have an old format
	// that leaves out the child span id. Detect this based on the size
	// and provide a workaround that doesn't crash.
	var childSpanID uint64
	if size == 12 {
		childSpanID = spanID
	} else {
		childSpanID = tp.Uint64()
	}
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}

	goid := uint32(tp.UVarint())
	_ = tp.UVarint() // CallLoc: no longer used
	defLoc := int32(tp.UVarint())

	c := &tracepb.RPCCall{
		SpanId:    childSpanID,
		Goid:      goid,
		DefLoc:    defLoc,
		StartTime: ts,
	}
	if tp.version >= 5 {
		c.Stack = tp.stack(filterNone)
	}
	tp.callMap[callID] = c
	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_Rpc{Rpc: c},
	})
	return nil
}

func (tp *traceParser) callEnd(ts uint64) error {
	callID := tp.UVarint()
	errMsg := tp.ByteString()
	c, ok := tp.callMap[callID].(*tracepb.RPCCall)
	if !ok {
		return eerror.New("trace_parser", "unknown call ", map[string]any{"callID": callID})
	}
	c.EndTime = ts
	c.Err = errMsg
	delete(tp.callMap, callID)
	return nil
}

func (tp *traceParser) httpStart(ts uint64) error {
	callID := tp.UVarint()
	spanID := tp.Uint64()
	childSpanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}
	c := &tracepb.HTTPCall{
		SpanId:    childSpanID,
		Goid:      uint32(tp.UVarint()),
		Method:    tp.String(),
		Url:       tp.String(),
		StartTime: ts,
	}
	tp.httpMap[callID] = c
	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_Http{Http: c},
	})
	return nil
}

func (tp *traceParser) httpEnd(ts uint64) error {
	callID := tp.UVarint()
	errMsg := tp.ByteString()
	status := tp.UVarint()
	c, ok := tp.httpMap[callID]
	if !ok {
		return eerror.New("trace_parser", "unknown call ", map[string]any{"callID": callID})
	}
	c.EndTime = ts
	c.Err = errMsg
	c.StatusCode = uint32(status)

	numEvents := tp.UVarint()
	c.Events = make([]*tracepb.HTTPTraceEvent, 0, numEvents)
	for i := 0; i < int(numEvents); i++ {
		ev, err := tp.httpEvent()
		if err != nil {
			return err
		}
		c.Events = append(c.Events, ev)
	}

	return nil
}

func (tp *traceParser) httpBodyClosed(ts uint64) error {
	callID := tp.UVarint()
	_ = tp.ByteString() // close error
	c, ok := tp.httpMap[callID]
	if !ok {
		return eerror.New("trace_parser", "unknown call ", map[string]any{"callID": callID})
	}
	c.BodyClosedTime = ts
	delete(tp.httpMap, callID)
	return nil
}

func (tp *traceParser) httpEvent() (*tracepb.HTTPTraceEvent, error) {
	code := tracepb.HTTPTraceEventCode(tp.Byte())
	ts := tp.Int64()
	ev := &tracepb.HTTPTraceEvent{
		Code: code,
		Time: uint64(ts),
	}

	switch code {
	case tracepb.HTTPTraceEventCode_GET_CONN:
		ev.Data = &tracepb.HTTPTraceEvent_GetConn{
			GetConn: &tracepb.HTTPGetConnData{
				HostPort: tp.String(),
			},
		}

	case tracepb.HTTPTraceEventCode_GOT_CONN:
		ev.Data = &tracepb.HTTPTraceEvent_GotConn{
			GotConn: &tracepb.HTTPGotConnData{
				Reused:         tp.Bool(),
				WasIdle:        tp.Bool(),
				IdleDurationNs: tp.Int64(),
			},
		}

	case tracepb.HTTPTraceEventCode_GOT_FIRST_RESPONSE_BYTE:
		// no data

	case tracepb.HTTPTraceEventCode_GOT_1XX_RESPONSE:
		ev.Data = &tracepb.HTTPTraceEvent_Got_1XxResponse{
			Got_1XxResponse: &tracepb.HTTPGot1XxResponseData{
				Code: int32(tp.Varint()),
			},
		}

	case tracepb.HTTPTraceEventCode_DNS_START:
		ev.Data = &tracepb.HTTPTraceEvent_DnsStart{
			DnsStart: &tracepb.HTTPDNSStartData{
				Host: tp.String(),
			},
		}

	case tracepb.HTTPTraceEventCode_DNS_DONE:
		data := &tracepb.HTTPDNSDoneData{
			Err: tp.ByteString(),
		}
		addrs := int(tp.UVarint())
		for j := 0; j < addrs; j++ {
			data.Addrs = append(data.Addrs, &tracepb.DNSAddr{
				Ip: tp.ByteString(),
			})
		}
		ev.Data = &tracepb.HTTPTraceEvent_DnsDone{DnsDone: data}

	case tracepb.HTTPTraceEventCode_CONNECT_START:
		ev.Data = &tracepb.HTTPTraceEvent_ConnectStart{
			ConnectStart: &tracepb.HTTPConnectStartData{
				Network: tp.String(),
				Addr:    tp.String(),
			},
		}

	case tracepb.HTTPTraceEventCode_CONNECT_DONE:
		ev.Data = &tracepb.HTTPTraceEvent_ConnectDone{
			ConnectDone: &tracepb.HTTPConnectDoneData{
				Network: tp.String(),
				Addr:    tp.String(),
				Err:     tp.ByteString(),
			},
		}

	case tracepb.HTTPTraceEventCode_TLS_HANDSHAKE_START:
		// no data

	case tracepb.HTTPTraceEventCode_TLS_HANDSHAKE_DONE:
		ev.Data = &tracepb.HTTPTraceEvent_TlsHandshakeDone{
			TlsHandshakeDone: &tracepb.HTTPTLSHandshakeDoneData{
				Err:                tp.ByteString(),
				TlsVersion:         tp.Uint32(),
				CipherSuite:        tp.Uint32(),
				ServerName:         tp.String(),
				NegotiatedProtocol: tp.String(),
			},
		}

	case tracepb.HTTPTraceEventCode_WROTE_HEADERS:
		// no data

	case tracepb.HTTPTraceEventCode_WROTE_REQUEST:
		ev.Data = &tracepb.HTTPTraceEvent_WroteRequest{
			WroteRequest: &tracepb.HTTPWroteRequestData{
				Err: tp.ByteString(),
			},
		}

	case tracepb.HTTPTraceEventCode_WAIT_100_CONTINUE:
		// no data

	default:
		return nil, eerror.New("trace_parser", "unknown http event", map[string]any{"code": code})
	}
	return ev, nil
}

func (tp *traceParser) logMessage(ts uint64) error {
	spanID := tp.Uint64()
	goid := uint32(tp.UVarint())
	level := tp.Byte()
	msg := tp.String()
	fields := int(tp.UVarint())

	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request", map[string]any{"spanID": spanID})
	} else if fields > 64 {
		return eerror.New("trace_parser", "too many fields", map[string]any{"fields": fields})
	}

	log := &tracepb.LogMessage{
		SpanId: spanID,
		Goid:   goid,
		Time:   ts,
		Msg:    msg,
	}

	// We introduced more log levels in trace version 8.
	if tp.version >= 8 {
		switch level {
		case 0:
			log.Level = tracepb.LogMessage_TRACE
		case 1:
			log.Level = tracepb.LogMessage_DEBUG
		case 2:
			log.Level = tracepb.LogMessage_INFO
		case 3:
			log.Level = tracepb.LogMessage_WARN
		case 4:
			log.Level = tracepb.LogMessage_ERROR
		default:
			return eerror.New("trace_parser", "unknown log message level", map[string]any{"level": level})
		}
	} else {
		switch level {
		case 0:
			log.Level = tracepb.LogMessage_DEBUG
		case 1:
			log.Level = tracepb.LogMessage_INFO
		case 2:
			log.Level = tracepb.LogMessage_ERROR
		default:
			return eerror.New("trace_parser", "unknown log message level", map[string]any{"level": level})
		}
	}

	for i := 0; i < fields; i++ {
		f, err := tp.logField()
		if err != nil {
			return eerror.Wrap(err, "trace_parser", "error parsing field", map[string]any{"field#": i})
		}
		log.Fields = append(log.Fields, f)
	}
	if tp.version >= 5 {
		log.Stack = tp.stack(filterNone)
	}

	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_Log{Log: log},
	})
	return nil
}

func (tp *traceParser) logField() (*tracepb.LogField, error) {
	typ := tp.Byte()
	key := tp.String()
	f := &tracepb.LogField{
		Key: key,
	}
	switch typ {
	case 1:
		if tp.version >= 7 { // We only added stack's to error log fields with version 7 (it was missing from the internal runtime before that)
			f.Value = &tracepb.LogField_ErrorWithStack{ErrorWithStack: &tracepb.ErrWithStack{
				Error: tp.String(),
				Stack: tp.stack(filterNone),
			}}
		} else {
			f.Value = &tracepb.LogField_ErrorWithoutStack{ErrorWithoutStack: tp.String()}
		}
	case 2:
		f.Value = &tracepb.LogField_Str{Str: tp.String()}
	case 3:
		f.Value = &tracepb.LogField_Bool{Bool: tp.Bool()}
	case 4:
		f.Value = &tracepb.LogField_Time{Time: timestamppb.New(tp.Time())}
	case 5:
		f.Value = &tracepb.LogField_Dur{Dur: tp.Int64()}
	case 6:
		b := make([]byte, 16)
		tp.Bytes(b)
		f.Value = &tracepb.LogField_Uuid{Uuid: b}
	case 7:
		val := tp.ByteString()
		err := tp.String()
		if err != "" {
			f.Value = &tracepb.LogField_ErrorWithoutStack{ErrorWithoutStack: err}
		} else {
			f.Value = &tracepb.LogField_Json{Json: val}
		}
	case 8:
		f.Value = &tracepb.LogField_Int{Int: tp.Varint()}
	case 9:
		f.Value = &tracepb.LogField_Uint{Uint: tp.UVarint()}
	case 10:
		f.Value = &tracepb.LogField_Float32{Float32: tp.Float32()}
	case 11:
		f.Value = &tracepb.LogField_Float64{Float64: tp.Float64()}
	default:
		return nil, eerror.New("trace_parser", "unknown field type", map[string]any{"typ": typ})
	}
	return f, nil
}

func (tp *traceParser) publishStart(ts uint64) error {
	publishID := tp.UVarint()
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}

	publish := &tracepb.PubsubMsgPublished{
		Goid:      tp.UVarint(),
		StartTime: ts,
		Topic:     tp.String(),
		Message:   tp.ByteString(),
		Stack:     tp.stack(filterNone),
	}
	tp.publishMap[publishID] = publish

	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_PublishedMsg{PublishedMsg: publish},
	})
	return nil
}

func (tp *traceParser) publishEnd(ts uint64) error {
	publishID := tp.UVarint()
	publish, ok := tp.publishMap[publishID]
	if !ok {
		return eerror.New("trace_parser", "unknown publish", map[string]any{"publishID": publishID})
	}
	publish.EndTime = ts
	publish.MessageId = tp.String()
	publish.Err = tp.ByteString()
	delete(tp.publishMap, publishID)
	return nil
}

func (tp *traceParser) serviceInitStart(ts uint64) error {
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}

	initID := tp.UVarint()
	svcInit := &tracepb.ServiceInit{
		Goid:      tp.UVarint(),
		DefLoc:    int32(tp.UVarint()),
		StartTime: ts,
		Service:   tp.String(),
	}
	tp.serviceInits[initID] = svcInit

	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_ServiceInit{ServiceInit: svcInit},
	})
	return nil
}

func (tp *traceParser) serviceInitEnd(ts uint64) error {
	initID := tp.UVarint()
	svcInit, ok := tp.serviceInits[initID]
	if !ok {
		return eerror.New("trace_parser", "unknown service init", map[string]any{"initID": initID})
	}
	svcInit.EndTime = ts
	svcInit.Err = tp.ByteString()
	if len(svcInit.Err) > 0 {
		svcInit.ErrStack = tp.stack(filterNone)
	}
	delete(tp.serviceInits, initID)
	return nil
}

func (tp *traceParser) cacheOpStart(ts uint64) error {
	opID := tp.UVarint()
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return eerror.New("trace_parser", "unknown request span", map[string]any{"spanID": spanID})
	}

	op := &tracepb.CacheOp{
		Goid:      uint32(tp.UVarint()),
		DefLoc:    int32(tp.UVarint()),
		StartTime: ts,
		Operation: tp.String(),
		Write:     tp.Bool(),
		Result:    tracepb.CacheOp_UNKNOWN,
		Stack:     tp.stack(filterNone),
	}

	numKeys := tp.UVarint()
	op.Keys = make([]string, numKeys)
	for i := 0; i < int(numKeys); i++ {
		op.Keys[i] = tp.String()
	}

	numInputs := tp.UVarint()
	op.Inputs = make([][]byte, numInputs)
	for i := 0; i < int(numInputs); i++ {
		op.Inputs[i] = tp.ByteString()
	}
	tp.cacheMap[opID] = op

	req.Events = append(req.Events, &tracepb.Event{
		Data: &tracepb.Event_Cache{Cache: op},
	})
	return nil
}

func (tp *traceParser) cacheOpEnd(ts uint64) error {
	opID := tp.UVarint()
	op, ok := tp.cacheMap[opID]
	if !ok {
		return eerror.New("trace_parser", "unknown cache", map[string]any{"opID": opID})
	}
	op.EndTime = ts

	res := trace.CacheOpResult(tp.Byte())
	switch res {
	case trace.CacheOK:
		op.Result = tracepb.CacheOp_OK
	case trace.CacheNoSuchKey:
		op.Result = tracepb.CacheOp_NO_SUCH_KEY
	case trace.CacheConflict:
		op.Result = tracepb.CacheOp_CONFLICT
	case trace.CacheErr:
		op.Result = tracepb.CacheOp_ERR
		op.Err = tp.ByteString()
	}

	numOutputs := tp.UVarint()
	op.Outputs = make([][]byte, numOutputs)
	for i := 0; i < int(numOutputs); i++ {
		op.Outputs[i] = tp.ByteString()
	}

	delete(tp.cacheMap, opID)
	return nil
}

type stackFilter int

const (
	filterNone stackFilter = iota
	filterDB
)

func (tp *traceParser) stack(filterMode stackFilter) *tracepb.StackTrace {
	n := int(tp.Byte())
	tr := &tracepb.StackTrace{}
	if n == 0 {
		return tr
	}

	diffs := make([]int64, n)
	for i := 0; i < n; i++ {
		diff := tp.Varint()
		diffs[i] = diff
	}
	tr.Pcs = diffs

	if tp.symTable == nil {
		return tr
	}

	// If we have a symTable, we can extract the full set of frames from the trace
	sym, err := tp.symTable.SymTable(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("could not parse sym table")
		return tr
	}

	prev := int64(0)
	pcs := make([]uint64, n)
	for i := 0; i < n; i++ {
		x := prev + diffs[i]
		prev = x
		pcs[i] = uint64(x) + sym.BaseOffset
	}

	tr.Frames = make([]*tracepb.StackFrame, 0, n)
PCLoop:
	for _, pc := range pcs {
		file, line, fn := sym.PCToLine(pc)
		if fn != nil {
			if filterMode == filterDB && strings.Contains(filepath.ToSlash(file), "/src/database/sql/") {
				continue PCLoop
			}
			tr.Frames = append(tr.Frames, &tracepb.StackFrame{
				Func:     fn.Name,
				Filename: file,
				Line:     int32(line),
			})
		}
	}
	return tr
}

func (tp *traceParser) formattedStack() *tracepb.StackTrace {
	n := int(tp.Byte())
	tr := &tracepb.StackTrace{}
	if n == 0 {
		return tr
	}

	tr.Frames = make([]*tracepb.StackFrame, 0, n)
	for i := 0; i < n; i++ {
		tr.Frames = append(tr.Frames, &tracepb.StackFrame{
			Filename: tp.String(),
			Line:     int32(tp.UVarint()),
			Func:     tp.String(),
		})
	}

	return tr
}

func (tp *traceParser) parseRequestType() (tracepb.Request_Type, error) {
	switch b := tp.Byte(); b {
	case 0x01:
		return tracepb.Request_RPC, nil
	case 0x02:
		return tracepb.Request_AUTH, nil
	case 0x03:
		return tracepb.Request_PUBSUB_MSG, nil
	default:
		return -1, eerror.New("trace_parser", "unknown request type", map[string]any{"type": fmt.Sprintf("%x", b)})
	}
}

func (tp *traceParser) parseTraceID() *tracepb.TraceID {
	var traceID [16]byte
	tp.Bytes(traceID[:])
	return &tracepb.TraceID{
		Low:  bin.Uint64(traceID[:8]),
		High: bin.Uint64(traceID[8:]),
	}
}

func (tp *traceParser) parseHTTPHeaders() map[string]string {
	numHeaders := tp.UVarint()
	h := make(map[string]string, numHeaders)
	for i := uint64(0); i < numHeaders; i++ {
		h[tp.String()] = tp.String()
	}
	return h
}

var bin = binary.LittleEndian

type traceReader struct {
	buf []byte
	off int
	err bool
}

func (tr *traceReader) Offset() int {
	return tr.off
}

func (tr *traceReader) Done() bool {
	return tr.off >= len(tr.buf)
}

func (tr *traceReader) Overflow() bool {
	return tr.err
}

func (tr *traceReader) Bytes(b []byte) {
	n := copy(b, tr.buf[tr.off:])
	tr.off += n
	if len(b) > n {
		tr.err = true
	}
}

func (tr *traceReader) Skip(n int) {
	tr.off += n
	if tr.off > len(tr.buf) {
		tr.off = len(tr.buf)
		tr.err = true
	}
}

func (tr *traceReader) Byte() byte {
	var buf [1]byte
	tr.Bytes(buf[:])
	return buf[0]
}

func (tr *traceReader) Bool() bool {
	return tr.Byte() != 0
}

func (tr *traceReader) String() string {
	return string(tr.ByteString())
}

func (tr *traceReader) ByteString() []byte {
	size := tr.UVarint()
	if (size) == 0 {
		return nil
	}
	b := make([]byte, int(size))
	tr.Bytes(b)
	return b
}

func (tr *traceReader) Time() time.Time {
	sec := tr.Int64()
	nsec := tr.Int32()
	return time.Unix(sec, int64(nsec)).UTC()
}

func (tr *traceReader) Int32() int32 {
	u := tr.Uint32()
	var v int32
	if u&1 == 0 {
		v = int32(u >> 1)
	} else {
		v = ^int32(u >> 1)
	}
	return v
}

func (tr *traceReader) Uint32() uint32 {
	var buf [4]byte
	tr.Bytes(buf[:])
	return bin.Uint32(buf[:])
}

func (tr *traceReader) Int64() int64 {
	u := tr.Uint64()
	var v int64
	if u&1 == 0 {
		v = int64(u >> 1)
	} else {
		v = ^int64(u >> 1)
	}
	return v
}

func (tr *traceReader) Uint64() uint64 {
	var buf [8]byte
	tr.Bytes(buf[:])
	return bin.Uint64(buf[:])
}

func (tr *traceReader) Varint() int64 {
	u := tr.UVarint()
	var v int64
	if u&1 == 0 {
		v = int64(u >> 1)
	} else {
		v = ^int64(u >> 1)
	}
	return v
}

func (tr *traceReader) UVarint() uint64 {
	var u uint64
	for i := 0; tr.off < len(tr.buf); i += 7 {
		b := tr.buf[tr.off]
		u |= uint64(b&^0x80) << i
		tr.off++
		if b&0x80 == 0 {
			break
		}
	}
	return u
}

func (tr *traceReader) Float32() float32 {
	b := tr.Uint32()
	return math.Float32frombits(b)
}

func (tr *traceReader) Float64() float64 {
	b := tr.Uint64()
	return math.Float64frombits(b)
}
