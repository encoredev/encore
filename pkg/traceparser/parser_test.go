package traceparser

import (
	"bufio"
	"bytes"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/types/uuid"
	tracepb2 "encr.dev/proto/encore/engine/trace2"
)

func TestParse(t *testing.T) {
	traceID := model.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := model.SpanID{8, 7, 6, 5, 4, 3, 2, 1}
	now := time.Now()
	err := errors.New("some-error")
	goid := uint32(123)
	defLoc := uint32(456)
	udefLoc := uint32(defLoc) // for compat
	uuidVal := uuid.UUID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	pbNow := timestamppb.New(now)
	pbTraceID := &tracepb2.TraceID{High: 1157159078456920585, Low: 578437695752307201}
	pbSpanID := uint64(72623859790382856)
	pbErr := &tracepb2.Error{Msg: "some-error"}
	pbUUID := uuidVal.Bytes()

	ep := trace2.EventParams{TraceID: traceID, SpanID: spanID, Goid: goid, DefLoc: defLoc}

	tests := []struct {
		Name string
		Emit func(l *trace2.Log)
		Want *tracepb2.TraceEvent
	}{
		{
			Name: "RequestSpanStart",
			Emit: func(l *trace2.Log) {
				l.RequestSpanStart(&model.Request{
					Type:         model.RPCCall,
					TraceID:      traceID,
					SpanID:       spanID,
					ParentSpanID: model.SpanID{},
					Start:        now,
					Traced:       true,
					DefLoc:       defLoc,
					RPCData: &model.RPCData{
						Desc: &model.RPCDesc{
							Service:  "service",
							Endpoint: "endpoint",
							Raw:      false,
						},
						HTTPMethod:     "POST",
						Path:           "/path/hello",
						PathParams:     model.PathParams{{Name: "one", Value: "hello"}},
						UserID:         "userid",
						AuthData:       nil,
						NonRawPayload:  []byte(`{"Body":"foo"}`),
						RequestHeaders: http.Header{"Content-Type": []string{"application/json"}},
					},
				}, goid)
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
					ParentTraceId:         nil,
					ParentSpanId:          nil,
					ExternalCorrelationId: nil,
					DefLoc:                &udefLoc,
					Goid:                  goid,
					Data: &tracepb2.SpanStart_Request{
						Request: &tracepb2.RequestSpanStart{
							ServiceName:      "service",
							EndpointName:     "endpoint",
							HttpMethod:       "POST",
							Path:             "/path/hello",
							PathParams:       []string{"hello"},
							RequestHeaders:   map[string]string{"Content-Type": "application/json"},
							RequestPayload:   []byte(`{"Body":"foo"}`),
							ExtCorrelationId: nil,
							Uid:              ptr("userid"),
						},
					},
				}},
			},
		},

		{
			Name: "RequestSpanEnd",
			Emit: func(l *trace2.Log) {
				l.RequestSpanEnd(trace2.RequestSpanEndParams{
					EventParams: ep,
					Req: &model.Request{
						RPCData: &model.RPCData{
							Desc: &model.RPCDesc{
								Service:  "service",
								Endpoint: "endpoint",
							},
						},
					},
					Resp: &model.Response{
						HTTPStatus:         123,
						Err:                err,
						Payload:            []byte("payload"),
						RawResponseHeaders: map[string][]string{"Content-Type": {"application/json"}},
					},
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{
					Error: pbErr,
					Data: &tracepb2.SpanEnd_Request{
						Request: &tracepb2.RequestSpanEnd{
							ServiceName:     "service",
							EndpointName:    "endpoint",
							HttpStatusCode:  123,
							ResponseHeaders: map[string]string{"Content-Type": "application/json"},
							ResponsePayload: []byte("payload"),
						},
					},
				}},
			},
		},

		{
			Name: "AuthSpanStart",
			Emit: func(l *trace2.Log) {
				l.AuthSpanStart(&model.Request{
					Type:         model.AuthHandler,
					TraceID:      traceID,
					SpanID:       spanID,
					ParentSpanID: model.SpanID{},
					Start:        now,
					Traced:       true,
					DefLoc:       defLoc,
					RPCData: &model.RPCData{
						Desc: &model.RPCDesc{
							Service:  "service",
							Endpoint: "endpoint",
							Raw:      false,
						},
						NonRawPayload: []byte(`{"Body":"foo"}`),
					},
				}, goid)
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
					ParentTraceId:         nil,
					ParentSpanId:          nil,
					ExternalCorrelationId: nil,
					DefLoc:                &udefLoc,
					Goid:                  goid,
					Data: &tracepb2.SpanStart_Auth{
						Auth: &tracepb2.AuthSpanStart{
							ServiceName:  "service",
							EndpointName: "endpoint",
							AuthPayload:  []byte(`{"Body":"foo"}`),
						},
					},
				}},
			},
		},

		{
			Name: "AuthSpanEnd",
			Emit: func(l *trace2.Log) {
				l.AuthSpanEnd(trace2.AuthSpanEndParams{
					EventParams: ep,
					Req: &model.Request{
						RPCData: &model.RPCData{
							Desc: &model.RPCDesc{
								Service:  "service",
								Endpoint: "endpoint",
							},
						},
					},
					Resp: &model.Response{
						HTTPStatus: 123,
						AuthUID:    "userid",
						Err:        err,
						Payload:    []byte("payload"),
					},
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{
					Error: pbErr,
					Data: &tracepb2.SpanEnd_Auth{
						Auth: &tracepb2.AuthSpanEnd{
							ServiceName:  "service",
							EndpointName: "endpoint",
							Uid:          "userid",
							UserData:     []byte("payload"),
						},
					},
				}},
			},
		},

		{
			Name: "PubsubMessageSpanStart",
			Emit: func(l *trace2.Log) {
				l.PubsubMessageSpanStart(&model.Request{
					Type:         model.PubSubMessage,
					TraceID:      traceID,
					SpanID:       spanID,
					ParentSpanID: model.SpanID{},
					Start:        now,
					Traced:       true,
					DefLoc:       defLoc,
					MsgData: &model.PubSubMsgData{
						Service:      "service",
						Topic:        "topic",
						Subscription: "subscription",
						MessageID:    "message-id",
						Attempt:      3,
						Published:    now,
						Payload:      []byte("payload"),
					},
				}, goid)
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanStart{SpanStart: &tracepb2.SpanStart{
					ParentTraceId:         nil,
					ParentSpanId:          nil,
					ExternalCorrelationId: nil,
					DefLoc:                &udefLoc,
					Goid:                  goid,
					Data: &tracepb2.SpanStart_PubsubMessage{
						PubsubMessage: &tracepb2.PubsubMessageSpanStart{
							ServiceName:      "service",
							TopicName:        "topic",
							SubscriptionName: "subscription",
							MessageId:        "message-id",
							Attempt:          3,
							PublishTime:      pbNow,
							MessagePayload:   []byte("payload"),
						},
					},
				}},
			},
		},

		{
			Name: "PubsubMessageSpanEnd",
			Emit: func(l *trace2.Log) {
				l.PubsubMessageSpanEnd(trace2.PubsubMessageSpanEndParams{
					EventParams: ep,
					Req: &model.Request{
						MsgData: &model.PubSubMsgData{
							Service:      "service",
							Topic:        "topic",
							Subscription: "subscription",
						},
					},
					Resp: &model.Response{Err: err},
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEnd{SpanEnd: &tracepb2.SpanEnd{
					Error: pbErr,
					Data: &tracepb2.SpanEnd_PubsubMessage{
						PubsubMessage: &tracepb2.PubsubMessageSpanEnd{
							ServiceName:      "service",
							TopicName:        "topic",
							SubscriptionName: "subscription",
						},
					},
				}},
			},
		},

		{
			Name: "RPCCallStart",
			Emit: func(l *trace2.Log) {
				l.RPCCallStart(&model.APICall{
					Source:             &model.Request{TraceID: traceID, SpanID: spanID},
					TargetServiceName:  "service",
					TargetEndpointName: "endpoint",
					DefLoc:             defLoc,
					StartEventID:       0,
				}, goid)
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:   goid,
					DefLoc: &udefLoc,
					Data: &tracepb2.SpanEvent_RpcCallStart{
						RpcCallStart: &tracepb2.RPCCallStart{
							TargetServiceName:  "service",
							TargetEndpointName: "endpoint",
							Stack:              nil,
						},
					},
				}},
			},
		},

		{
			Name: "RPCCallEnd",
			Emit: func(l *trace2.Log) {
				l.RPCCallEnd(&model.APICall{
					Source:       &model.Request{TraceID: traceID, SpanID: spanID},
					DefLoc:       defLoc,
					StartEventID: 1,
				}, goid, err)
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:               goid,
					CorrelationEventId: ptr[uint64](1),
					Data: &tracepb2.SpanEvent_RpcCallEnd{
						RpcCallEnd: &tracepb2.RPCCallEnd{
							Err: pbErr,
						},
					},
				}},
			},
		},

		{
			Name: "DBQueryStart",
			Emit: func(l *trace2.Log) {
				l.DBQueryStart(trace2.DBQueryStartParams{
					EventParams: ep,
					TxStartID:   1,
					Query:       "query",
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:               goid,
					DefLoc:             &udefLoc,
					CorrelationEventId: ptr[uint64](1),
					Data: &tracepb2.SpanEvent_DbQueryStart{
						DbQueryStart: &tracepb2.DBQueryStart{
							Query: "query",
						},
					},
				}},
			},
		},

		{
			Name: "DBQueryEnd",
			Emit: func(l *trace2.Log) {
				l.DBQueryEnd(ep, 1, err)
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:               goid,
					DefLoc:             &udefLoc,
					CorrelationEventId: ptr[uint64](1),
					Data: &tracepb2.SpanEvent_DbQueryEnd{
						DbQueryEnd: &tracepb2.DBQueryEnd{
							Err: pbErr,
						},
					},
				}},
			},
		},

		{
			Name: "DBTransactionStart",
			Emit: func(l *trace2.Log) {
				l.DBTransactionStart(ep, stack.Stack{})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:   goid,
					DefLoc: &udefLoc,
					Data: &tracepb2.SpanEvent_DbTransactionStart{
						DbTransactionStart: &tracepb2.DBTransactionStart{},
					},
				}},
			},
		},

		{
			Name: "DBTransactionEnd",
			Emit: func(l *trace2.Log) {
				l.DBTransactionEnd(trace2.DBTransactionEndParams{
					EventParams: ep,
					StartID:     1,
					Commit:      true,
					Err:         err,
					Stack:       stack.Stack{},
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:               goid,
					DefLoc:             &udefLoc,
					CorrelationEventId: ptr[uint64](1),
					Data: &tracepb2.SpanEvent_DbTransactionEnd{
						DbTransactionEnd: &tracepb2.DBTransactionEnd{
							Completion: tracepb2.DBTransactionEnd_COMMIT,
							Err:        pbErr,
							Stack:      nil,
						},
					},
				}},
			},
		},

		{
			Name: "PubsubPublishStart",
			Emit: func(l *trace2.Log) {
				l.PubsubPublishStart(trace2.PubsubPublishStartParams{
					EventParams: ep,
					Topic:       "topic",
					Message:     []byte("message"),
					Stack:       stack.Stack{},
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:   goid,
					DefLoc: &udefLoc,
					Data: &tracepb2.SpanEvent_PubsubPublishStart{
						PubsubPublishStart: &tracepb2.PubsubPublishStart{
							Topic:   "topic",
							Message: []byte("message"),
							Stack:   nil,
						},
					},
				}},
			},
		},

		{
			Name: "PubsubPublishEnd",
			Emit: func(l *trace2.Log) {
				l.PubsubPublishEnd(trace2.PubsubPublishEndParams{
					EventParams: ep,
					StartID:     1,
					MessageID:   "message-id",
					Err:         err,
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:               goid,
					DefLoc:             &udefLoc,
					CorrelationEventId: ptr[uint64](1),
					Data: &tracepb2.SpanEvent_PubsubPublishEnd{
						PubsubPublishEnd: &tracepb2.PubsubPublishEnd{
							MessageId: ptr("message-id"),
							Err:       pbErr,
						},
					},
				}},
			},
		},

		{
			Name: "ServiceInitStart",
			Emit: func(l *trace2.Log) {
				l.ServiceInitStart(trace2.ServiceInitStartParams{
					EventParams: ep,
					Service:     "service",
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:   goid,
					DefLoc: &udefLoc,
					Data: &tracepb2.SpanEvent_ServiceInitStart{
						ServiceInitStart: &tracepb2.ServiceInitStart{
							Service: "service",
						},
					},
				}},
			},
		},

		{
			Name: "ServiceInitEnd",
			Emit: func(l *trace2.Log) {
				l.ServiceInitEnd(ep, 1, err)
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:               goid,
					DefLoc:             &udefLoc,
					CorrelationEventId: ptr[uint64](1),
					Data: &tracepb2.SpanEvent_ServiceInitEnd{
						ServiceInitEnd: &tracepb2.ServiceInitEnd{
							Err: pbErr,
						},
					},
				}},
			},
		},

		{
			Name: "CacheCallStart",
			Emit: func(l *trace2.Log) {
				l.CacheCallStart(trace2.CacheCallStartParams{
					EventParams: ep,
					Operation:   "operation",
					IsWrite:     true,
					Keys:        []string{"one", "two"},
					Stack:       stack.Stack{},
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:   goid,
					DefLoc: &udefLoc,
					Data: &tracepb2.SpanEvent_CacheCallStart{
						CacheCallStart: &tracepb2.CacheCallStart{
							Operation: "operation",
							Write:     true,
							Keys:      []string{"one", "two"},
							Stack:     nil,
						},
					},
				}},
			},
		},

		{
			Name: "CacheCallEnd",
			Emit: func(l *trace2.Log) {
				l.CacheCallEnd(trace2.CacheCallEndParams{
					EventParams: ep,
					StartID:     1,
					Res:         trace2.CacheErr,
					Err:         err,
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:               goid,
					DefLoc:             &udefLoc,
					CorrelationEventId: ptr[uint64](1),
					Data: &tracepb2.SpanEvent_CacheCallEnd{
						CacheCallEnd: &tracepb2.CacheCallEnd{
							Result: tracepb2.CacheCallEnd_ERR,
							Err:    pbErr,
						},
					},
				}},
			},
		},

		{
			Name: "LogMessage",
			Emit: func(l *trace2.Log) {
				l.LogMessage(trace2.LogMessageParams{
					EventParams: ep,
					Level:       model.LevelWarn,
					Msg:         "message",
					Stack:       stack.Stack{},
					Fields: []trace2.LogField{
						{Key: "error", Value: err},
						{Key: "string", Value: "string"},
						{Key: "bool", Value: true},
						{Key: "time", Value: now},
						{Key: "duration", Value: time.Second},
						{Key: "uuid", Value: uuidVal},
						{Key: "json", Value: map[string]any{"json": true}},
						{Key: "json_err", Value: func() {}},
						{Key: "int8", Value: int8(-8)},
						{Key: "int16", Value: int16(-16)},
						{Key: "int32", Value: int32(-32)},
						{Key: "int64", Value: int64(-64)},
						{Key: "int", Value: int(-1)},
						{Key: "uint8", Value: uint8(8)},
						{Key: "uint16", Value: uint16(16)},
						{Key: "uint32", Value: uint32(32)},
						{Key: "uint64", Value: uint64(64)},
						{Key: "uint", Value: uint(1)},
						{Key: "float32", Value: float32(1.2)},
						{Key: "float64", Value: float64(3.4)},
					},
				})
			},
			Want: &tracepb2.TraceEvent{
				TraceId: pbTraceID,
				SpanId:  pbSpanID,
				Event: &tracepb2.TraceEvent_SpanEvent{SpanEvent: &tracepb2.SpanEvent{
					Goid:   goid,
					DefLoc: &udefLoc,
					Data: &tracepb2.SpanEvent_LogMessage{
						LogMessage: &tracepb2.LogMessage{
							Level: tracepb2.LogMessage_WARN,
							Msg:   "message",
							Stack: nil,
							Fields: []*tracepb2.LogField{
								{Key: "error", Value: &tracepb2.LogField_Error{Error: pbErr}},
								{Key: "string", Value: &tracepb2.LogField_Str{Str: "string"}},
								{Key: "bool", Value: &tracepb2.LogField_Bool{Bool: true}},
								{Key: "time", Value: &tracepb2.LogField_Time{Time: pbNow}},
								{Key: "duration", Value: &tracepb2.LogField_Dur{Dur: int64(time.Second)}},
								{Key: "uuid", Value: &tracepb2.LogField_Uuid{Uuid: pbUUID}},
								{Key: "json", Value: &tracepb2.LogField_Json{Json: []byte(`{"json":true}`)}},
								{Key: "json_err", Value: &tracepb2.LogField_Error{Error: &tracepb2.Error{Msg: "json: unsupported type: func()"}}},
								{Key: "int8", Value: &tracepb2.LogField_Int{Int: -8}},
								{Key: "int16", Value: &tracepb2.LogField_Int{Int: -16}},
								{Key: "int32", Value: &tracepb2.LogField_Int{Int: -32}},
								{Key: "int64", Value: &tracepb2.LogField_Int{Int: -64}},
								{Key: "int", Value: &tracepb2.LogField_Int{Int: -1}},
								{Key: "uint8", Value: &tracepb2.LogField_Uint{Uint: 8}},
								{Key: "uint16", Value: &tracepb2.LogField_Uint{Uint: 16}},
								{Key: "uint32", Value: &tracepb2.LogField_Uint{Uint: 32}},
								{Key: "uint64", Value: &tracepb2.LogField_Uint{Uint: 64}},
								{Key: "uint", Value: &tracepb2.LogField_Uint{Uint: 1}},
								{Key: "float32", Value: &tracepb2.LogField_Float32{Float32: 1.2}},
								{Key: "float64", Value: &tracepb2.LogField_Float64{Float64: 3.4}},
							},
						},
					},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			log := trace2.NewLog()
			tt.Emit(log)
			data, _ := log.GetAndClear()
			ta := trace2.NewTimeAnchor(0, now)
			got, err := ParseEvent(bufio.NewReader(bytes.NewReader(data)), ta)
			if err != nil {
				t.Fatal(err)
			}

			opt := []cmp.Option{
				protocmp.Transform(),
				protocmp.IgnoreFields(&tracepb2.TraceEvent{}, "event_time"),
				protocmp.IgnoreFields(&tracepb2.TraceEvent{}, "event_id"),
				protocmp.IgnoreMessages(&tracepb2.StackTrace{}),
			}
			if diff := cmp.Diff(tt.Want, got, opt...); diff != "" {
				t.Errorf("ParseEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func ptr[T any](val T) *T {
	return &val
}
