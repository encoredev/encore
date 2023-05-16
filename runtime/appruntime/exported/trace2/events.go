package trace2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/types/uuid"
)

type EventType byte

const (
	RequestSpanStart       EventType = 0x01
	RequestSpanEnd         EventType = 0x02
	AuthSpanStart          EventType = 0x03
	AuthSpanEnd            EventType = 0x04
	PubsubMessageSpanStart EventType = 0x05
	PubsubMessageSpanEnd   EventType = 0x06
	DBTransactionStart     EventType = 0x07
	DBTransactionEnd       EventType = 0x08
	DBQueryStart           EventType = 0x09
	DBQueryEnd             EventType = 0x0A
	RPCCallStart           EventType = 0x0B
	RPCCallEnd             EventType = 0x0C
	HTTPCallStart          EventType = 0x0D
	HTTPCallEnd            EventType = 0x0E
	LogMessage             EventType = 0x0F
	PubsubPublishStart     EventType = 0x10
	PubsubPublishEnd       EventType = 0x11
	ServiceInitStart       EventType = 0x12
	ServiceInitEnd         EventType = 0x13
	CacheCallStart         EventType = 0x14
	CacheCallEnd           EventType = 0x15
	BodyStream             EventType = 0x16
)

func (te EventType) String() string {
	switch te {
	case RequestSpanStart:
		return "RequestSpanStart"
	case RequestSpanEnd:
		return "RequestSpanEnd"
	case AuthSpanStart:
		return "AuthSpanStart"
	case AuthSpanEnd:
		return "AuthSpanEnd"
	case PubsubMessageSpanStart:
		return "PubsubMessageSpanStart"
	case PubsubMessageSpanEnd:
		return "PubsubMessageSpanEnd"
	case DBTransactionStart:
		return "DBTransactionStart"
	case DBTransactionEnd:
		return "DBTransactionEnd"
	case DBQueryStart:
		return "QueryStart"
	case DBQueryEnd:
		return "QueryEnd"
	case RPCCallStart:
		return "RPCCallStart"
	case RPCCallEnd:
		return "RPCCallEnd"
	case HTTPCallStart:
		return "HTTPCallStart"
	case HTTPCallEnd:
		return "HTTPCallEnd"
	case LogMessage:
		return "LogMessage"
	case PubsubPublishStart:
		return "PubsubPublishStart"
	case PubsubPublishEnd:
		return "PubsubPublishEnd"
	case ServiceInitStart:
		return "ServiceInitStart"
	case ServiceInitEnd:
		return "ServiceInitEnd"
	case CacheCallStart:
		return "CacheCallStart"
	case CacheCallEnd:
		return "CacheCallEnd"
	case BodyStream:
		return "BodyStream"
	default:
		return fmt.Sprintf("Unknown(%x)", byte(te))
	}
}

type EventParams struct {
	TraceID model.TraceID
	SpanID  model.SpanID
	Goid    uint32
	DefLoc  int32
}

type spanStartEventData struct {
	Goid             uint32
	ParentTraceID    model.TraceID
	ParentSpanID     model.SpanID
	DefLoc           int32
	CallerEventID    model.TraceEventID
	ExtCorrelationID string

	ExtraSpace int
}

func (l *Log) newSpanStartEvent(data spanStartEventData) EventBuffer {
	tb := NewEventBuffer(4 + 16 + 8 + 4 + len(data.ExtCorrelationID) + 2 + data.ExtraSpace)
	tb.UVarint(uint64(data.Goid))
	tb.Bytes(data.ParentTraceID[:])
	tb.Bytes(data.ParentSpanID[:])
	tb.UVarint(uint64(data.DefLoc))
	tb.UVarint(uint64(data.CallerEventID))
	tb.String(data.ExtCorrelationID)
	return tb
}

type spanEndEventData struct {
	Duration      time.Duration
	Err           error
	ParentTraceID model.TraceID
	ParentSpanID  model.SpanID
	ExtraSpace    int
}

func (l *Log) newSpanEndEvent(data spanEndEventData) EventBuffer {
	tb := NewEventBuffer(8 + 12 + 8 + data.ExtraSpace)
	tb.Duration(data.Duration)
	tb.ErrWithStack(data.Err)
	tb.Bytes(data.ParentTraceID[:])
	tb.Bytes(data.ParentSpanID[:])
	return tb
}

type eventData struct {
	Common             EventParams
	CorrelationEventID EventID
	ExtraSpace         int
}

func (l *Log) newEvent(data eventData) EventBuffer {
	tb := NewEventBuffer(4 + 4 + data.ExtraSpace)
	tb.UVarint(uint64(data.Common.DefLoc))
	tb.UVarint(uint64(data.Common.Goid))
	tb.EventID(data.CorrelationEventID)
	return tb
}

func (l *Log) RequestSpanStart(req *model.Request, goid uint32) {
	data := req.RPCData
	desc := data.Desc
	tb := l.newSpanStartEvent(spanStartEventData{
		ParentTraceID:    req.ParentTraceID,
		ParentSpanID:     req.ParentID,
		DefLoc:           req.DefLoc,
		Goid:             goid,
		CallerEventID:    req.CallerEventID,
		ExtCorrelationID: req.ExtCorrelationID,
		ExtraSpace:       100,
	})

	tb.String(desc.Service)
	tb.String(desc.Endpoint)
	tb.String(data.HTTPMethod)

	tb.String(data.Path)
	tb.UVarint(uint64(len(data.PathParams)))
	for _, pp := range data.PathParams {
		tb.String(pp.Value)
	}

	l.logHeaders(&tb, data.RequestHeaders)
	tb.ByteString(data.NonRawPayload)
	tb.String(req.ExtCorrelationID)
	tb.String(string(data.UserID))

	l.Add(Event{
		Type:    RequestSpanStart,
		TraceID: req.TraceID,
		SpanID:  req.SpanID,
		Data:    tb,
	})
}

type RequestSpanEndParams struct {
	EventParams
	Req  *model.Request
	Resp *model.Response
}

func (l *Log) RequestSpanEnd(p RequestSpanEndParams) {
	desc := p.Req.RPCData.Desc
	tb := l.newSpanEndEvent(spanEndEventData{
		Duration:      p.Resp.Duration,
		Err:           p.Resp.Err,
		ParentTraceID: p.Req.ParentTraceID,
		ParentSpanID:  p.Req.ParentID,
		ExtraSpace:    len(desc.Service) + len(desc.Endpoint) + 64 + len(p.Resp.Payload),
	})

	tb.String(desc.Service)
	tb.String(desc.Endpoint)

	tb.UVarint(uint64(p.Resp.HTTPStatus))
	l.logHeaders(&tb, p.Resp.RawResponseHeaders)
	tb.ByteString(p.Resp.Payload)

	l.Add(Event{
		Type:    RequestSpanEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func (l *Log) AuthSpanStart(req *model.Request, goid uint32) {
	data := req.RPCData
	desc := data.Desc
	tb := l.newSpanStartEvent(spanStartEventData{
		ParentTraceID:    req.ParentTraceID,
		ParentSpanID:     req.ParentID,
		DefLoc:           req.DefLoc,
		Goid:             goid,
		CallerEventID:    req.CallerEventID,
		ExtCorrelationID: req.ExtCorrelationID,
		ExtraSpace:       len(desc.Service) + len(desc.Endpoint) + len(data.NonRawPayload) + 5,
	})

	tb.String(desc.Service)
	tb.String(desc.Endpoint)
	tb.ByteString(data.NonRawPayload)

	l.Add(Event{
		Type:    AuthSpanStart,
		TraceID: req.TraceID,
		SpanID:  req.SpanID,
		Data:    tb,
	})
}

type AuthSpanEndParams struct {
	EventParams
	Req  *model.Request
	Resp *model.Response
}

func (l *Log) AuthSpanEnd(p AuthSpanEndParams) {
	desc := p.Req.RPCData.Desc
	tb := l.newSpanEndEvent(spanEndEventData{
		Duration:      p.Resp.Duration,
		Err:           p.Resp.Err,
		ParentTraceID: p.Req.ParentTraceID,
		ParentSpanID:  p.Req.ParentID,
		ExtraSpace:    len(desc.Service) + len(desc.Endpoint) + 64 + len(p.Resp.Payload),
	})

	tb.String(desc.Service)
	tb.String(desc.Endpoint)
	tb.String(string(p.Resp.AuthUID))
	tb.ByteString(p.Resp.Payload)

	l.Add(Event{
		Type:    AuthSpanEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func (l *Log) PubsubMessageSpanStart(req *model.Request, goid uint32) {
	data := req.MsgData
	tb := l.newSpanStartEvent(spanStartEventData{
		ParentTraceID:    req.ParentTraceID,
		ParentSpanID:     req.ParentID,
		DefLoc:           req.DefLoc,
		Goid:             goid,
		CallerEventID:    req.CallerEventID,
		ExtCorrelationID: req.ExtCorrelationID,
		ExtraSpace:       len(data.Service) + len(data.Topic) + len(data.Subscription) + len(data.Payload) + 20,
	})

	tb.String(data.Service)
	tb.String(data.Topic)
	tb.String(data.Subscription)
	tb.String(data.MessageID)
	tb.UVarint(uint64(data.Attempt))
	tb.Time(data.Published)
	tb.ByteString(data.Payload)

	l.Add(Event{
		Type:    PubsubMessageSpanStart,
		TraceID: req.TraceID,
		SpanID:  req.SpanID,
		Data:    tb,
	})
}

type PubsubMessageSpanEndParams struct {
	EventParams
	Req  *model.Request
	Resp *model.Response
}

func (l *Log) PubsubMessageSpanEnd(p PubsubMessageSpanEndParams) {
	msg := p.Req.MsgData
	tb := l.newSpanEndEvent(spanEndEventData{
		Duration:      p.Resp.Duration,
		Err:           p.Resp.Err,
		ParentTraceID: p.Req.ParentTraceID,
		ParentSpanID:  p.Req.ParentID,
		ExtraSpace:    len(msg.Service) + len(msg.Topic) + len(msg.Subscription) + 4,
	})

	tb.String(msg.Service)
	tb.String(msg.Topic)
	tb.String(msg.Subscription)

	l.Add(Event{
		Type:    PubsubMessageSpanEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func (l *Log) RPCCallStart(call *model.APICall, goid uint32) EventID {
	tb := l.newEvent(eventData{
		Common: EventParams{
			Goid:   goid,
			DefLoc: call.DefLoc,
		},
		ExtraSpace: len(call.TargetServiceName) + len(call.TargetServiceName) + 4 + 64,
	})
	tb.String(call.TargetServiceName)
	tb.String(call.TargetEndpointName)
	tb.Stack(stack.Build(3))
	return l.Add(Event{
		Type:    RPCCallStart,
		TraceID: call.Source.TraceID,
		SpanID:  call.Source.SpanID,
		Data:    tb,
	})
}

func (l *Log) RPCCallEnd(call *model.APICall, goid uint32, err error) {
	tb := l.newEvent(eventData{
		Common:             EventParams{Goid: goid},
		ExtraSpace:         64,
		CorrelationEventID: call.StartEventID,
	})

	tb.ErrWithStack(err)

	l.Add(Event{
		Type:    RPCCallEnd,
		TraceID: call.Source.TraceID,
		SpanID:  call.Source.SpanID,
		Data:    tb,
	})
}

type DBQueryStartParams struct {
	EventParams
	TxStartID EventID // zero if not in a transaction
	Stack     stack.Stack
	Query     string
}

func (l *Log) DBQueryStart(p DBQueryStartParams) EventID {
	tb := l.newEvent(eventData{
		Common:             p.EventParams,
		CorrelationEventID: p.TxStartID,
		ExtraSpace:         64,
	})

	tb.String(p.Query)
	tb.Stack(p.Stack)

	return l.Add(Event{
		Type:    DBQueryStart,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func (l *Log) DBQueryEnd(p EventParams, startID EventID, err error) {
	tb := l.newEvent(eventData{
		Common:             p,
		ExtraSpace:         64,
		CorrelationEventID: startID,
	})
	tb.ErrWithStack(err)
	l.Add(Event{
		Type:    DBQueryEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func (l *Log) DBTransactionStart(p EventParams, stack stack.Stack) EventID {
	tb := l.newEvent(eventData{
		Common:     p,
		ExtraSpace: 64,
	})

	tb.Stack(stack)

	return l.Add(Event{
		Type:    DBTransactionStart,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

type DBTransactionEndParams struct {
	EventParams
	StartID EventID
	Commit  bool
	Err     error
	Stack   stack.Stack
}

func (l *Log) DBTransactionEnd(p DBTransactionEndParams) {
	tb := l.newEvent(eventData{
		Common:             p.EventParams,
		CorrelationEventID: p.StartID,
		ExtraSpace:         64,
	})

	tb.Bool(p.Commit)
	tb.Stack(p.Stack)
	tb.ErrWithStack(p.Err)

	l.Add(Event{
		Type:    DBTransactionEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

type PubsubPublishStartParams struct {
	EventParams
	Topic   string
	Message []byte
	Stack   stack.Stack
}

func (l *Log) PubsubPublishStart(p PubsubPublishStartParams) EventID {
	tb := l.newEvent(eventData{
		Common:     p.EventParams,
		ExtraSpace: 64,
	})

	tb.String(p.Topic)
	tb.ByteString(p.Message)
	tb.Stack(p.Stack)

	return l.Add(Event{
		Type:    PubsubPublishStart,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

type PubsubPublishEndParams struct {
	EventParams
	StartID   EventID
	MessageID string
	Err       error
}

func (l *Log) PubsubPublishEnd(p PubsubPublishEndParams) {
	tb := l.newEvent(eventData{
		Common:             p.EventParams,
		CorrelationEventID: p.StartID,
		ExtraSpace:         64,
	})

	tb.String(p.MessageID)
	tb.ErrWithStack(p.Err)

	l.Add(Event{
		Type:    PubsubPublishEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

type ServiceInitStartParams struct {
	EventParams
	Service string
}

func (l *Log) ServiceInitStart(p ServiceInitStartParams) EventID {
	tb := l.newEvent(eventData{
		Common:     p.EventParams,
		ExtraSpace: 64,
	})
	tb.String(p.Service)

	return l.Add(Event{
		Type:    ServiceInitStart,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func (l *Log) ServiceInitEnd(p EventParams, start EventID, err error) {
	tb := l.newEvent(eventData{
		Common:             p,
		ExtraSpace:         64,
		CorrelationEventID: start,
	})

	tb.ErrWithStack(err)

	l.Add(Event{
		Type:    ServiceInitEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

type CacheCallStartParams struct {
	EventParams
	Operation string
	IsWrite   bool
	Keys      []string
	Stack     stack.Stack
}

func (l *Log) CacheCallStart(p CacheCallStartParams) EventID {
	tb := l.newEvent(eventData{
		Common:     p.EventParams,
		ExtraSpace: 64,
	})

	tb.String(p.Operation)
	tb.Bool(p.IsWrite)
	tb.Stack(p.Stack)

	tb.UVarint(uint64(len(p.Keys)))
	for _, k := range p.Keys {
		tb.String(k)
	}

	return l.Add(Event{
		Type:    CacheCallStart,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

type CacheCallEndParams struct {
	EventParams
	StartID EventID
	Res     CacheCallResult
	Err     error
}

func (l *Log) CacheCallEnd(p CacheCallEndParams) {
	tb := l.newEvent(eventData{
		Common:             p.EventParams,
		ExtraSpace:         64,
		CorrelationEventID: p.StartID,
	})

	tb.Byte(byte(p.Res))
	tb.ErrWithStack(p.Err)

	l.Add(Event{
		Type:    CacheCallEnd,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

type CacheCallResult uint8

const (
	CacheOK        CacheCallResult = 1
	CacheNoSuchKey CacheCallResult = 2
	CacheConflict  CacheCallResult = 3
	CacheErr       CacheCallResult = 4
)

type BodyStreamParams struct {
	EventParams

	// IsResponse specifies whether the stream was a response body
	// or a request body.
	IsResponse bool

	// Overflowed specifies whether the capturing overflowed.
	Overflowed bool

	// Data is the data read.
	Data []byte
}

func (l *Log) BodyStream(p BodyStreamParams) {
	tb := l.newEvent(eventData{
		Common:     p.EventParams,
		ExtraSpace: 64,
	})

	var flags byte = 0
	if p.IsResponse {
		flags |= 1 << 0
	}
	if p.Overflowed {
		flags |= 1 << 1
	}
	tb.Byte(flags)
	tb.ByteString(p.Data)

	l.Add(Event{
		Type:    BodyStream,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func (l *Log) logHeaders(tb *EventBuffer, headers http.Header) {
	tb.UVarint(uint64(len(headers)))
	for k, v := range headers {
		firstVal := ""
		if len(v) > 0 {
			firstVal = v[0]
		}
		tb.String(k)
		tb.String(firstVal)
	}
}

type LogMessageParams struct {
	EventParams
	Level  model.LogLevel
	Msg    string
	Stack  stack.Stack
	Fields []LogField
}

type LogField struct {
	Key   string
	Value any
}

func (l *Log) LogMessage(p LogMessageParams) {
	tb := l.newEvent(eventData{
		Common:     p.EventParams,
		ExtraSpace: len(p.Msg) + 1 + 64*len(p.Fields),
	})

	tb.Byte(byte(p.Level))
	tb.String(p.Msg)

	tb.UVarint(uint64(len(p.Fields)))
	for _, f := range p.Fields {
		addLogField(&tb, f.Key, f.Value)
	}
	tb.Stack(p.Stack)

	l.Add(Event{
		Type:    LogMessage,
		TraceID: p.TraceID,
		SpanID:  p.SpanID,
		Data:    tb,
	})
}

func addLogField(tb *EventBuffer, key string, val any) {
	switch val := val.(type) {
	case error:
		tb.Byte(byte(model.ErrField))
		tb.String(key)
		tb.ErrWithStack(val)
	case string:
		tb.Byte(byte(model.StringField))
		tb.String(key)
		tb.String(val)
	case bool:
		tb.Byte(byte(model.BoolField))
		tb.String(key)
		tb.Bool(val)
	case time.Time:
		tb.Byte(byte(model.TimeField))
		tb.String(key)
		tb.Time(val)
	case time.Duration:
		tb.Byte(byte(model.DurationField))
		tb.String(key)
		tb.Int64(int64(val))
	case uuid.UUID:
		tb.Byte(byte(model.UUIDField))
		tb.String(key)
		tb.Bytes(val[:])

	default:
		tb.Byte(byte(model.JSONField))
		tb.String(key)
		data, err := json.Marshal(val)
		if err != nil {
			tb.ByteString(nil)
			tb.ErrWithStack(err)
		} else {
			tb.ByteString(data)
			tb.ErrWithStack(nil)
		}

	case int8:
		tb.Byte(byte(model.IntField))
		tb.String(key)
		tb.Varint(int64(val))
	case int16:
		tb.Byte(byte(model.IntField))
		tb.String(key)
		tb.Varint(int64(val))
	case int32:
		tb.Byte(byte(model.IntField))
		tb.String(key)
		tb.Varint(int64(val))
	case int64:
		tb.Byte(byte(model.IntField))
		tb.String(key)
		tb.Varint(int64(val))
	case int:
		tb.Byte(byte(model.IntField))
		tb.String(key)
		tb.Varint(int64(val))

	case uint8:
		tb.Byte(byte(model.UintField))
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint16:
		tb.Byte(byte(model.UintField))
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint32:
		tb.Byte(byte(model.UintField))
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint64:
		tb.Byte(byte(model.UintField))
		tb.String(key)
		tb.UVarint(uint64(val))
	case uint:
		tb.Byte(byte(model.UintField))
		tb.String(key)
		tb.UVarint(uint64(val))

	case float32:
		tb.Byte(byte(model.Float32Field))
		tb.String(key)
		tb.Float32(val)
	case float64:
		tb.Byte(byte(model.Float64Field))
		tb.String(key)
		tb.Float64(val)
	}
}
