package trace2

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"sync"
	_ "unsafe" // for go:linkname

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
)

func (l *Log) HTTPBeginRoundTrip(httpReq *http.Request, req *model.Request, goid uint32) (context.Context, error) {
	if l == nil {
		return httpReq.Context(), nil
	}

	callCorrelationParentSpanID, err := model.GenSpanID()
	if err != nil {
		return nil, err
	}

	requestURL := httpReq.URL.String()

	tb := l.newEvent(eventData{
		Common:     EventParams{Goid: goid},
		ExtraSpace: 8 + len(httpReq.Method) + len(requestURL) + 4,
	})

	tb.Bytes(callCorrelationParentSpanID[:])
	tb.String(httpReq.Method)
	tb.String(requestURL)
	tb.Stack(stack.Build(4))
	tb.Int64(nanotime())

	eventID := l.Add(Event{
		Type:    HTTPCallStart,
		TraceID: req.TraceID,
		SpanID:  req.SpanID,
		Data:    tb,
	})

	rt := &httpRoundTrip{
		TraceID:                 req.TraceID,
		SpanID:                  req.SpanID,
		StartID:                 eventID,
		CorrelationParentSpanID: callCorrelationParentSpanID,
		log:                     l,
	}

	ctx := context.WithValue(httpReq.Context(), rtKey, rt)
	tr := &httptrace.ClientTrace{
		GetConn:              rt.getConn,
		GotConn:              rt.gotConn,
		GotFirstResponseByte: rt.gotFirstResponseByte,
		Got1xxResponse:       rt.got1xxResponse,
		DNSStart:             rt.dnsStart,
		DNSDone:              rt.dnsDone,
		ConnectStart:         rt.connectStart,
		ConnectDone:          rt.connectDone,
		TLSHandshakeStart:    rt.tlsHandshakeStart,
		TLSHandshakeDone:     rt.tlsHandshakeDone,
		WroteHeaders:         rt.wroteHeaders,
		Wait100Continue:      rt.wait100Continue,
		WroteRequest:         rt.wroteRequest,
	}
	return httptrace.WithClientTrace(ctx, tr), nil
}

func (l *Log) HTTPCompleteRoundTrip(req *http.Request, resp *http.Response, goid uint32, err error) {
	rt, ok := req.Context().Value(rtKey).(*httpRoundTrip)
	if !ok {
		return
	}

	tb := l.newEvent(eventData{
		Common:             EventParams{Goid: goid},
		CorrelationEventID: rt.StartID,
		ExtraSpace:         64,
	})

	if resp != nil {
		tb.UVarint(uint64(resp.StatusCode))
	} else {
		tb.UVarint(0)
	}
	tb.ErrWithStack(err)

	rt.encodeEvents(&tb)
	rt.log.Add(Event{
		Type:    HTTPCallEnd,
		TraceID: rt.TraceID,
		SpanID:  rt.SpanID,
		Data:    tb,
	})

	if req.Method != "HEAD" && resp != nil {
		resp.Body = wrapRespBody(resp.Body, rt)
	}
}

type httpRoundTrip struct {
	TraceID                 model.TraceID
	SpanID                  model.SpanID
	StartID                 EventID
	CorrelationParentSpanID model.SpanID

	log Logger

	mu     sync.Mutex
	events []httpEvent
}

func (rt *httpRoundTrip) getConn(hostPort string) {
	rt.addEvent(GetConn, &getConnEvent{hostPort: hostPort})
}

func (rt *httpRoundTrip) gotConn(info httptrace.GotConnInfo) {
	rt.addEvent(GotConn, &gotConnEvent{info: info})
}

func (rt *httpRoundTrip) gotFirstResponseByte() {
	rt.addEvent(GotFirstResponseByte, nil)
}

func (rt *httpRoundTrip) got1xxResponse(code int, header textproto.MIMEHeader) error {
	rt.addEvent(Got1xxResponse, &got1xxResponseEvent{code: code, header: header})
	return nil
}

func (rt *httpRoundTrip) dnsStart(info httptrace.DNSStartInfo) {
	rt.addEvent(DNSStart, &dnsStartEvent{info: info})
}

func (rt *httpRoundTrip) dnsDone(info httptrace.DNSDoneInfo) {
	rt.addEvent(DNSDone, &dnsDoneEvent{info: info})
}

func (rt *httpRoundTrip) connectStart(network, addr string) {
	rt.addEvent(ConnectStart, &connectStartEvent{network: network, addr: addr})
}

func (rt *httpRoundTrip) connectDone(network, addr string, err error) {
	rt.addEvent(ConnectDone, &connectDoneEvent{network: network, addr: addr, err: err})
}

func (rt *httpRoundTrip) tlsHandshakeStart() {
	rt.addEvent(TLSHandshakeStart, nil)
}

func (rt *httpRoundTrip) tlsHandshakeDone(state tls.ConnectionState, err error) {
	rt.addEvent(TLSHandshakeDone, &tlsHandshakeDoneEvent{info: state, err: err})
}

func (rt *httpRoundTrip) wroteHeaders() {
	rt.addEvent(WroteHeaders, nil)
}

func (rt *httpRoundTrip) wroteRequest(info httptrace.WroteRequestInfo) {
	rt.addEvent(WroteRequest, &wroteRequestEvent{info: info})
}

func (rt *httpRoundTrip) wait100Continue() {
	rt.addEvent(Wait100Continue, nil)
}

func (rt *httpRoundTrip) addEvent(code HTTPEventCode, data httpEventData) {
	ts := nanotime()
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.events = append(rt.events, httpEvent{
		code: code,
		ts:   ts,
		data: data,
	})
}

func (rt *httpRoundTrip) encodeEvents(tb *EventBuffer) {
	rt.mu.Lock()
	n := len(rt.events)
	evs := rt.events[:]
	rt.mu.Unlock()

	tb.UVarint(uint64(n))
	for _, e := range evs {
		tb.Bytes([]byte{byte(e.code)})
		tb.Int64(e.ts)
		if e.data != nil {
			e.data.Encode(tb)
		}
	}
}

func (rt *httpRoundTrip) closedBody(err error) {
	rt.addEvent(ClosedBody, &closedBodyEvent{err: err})
}

func wrapRespBody(body io.ReadCloser, rt *httpRoundTrip) io.ReadCloser {
	if readWriteCloser, ok := body.(io.ReadWriteCloser); ok {
		return writerCloseTracker{readWriteCloser, rt}
	}
	return closeTracker{body, rt}
}

type closeTracker struct {
	io.ReadCloser
	rt *httpRoundTrip
}

func (c closeTracker) Close() error {
	err := c.ReadCloser.Close()
	c.rt.closedBody(err)
	return err
}

type writerCloseTracker struct {
	io.ReadWriteCloser
	rt *httpRoundTrip
}

func (c writerCloseTracker) Close() error {
	err := c.ReadWriteCloser.Close()
	c.rt.closedBody(err)
	return err
}

type httpEvent struct {
	code HTTPEventCode
	ts   int64
	data httpEventData // or nil
}

type httpEventData interface {
	Encode(tb *EventBuffer)
}

type HTTPEventCode byte

const (
	GetConn              = 1
	GotConn              = 2
	GotFirstResponseByte = 3
	Got1xxResponse       = 4
	DNSStart             = 5
	DNSDone              = 6
	ConnectStart         = 7
	ConnectDone          = 8
	TLSHandshakeStart    = 9
	TLSHandshakeDone     = 10
	WroteHeaders         = 11
	WroteRequest         = 12
	Wait100Continue      = 13
	ClosedBody           = 14
)

type getConnEvent struct {
	hostPort string
}

func (e *getConnEvent) Encode(tb *EventBuffer) {
	tb.String(e.hostPort)
}

type gotConnEvent struct {
	info httptrace.GotConnInfo
}

func (e *gotConnEvent) Encode(tb *EventBuffer) {
	tb.Bool(e.info.Reused)
	tb.Bool(e.info.WasIdle)
	tb.Int64(int64(e.info.IdleTime))
}

type got1xxResponseEvent struct {
	code   int
	header textproto.MIMEHeader
}

func (e *got1xxResponseEvent) Encode(tb *EventBuffer) {
	tb.Varint(int64(e.code))
	// TODO: write header as well?
}

type dnsStartEvent struct {
	info httptrace.DNSStartInfo
}

func (e *dnsStartEvent) Encode(tb *EventBuffer) {
	tb.String(e.info.Host)
}

type dnsDoneEvent struct {
	info httptrace.DNSDoneInfo
}

func (e *dnsDoneEvent) Encode(tb *EventBuffer) {
	if err := e.info.Err; err != nil {
		msg := err.Error()
		if msg == "" {
			msg = "unknown error"
		}
		tb.String(msg)
	} else {
		tb.String("")
	}
	tb.UVarint(uint64(len(e.info.Addrs)))
	for _, a := range e.info.Addrs {
		tb.ByteString(a.IP)
	}
}

type connectStartEvent struct {
	network string
	addr    string
}

func (e *connectStartEvent) Encode(tb *EventBuffer) {
	tb.String(e.network)
	tb.String(e.addr)
}

type connectDoneEvent struct {
	network string
	addr    string
	err     error
}

func (e *connectDoneEvent) Encode(tb *EventBuffer) {
	tb.String(e.network)
	tb.String(e.addr)
	tb.Err(e.err)
}

type tlsHandshakeDoneEvent struct {
	info tls.ConnectionState
	err  error
}

func (e *tlsHandshakeDoneEvent) Encode(tb *EventBuffer) {
	tb.Err(e.err)
	tb.Uint32(uint32(e.info.Version))
	tb.Uint32(uint32(e.info.CipherSuite))
	tb.String(e.info.ServerName)
	tb.String(e.info.NegotiatedProtocol)
}

type wroteRequestEvent struct {
	info httptrace.WroteRequestInfo
}

func (e *wroteRequestEvent) Encode(tb *EventBuffer) {
	tb.Err(e.info.Err)
}

type closedBodyEvent struct {
	err error
}

func (e *closedBodyEvent) Encode(tb *EventBuffer) {
	tb.Err(e.err)
}

type contextKey int

const (
	rtKey contextKey = iota
)
