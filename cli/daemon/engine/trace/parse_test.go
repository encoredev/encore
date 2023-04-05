package trace

import (
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace"
	"encore.dev/beta/errs"
)

type parseTest[T any] struct {
	name string
	val  T
	emit func(l *trace.Log, val T)
}

func (pt parseTest[T]) Name() string {
	return pt.name
}

func (pt parseTest[T]) Data() []byte {
	log := &trace.Log{}
	pt.emit(log, pt.val)
	return log.GetAndClear()
}

func TestParse(t *testing.T) {
	type reqResp struct {
		Req  *model.Request
		Resp *model.Response
	}
	tests := []interface {
		Name() string
		Data() []byte
	}{
		parseTest[*model.Request]{
			name: "basic",
			val: &model.Request{
				Type:     model.RPCCall,
				SpanID:   model.SpanID{0, 0, 0, 0, 0, 0, 0, 1},
				ParentID: model.SpanID{},
				Start:    time.Now(),
				Traced:   true,
				RPCData: &model.RPCData{
					Desc: &model.RPCDesc{
						Service:  "service",
						Endpoint: "endpoint",
						Raw:      false,
					},
					HTTPMethod:     "POST",
					Path:           "/path/hello",
					PathParams:     model.PathParams{{Name: "one", Value: "hello"}},
					UserID:         "",
					AuthData:       nil,
					NonRawPayload:  []byte(`{"Body":"foo"}`),
					RequestHeaders: http.Header{"Content-Type": []string{"application/json"}},
				},
			},
			emit: func(l *trace.Log, val *model.Request) { l.BeginRequest(val, 0) },
		},
		parseTest[reqResp]{
			name: "raw_err",
			val: reqResp{
				Req: &model.Request{
					Type:     model.RPCCall,
					SpanID:   model.SpanID{0, 0, 0, 0, 0, 0, 0, 1},
					ParentID: model.SpanID{},
					Start:    time.Now(),
					Traced:   true,
					RPCData: &model.RPCData{
						Desc: &model.RPCDesc{
							Service:  "service",
							Endpoint: "endpoint",
							Raw:      true,
						},
						HTTPMethod:     "POST",
						Path:           "/path/hello",
						PathParams:     model.PathParams{{Name: "one", Value: "hello"}},
						RequestHeaders: http.Header{"Content-Type": []string{"application/json"}},
					},
				},
				Resp: &model.Response{
					HTTPStatus:         500,
					Err:                &errs.Error{Code: errs.Unavailable},
					RawRequestPayload:  []byte("foo"),
					RawResponsePayload: []byte("bar"),
				},
			},
			emit: func(l *trace.Log, val reqResp) {
				l.BeginRequest(val.Req, 0)
				l.FinishRequest(val.Req, val.Resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name(), func(t *testing.T) {
			data := tt.Data()
			logger := zerolog.New(zerolog.NewTestWriter(t))
			_, err := Parse(&logger, ID{}, data, trace.CurrentVersion, nil)
			if err != nil {
				t.Fatalf("failed to parse trace: %v", err)
			}
		})
	}
}
