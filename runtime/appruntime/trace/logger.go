package trace

import (
	"context"
	"net/http"

	"encore.dev/appruntime/model"
)

//go:generate mockgen -source=./logger.go -destination ./mock_trace/mock_trace.go Logger

type Logger interface {
	Add(event EventType, data []byte)
	GetAndClear() []byte
	BeginRequest(req *model.Request, goid uint32)
	FinishRequest(req *model.Request, resp *model.Response)
	BeginCall(call *model.APICall, goid uint32)
	FinishCall(call *model.APICall, err error)
	BeginAuth(call *model.AuthCall, goid uint32)
	FinishAuth(call *model.AuthCall, uid model.UID, err error)
	DBQueryStart(p DBQueryStartParams)
	DBQueryEnd(queryID uint64, err error)
	DBTxStart(p DBTxStartParams)
	DBTxEnd(p DBTxEndParams)
	PublishStart(topic string, msg []byte, spanID model.SpanID, goid uint32, publishID uint64, skipFrames int)
	PublishEnd(publishID uint64, messageID string, err error)
	GoStart(spanID model.SpanID, goctr uint32)
	GoClear(spanID model.SpanID, goctr uint32)
	GoEnd(spanID model.SpanID, goctr uint32)
	ServiceInitStart(p ServiceInitStartParams)
	ServiceInitEnd(initCtr uint64, err error)
	CacheOpStart(p CacheOpStartParams)
	CacheOpEnd(p CacheOpEndParams)
	BodyStream(p BodyStreamParams)
	HTTPBeginRoundTrip(httpReq *http.Request, req *model.Request, goid uint32) (context.Context, error)
	HTTPCompleteRoundTrip(req *http.Request, resp *http.Response, err error)
}
