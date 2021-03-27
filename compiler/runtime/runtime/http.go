package runtime

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"sync"
	"sync/atomic"
)

var httpReqIDCtr uint64

type httpRoundTrip struct {
	ReqID  uint64
	SpanID SpanID

	mu     sync.Mutex
	events []httpEvent
}

func (rt *httpRoundTrip) getConn(hostPort string) {
	rt.addEvent(getConn, &getConnEvent{hostPort: hostPort})
}

func (rt *httpRoundTrip) gotConn(info httptrace.GotConnInfo) {
	rt.addEvent(gotConn, &gotConnEvent{info: info})
}

func (rt *httpRoundTrip) gotFirstResponseByte() {
	rt.addEvent(gotFirstResponseByte, nil)
}

func (rt *httpRoundTrip) got1xxResponse(code int, header textproto.MIMEHeader) error {
	rt.addEvent(got1xxResponse, &got1xxResponseEvent{code: code, header: header})
	return nil
}

func (rt *httpRoundTrip) dnsStart(info httptrace.DNSStartInfo) {
	rt.addEvent(dnsStart, &dnsStartEvent{info: info})
}

func (rt *httpRoundTrip) dnsDone(info httptrace.DNSDoneInfo) {
	rt.addEvent(dnsDone, &dnsDoneEvent{info: info})
}

func (rt *httpRoundTrip) connectStart(network, addr string) {
	rt.addEvent(connectStart, &connectStartEvent{network: network, addr: addr})
}

func (rt *httpRoundTrip) connectDone(network, addr string, err error) {
	rt.addEvent(connectDone, &connectDoneEvent{network: network, addr: addr, err: err})
}

func (rt *httpRoundTrip) tlsHandshakeStart() {
	rt.addEvent(tlsHandshakeStart, nil)
}

func (rt *httpRoundTrip) tlsHandshakeDone(state tls.ConnectionState, err error) {
	rt.addEvent(tlsHandshakeDone, &tlsHandshakeDoneEvent{info: state, err: err})
}

func (rt *httpRoundTrip) wroteHeaders() {
	rt.addEvent(wroteHeaders, nil)
}

func (rt *httpRoundTrip) wroteRequest(info httptrace.WroteRequestInfo) {
	rt.addEvent(wroteRequest, &wroteRequestEvent{info: info})
}

func (rt *httpRoundTrip) wait100Continue() {
	rt.addEvent(wait100Continue, nil)
}

func (rt *httpRoundTrip) addEvent(code httpEventCode, data httpEventData) {
	ts := nanotime()
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.events = append(rt.events, httpEvent{
		code: code,
		ts:   ts,
		data: data,
	})
}

func (rt *httpRoundTrip) encodeEvents(tb *TraceBuf) {
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

func httpBeginRoundTrip(req *http.Request) (context.Context, error) {
	g := encoreGetG()
	if g == nil || g.req == nil || !g.req.data.Traced {
		return req.Context(), nil
	} else if req.URL == nil {
		return nil, fmt.Errorf("http: nil Request.URL")
	}

	spanID, err := genSpanID()
	if err != nil {
		return nil, err
	}

	reqID := atomic.AddUint64(&httpReqIDCtr, 1)

	tb := NewTraceBuf(8 + 4 + 4 + 4 + len(req.Method) + 128)
	tb.UVarint(reqID)
	tb.Bytes(g.req.data.SpanID[:])
	tb.Bytes(spanID[:])
	tb.UVarint(uint64(g.goid))
	tb.String(req.Method)
	tb.String(req.URL.String())

	encoreTraceEvent(HTTPCallStart, tb.Buf())

	rt := &httpRoundTrip{
		ReqID:  reqID,
		SpanID: spanID,
	}
	ctx := context.WithValue(req.Context(), rtKey, rt)
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

func httpCompleteRoundTrip(req *http.Request, resp *http.Response, err error) {
	rt, ok := req.Context().Value(rtKey).(*httpRoundTrip)
	if !ok {
		return
	}

	tb := NewTraceBuf(8 + 4 + 4 + 4)
	tb.UVarint(rt.ReqID)
	if err != nil {
		msg := err.Error()
		if msg == "" {
			msg = "unknown error"
		}
		tb.String(msg)
		tb.UVarint(0)
	} else {
		tb.String("")
		tb.UVarint(uint64(resp.StatusCode))
	}
	rt.encodeEvents(&tb)
	encoreTraceEvent(HTTPCallEnd, tb.Buf())

	if req.Method != "HEAD" {
		resp.Body = wrapRespBody(resp.Body, rt)
	}
}

func (rt *httpRoundTrip) ClosedBody(err error) {
	tb := NewTraceBuf(8 + 4)
	tb.UVarint(rt.ReqID)
	if err != nil {
		msg := err.Error()
		if msg == "" {
			msg = "unknown error"
		}
		tb.String(msg)
	} else {
		tb.String("")
	}
	encoreTraceEvent(HTTPCallBodyClosed, tb.Buf())
}

func wrapRespBody(body io.ReadCloser, rt *httpRoundTrip) io.ReadCloser {
	readWriteCloser, ok := body.(io.ReadWriteCloser)
	if ok {
		return writerCloseTracker{readWriteCloser, rt}
	} else {
		return closeTracker{body, rt}
	}

}

type closeTracker struct {
	io.ReadCloser
	rt *httpRoundTrip
}

func (c closeTracker) Close() error {
	err := c.ReadCloser.Close()
	c.rt.ClosedBody(err)
	return err
}

type writerCloseTracker struct {
	io.ReadWriteCloser
	rt *httpRoundTrip
}

func (c writerCloseTracker) Close() error {
	err := c.ReadWriteCloser.Close()
	c.rt.ClosedBody(err)
	return err
}

type httpEvent struct {
	code httpEventCode
	ts   int64
	data httpEventData // or nil
}

type httpEventData interface {
	Encode(tb *TraceBuf)
}

type httpEventCode byte

const (
	getConn              = 0x01
	gotConn              = 0x02
	gotFirstResponseByte = 0x03
	got1xxResponse       = 0x04
	dnsStart             = 0x05
	dnsDone              = 0x06
	connectStart         = 0x07
	connectDone          = 0x08
	tlsHandshakeStart    = 0x09
	tlsHandshakeDone     = 0x0A
	wroteHeaders         = 0x0B
	wroteRequest         = 0x0C
	wait100Continue      = 0x0D
)

type getConnEvent struct {
	hostPort string
}

func (e *getConnEvent) Encode(tb *TraceBuf) {
	tb.String(e.hostPort)
}

type gotConnEvent struct {
	info httptrace.GotConnInfo
}

func (e *gotConnEvent) Encode(tb *TraceBuf) {
	tb.Bool(e.info.Reused)
	tb.Bool(e.info.WasIdle)
	tb.Int64(int64(e.info.IdleTime))
}

type got1xxResponseEvent struct {
	code   int
	header textproto.MIMEHeader
}

func (e *got1xxResponseEvent) Encode(tb *TraceBuf) {
	tb.Varint(int64(e.code))
	// TODO: write header as well?
}

type dnsStartEvent struct {
	info httptrace.DNSStartInfo
}

func (e *dnsStartEvent) Encode(tb *TraceBuf) {
	tb.String(e.info.Host)
}

type dnsDoneEvent struct {
	info httptrace.DNSDoneInfo
}

func (e *dnsDoneEvent) Encode(tb *TraceBuf) {
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

func (e *connectStartEvent) Encode(tb *TraceBuf) {
	tb.String(e.network)
	tb.String(e.addr)
}

type connectDoneEvent struct {
	network string
	addr    string
	err     error
}

func (e *connectDoneEvent) Encode(tb *TraceBuf) {
	tb.String(e.network)
	tb.String(e.addr)
	tb.Err(e.err)
}

type tlsHandshakeDoneEvent struct {
	info tls.ConnectionState
	err  error
}

func (e *tlsHandshakeDoneEvent) Encode(tb *TraceBuf) {
	tb.Err(e.err)
	tb.Uint32(uint32(e.info.Version))
	tb.Uint32(uint32(e.info.CipherSuite))
	tb.String(e.info.ServerName)
	tb.String(e.info.NegotiatedProtocol)
}

type wroteRequestEvent struct {
	info httptrace.WroteRequestInfo
}

func (e *wroteRequestEvent) Encode(tb *TraceBuf) {
	tb.Err(e.info.Err)
}

type contextKey int

const (
	rtKey contextKey = iota
)
