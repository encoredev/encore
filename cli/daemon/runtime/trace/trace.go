package trace

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	tracepb "encr.dev/proto/encore/engine/trace"
	metapb "encr.dev/proto/encore/parser/meta/v1"
	"github.com/rs/zerolog/log"
)

type ID [16]byte

type TraceMeta struct {
	ID    ID
	Reqs  []*tracepb.Request
	AppID string
	EnvID string
	Date  time.Time
	Meta  *metapb.Data
}

// A Store stores traces received from running applications.
type Store struct {
	trmu   sync.Mutex
	traces map[string][]*TraceMeta

	lnmu sync.Mutex
	ln   map[chan<- *TraceMeta]struct{}
}

func NewStore() *Store {
	return &Store{
		traces: make(map[string][]*TraceMeta),
		ln:     make(map[chan<- *TraceMeta]struct{}),
	}
}

func (st *Store) Listen(ch chan<- *TraceMeta) {
	st.lnmu.Lock()
	st.ln[ch] = struct{}{}
	st.lnmu.Unlock()
}

func (st *Store) Store(ctx context.Context, tr *TraceMeta) error {
	st.trmu.Lock()
	st.traces[tr.AppID] = append(st.traces[tr.AppID], tr)
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

func (st *Store) List(appID string) []*TraceMeta {
	st.trmu.Lock()
	tr := st.traces[appID]
	st.trmu.Unlock()
	return tr
}

func Parse(traceID ID, data []byte) ([]*tracepb.Request, error) {
	id := &tracepb.TraceID{
		Low:  bin.Uint64(traceID[:8]),
		High: bin.Uint64(traceID[8:]),
	}
	tp := &traceParser{
		traceReader: traceReader{buf: data},
		traceID:     id,
		reqMap:      make(map[uint64]*tracepb.Request),
		txMap:       make(map[uint64]*tracepb.DBTransaction),
		queryMap:    make(map[uint64]*tracepb.DBQuery),
		callMap:     make(map[uint64]interface{}),
		goMap:       make(map[goKey]*tracepb.Goroutine),
		httpMap:     make(map[uint64]*tracepb.HTTPCall),
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

type traceParser struct {
	traceReader
	traceID  *tracepb.TraceID
	reqs     []*tracepb.Request
	reqMap   map[uint64]*tracepb.Request
	txMap    map[uint64]*tracepb.DBTransaction
	queryMap map[uint64]*tracepb.DBQuery
	callMap  map[uint64]interface{} // *RPCCall or *AuthCall
	httpMap  map[uint64]*tracepb.HTTPCall
	goMap    map[goKey]*tracepb.Goroutine
}

func (tp *traceParser) Parse() error {
	for i := 0; !tp.Done(); i++ {
		ev := tp.Byte()
		ts := tp.Uint64()
		size := int(tp.Uint32())
		startOff := tp.Offset()

		var err error
		switch ev {
		case 0x01:
			err = tp.requestStart(ts)
		case 0x02:
			err = tp.requestEnd(ts)
		case 0x03:
			err = tp.goroutineStart(ts)
		case 0x04:
			err = tp.goroutineEnd(ts)
		case 0x05:
			err = tp.goroutineClear(ts)
		case 0x06:
			err = tp.transactionStart(ts)
		case 0x07:
			err = tp.transactionEnd(ts)
		case 0x08:
			err = tp.queryStart(ts)
		case 0x09:
			err = tp.queryEnd(ts)
		case 0x0A:
			err = tp.callStart(ts)
		case 0x0B:
			err = tp.callEnd(ts)
		case 0x0C, 0x0D:
			// Skip these events for now
			tp.Skip(size)

		case 0x0E:
			err = tp.httpStart(ts)
		case 0x0F:
			err = tp.httpEnd(ts)
		case 0x10:
			err = tp.httpBodyClosed(ts)

		default:
			log.Error().Int("idx", i).Hex("event", []byte{ev}).Msg("trace: unknown event type, skipping")
			tp.Skip(size)
			err = nil
		}
		if err != nil {
			return fmt.Errorf("event #%d: parsing event=%x: %v", i, ev, err)
		}

		if tp.Overflow() {
			return fmt.Errorf("event #%d: invalid trace format (reader overflow parsing event %x)", i, ev)
		} else if off, want := tp.Offset(), startOff+size; off < want {
			log.Error().Int("idx", i).Hex("event", []byte{ev}).Int("remainingBytes", want-off).Msg("trace: parser did not consume whole frame, skipping ahead")
			tp.Skip(want - off)
		} else if off > want {
			return fmt.Errorf("event #%d: parser (event=%x) exceeded frame size by %d bytes", i, ev, off-want)
		}
	}

	return nil
}

func (tp *traceParser) requestStart(ts uint64) error {
	var typ tracepb.Request_Type
	switch b := tp.Byte(); b {
	case 0x01:
		typ = tracepb.Request_RPC
	case 0x02:
		typ = tracepb.Request_AUTH
	default:
		return fmt.Errorf("unknown request type %x", b)
	}

	req := &tracepb.Request{
		TraceId:      tp.traceID,
		SpanId:       tp.Uint64(),
		ParentSpanId: tp.Uint64(),
		StartTime:    tp.Uint64(),
		// EndTime not set yet
		Goid:    uint32(tp.UVarint()),
		CallLoc: int32(tp.UVarint()),
		DefLoc:  int32(tp.UVarint()),
		Uid:     tp.String(),
		Type:    typ,
	}
	// We use event timestamps instead
	req.StartTime = ts

	for n, i := tp.UVarint(), uint64(0); i < n; i++ {
		size := tp.UVarint()
		if size > (10 << 20) {
			return fmt.Errorf("input too large: %d bytes", size)
		}
		input := make([]byte, size)
		tp.Bytes(input)
		req.Inputs = append(req.Inputs, input)
	}
	tp.reqs = append(tp.reqs, req)
	tp.reqMap[req.SpanId] = req
	return nil
}

func (tp *traceParser) requestEnd(ts uint64) error {
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return fmt.Errorf("unknown request span: %v", spanID)
	}
	// dur := ts - rd.startTs
	req.EndTime = ts

	if tp.Byte() == 0 {
		// No error
		for n, i := tp.UVarint(), uint64(0); i < n; i++ {
			size := tp.UVarint()
			if size > (10 << 20) {
				return fmt.Errorf("input too large: %d bytes", size)
			}
			output := make([]byte, size)
			tp.Bytes(output)
			req.Outputs = append(req.Outputs, output)
		}
	} else {
		msg := tp.ByteString()
		if len(msg) == 0 {
			msg = []byte("unknown error")
		}
		req.Err = msg
	}
	return nil
}

func (tp *traceParser) goroutineStart(ts uint64) error {
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return fmt.Errorf("unknown request span id: %v", spanID)
	}
	goid := tp.Uint32()
	g := &tracepb.Goroutine{
		Goid:      goid,
		CallLoc:   0, // not yet supported
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
		return fmt.Errorf("unknown goroutine id: %v", goid)
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
		return fmt.Errorf("unknown goroutine id: %v/%v", spanID, goid)
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
		return fmt.Errorf("unknown request span: %v", spanID)
	}
	goid := uint32(tp.UVarint())
	tx := &tracepb.DBTransaction{
		Goid:      goid,
		StartLoc:  int32(tp.UVarint()),
		StartTime: ts,
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
		return fmt.Errorf("unknown transaction id: %v", txid)
	}
	_ = uint32(tp.UVarint()) // goid
	compl := tp.Byte()
	endLoc := int32(tp.UVarint())
	errMsg := tp.ByteString()

	// It's possible to get multiple transaction end events.
	// Ignore them for now; we will expose this information later.
	if tx.EndTime == 0 {
		tx.EndTime = ts
		tx.EndLoc = endLoc
		tx.Err = errMsg
		switch compl {
		case 0:
			tx.Completion = tracepb.DBTransaction_ROLLBACK
		case 1:
			tx.Completion = tracepb.DBTransaction_COMMIT
		default:
			return fmt.Errorf("unknown completion type: %x", compl)
		}
	}
	return nil
}

func (tp *traceParser) queryStart(ts uint64) error {
	qid := tp.UVarint()
	spanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return fmt.Errorf("unknown request span: %v", spanID)
	}
	txid := tp.UVarint()
	goid := uint32(tp.UVarint())
	q := &tracepb.DBQuery{
		Goid:      goid,
		CallLoc:   int32(tp.UVarint()),
		StartTime: ts,
		Query:     tp.ByteString(),
	}
	tp.queryMap[qid] = q

	if txid != 0 {
		tx, ok := tp.txMap[txid]
		if !ok {
			return fmt.Errorf("unknown transaction id: %v", txid)
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
		return fmt.Errorf("unknown query id: %v", qid)
	}
	q.EndTime = ts
	q.Err = tp.ByteString()
	return nil
}

func (tp *traceParser) callStart(ts uint64) error {
	callID := tp.UVarint()
	spanID := tp.Uint64()
	childSpanID := tp.Uint64()
	req, ok := tp.reqMap[spanID]
	if !ok {
		return fmt.Errorf("unknown request span: %v", spanID)
	}
	c := &tracepb.RPCCall{
		SpanId:    childSpanID,
		Goid:      uint32(tp.UVarint()),
		CallLoc:   int32(tp.UVarint()),
		DefLoc:    int32(tp.UVarint()),
		StartTime: ts,
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
		return fmt.Errorf("unknown call: %v ", callID)
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
		return fmt.Errorf("unknown request span: %v", spanID)
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
		return fmt.Errorf("unknown call: %v ", callID)
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
		return fmt.Errorf("unknown call: %v ", callID)
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
		return nil, fmt.Errorf("unknown http event %v", code)
	}
	return ev, nil
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
	b := make([]byte, int(size))
	tr.Bytes(b)
	return b
}

func (tr *traceReader) Time() time.Time {
	ns := tr.Int64()
	return time.Unix(0, ns)
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
