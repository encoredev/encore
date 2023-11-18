package trace2

import (
	"context"
	"net/http"
	"time"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
)

//go:generate mockgen -source=./logger.go -package=mock_trace -destination ../../shared/traceprovider/mock_trace/mock_trace.go Logger

type Logger interface {
	MarkDone()
	Add(Event) EventID

	WaitUntilDone()
	WaitAtLeast(time.Duration) bool
	GetAndClear() (data []byte, done bool)
	WaitAndClear() (data []byte, done bool)

	RequestSpanStart(req *model.Request, goid uint32)
	RequestSpanEnd(params RequestSpanEndParams)
	AuthSpanStart(req *model.Request, goid uint32)
	AuthSpanEnd(params AuthSpanEndParams)
	PubsubMessageSpanStart(req *model.Request, goid uint32)
	PubsubMessageSpanEnd(params PubsubMessageSpanEndParams)
	RPCCallStart(call *model.APICall, goid uint32) EventID
	RPCCallEnd(call *model.APICall, goid uint32, err error)
	DBQueryStart(p DBQueryStartParams) EventID
	DBQueryEnd(EventParams, EventID, error)
	DBTransactionStart(EventParams, stack.Stack) EventID
	DBTransactionEnd(DBTransactionEndParams)
	PubsubPublishStart(PubsubPublishStartParams) EventID
	PubsubPublishEnd(PubsubPublishEndParams)
	ServiceInitStart(ServiceInitStartParams) EventID
	ServiceInitEnd(EventParams, EventID, error)
	CacheCallStart(CacheCallStartParams) EventID
	CacheCallEnd(CacheCallEndParams)
	BodyStream(BodyStreamParams)
	LogMessage(LogMessageParams)
	HTTPBeginRoundTrip(httpReq *http.Request, req *model.Request, goid uint32) (context.Context, error)
	HTTPCompleteRoundTrip(req *http.Request, resp *http.Response, goid uint32, err error)
}
