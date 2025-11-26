package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/apisdk/api"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/health"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/testsupport"
	"encore.dev/appruntime/shared/traceprovider"
	"encore.dev/appruntime/shared/traceprovider/mock_trace"
	"encore.dev/beta/errs"
	usermetrics "encore.dev/metrics"
	"encore.dev/middleware"
	"encore.dev/pubsub"
)

type mockReq struct {
	Body string
}

type mockResp struct {
	Message string
}

func TestDesc_EndToEnd(t *testing.T) {
	server, _, metricsRegistry := testServer(t, clock.New(), false)

	tests := []struct {
		name        string
		access      api.Access
		reqBody     string
		respBody    string
		status      int
		respHeaders http.Header
	}{
		{
			name:        "echo",
			access:      api.Public,
			reqBody:     `{"Body": "foo"}`,
			respBody:    `{"Message":"foo"}`,
			status:      200,
			respHeaders: http.Header{"Content-Type": []string{"application/json"}},
		},
		{
			name:        "invalid",
			access:      api.Public,
			reqBody:     `invalid json`,
			respBody:    ``,
			status:      400,
			respHeaders: http.Header{"Content-Type": []string{"application/json"}},
		},
		{
			name:        "unauthenticated",
			access:      api.RequiresAuth,
			reqBody:     `{}`,
			respBody:    ``,
			status:      401,
			respHeaders: http.Header{"Content-Type": []string{"application/json"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", strings.NewReader(test.reqBody))
			ps := api.UnnamedParams{"value"}
			desc := newMockAPIDesc(test.access)
			desc.Handle(server.NewIncomingContext(w, req, ps, api.CallMeta{}))
			if w.Code != test.status {
				t.Errorf("got code %d, want %d", w.Code, test.status)
				return
			}
			if test.respBody != "" {
				if got := w.Body.String(); got != test.respBody {
					t.Errorf("got body %q, want %q", got, test.respBody)
				}
			}
			if test.respHeaders != nil {
				for key, val := range test.respHeaders {
					if diff := cmp.Diff(val, w.Header()[key]); diff != "" {
						t.Errorf("header %s: unexpected response header value (-want +got):\n%s", key, diff)
					}
				}
			}

		})
	}

	collected := metricsRegistry.Collect()
	if len(collected) != 2 {
		t.Fatalf("got %d metrics, want 2", len(collected))
	}

	okLabels := []usermetrics.KeyValue{
		{
			Key:   "endpoint",
			Value: "endpoint",
		},
		{
			Key:   "code",
			Value: "ok",
		},
	}
	requestsTotalOk := findMetric(collected, "e_requests_total", okLabels)
	if requestsTotalOk == nil {
		t.Log(`e_requests_total{endpoint="endpoint",code="ok"} metric not found`)
		t.FailNow()
	}

	if _, ok := requestsTotalOk.Val.([]uint64); !ok {
		t.Log(`expected e_requests_total{endpoint="endpoint",code="ok"} value to be []uint64`)
		t.FailNow()
	}

	invalidArgLabels := []usermetrics.KeyValue{
		{
			Key:   "endpoint",
			Value: "endpoint",
		},
		{
			Key:   "code",
			Value: errs.InvalidArgument.String(),
		},
	}
	requestsTotalInvalidArg := findMetric(collected, "e_requests_total", invalidArgLabels)
	if requestsTotalInvalidArg == nil {
		t.Log(`e_requests_total{endpoint="endpoint",code="invalid_argument"} metric not found`)
		t.FailNow()
	}

	if _, ok := requestsTotalInvalidArg.Val.([]uint64); !ok {
		t.Log(`expected e_requests_total{endpoint="endpoint",code="invalid_argument"} value to be []uint64`)
		t.FailNow()
	}
}

func findMetric(collected []usermetrics.CollectedMetric, name string, labels []usermetrics.KeyValue) *usermetrics.CollectedMetric {
	for _, metric := range collected {
		if metric.Info.Name() == name &&
			reflect.DeepEqual(metric.Labels, labels) {
			return &metric
		}
	}
	return nil
}

func TestDescGeneratesTrace(t *testing.T) {
	model.EnableTestMode(t)
	klock := clock.NewMock()
	klock.Set(time.Now())

	tests := []struct {
		name       string
		access     api.Access
		raw        bool
		reqBody    string
		reqHeaders http.Header
		want       *model.Request
	}{
		{
			name:       "echo",
			access:     api.Public,
			reqBody:    `{"Body": "foo"}`,
			reqHeaders: http.Header{"Content-Type": []string{"application/json"}},
			want: &model.Request{
				Type:         model.RPCCall,
				TraceID:      model.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				SpanID:       model.SpanID{0, 0, 0, 0, 0, 0, 0, 1},
				ParentSpanID: model.SpanID{},
				Start:        klock.Now(),
				Traced:       true,
				RPCData: &model.RPCData{
					Desc: &model.RPCDesc{
						Service:      "service",
						Endpoint:     "endpoint",
						Raw:          false,
						RequestType:  reflect.TypeOf(&mockReq{}),
						ResponseType: reflect.TypeOf(&mockResp{}),
						Exposed:      true,
						AuthRequired: false,
					},
					HTTPMethod:     "POST",
					Path:           "/path/hello",
					PathParams:     model.PathParams{{Name: "one", Value: "hello"}},
					UserID:         "",
					AuthData:       nil,
					TypedPayload:   &mockReq{Body: "foo"},
					NonRawPayload:  []byte(`{"Body":"foo"}`),
					RequestHeaders: http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
		{
			name:    "invalid",
			access:  api.Public,
			reqBody: `invalid json`,
			want: &model.Request{
				Type:         model.RPCCall,
				TraceID:      model.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				SpanID:       model.SpanID{0, 0, 0, 0, 0, 0, 0, 1},
				ParentSpanID: model.SpanID{},
				Start:        klock.Now(),
				Traced:       true,
				RPCData: &model.RPCData{
					Desc: &model.RPCDesc{
						Service:      "service",
						Endpoint:     "endpoint",
						Raw:          false,
						RequestType:  reflect.TypeOf(&mockReq{}),
						ResponseType: reflect.TypeOf(&mockResp{}),
						Exposed:      true,
						AuthRequired: false,
					},
					HTTPMethod:     "POST",
					Path:           "/path/hello",
					PathParams:     model.PathParams{{Name: "one", Value: "hello"}},
					UserID:         "",
					AuthData:       nil,
					TypedPayload:   nil,
					RequestHeaders: nil,
				},
			},
		},
		{
			name:    "unauthenticated",
			access:  api.RequiresAuth,
			reqBody: `{}`,
			want:    nil,
		},
		{
			name:       "raw",
			access:     api.Public,
			raw:        true,
			reqBody:    `{}`,
			reqHeaders: http.Header{"Content-Type": []string{"application/json"}},
			want: &model.Request{
				Type:         model.RPCCall,
				TraceID:      model.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				SpanID:       model.SpanID{0, 0, 0, 0, 0, 0, 0, 1},
				ParentSpanID: model.SpanID{},
				Start:        klock.Now(),
				Traced:       true,
				RPCData: &model.RPCData{
					Desc: &model.RPCDesc{
						Service:      "service",
						Endpoint:     "raw",
						Raw:          true,
						RequestType:  reflect.TypeOf(&rawMockReq{}),
						ResponseType: nil,
						Exposed:      true,
						AuthRequired: false,
					},
					HTTPMethod:     "POST",
					Path:           "/path/hello",
					PathParams:     model.PathParams{{Name: "one", Value: "hello"}},
					UserID:         "",
					AuthData:       nil,
					TypedPayload:   nil,
					RequestHeaders: http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	opts := []cmp.Option{
		cmpopts.IgnoreFields(model.Request{}, "Logger"),
		cmp.Comparer(func(a, b reflect.Type) bool { return a == b }),
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, traceMock, _ := testServer(t, klock, true)

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/path/hello", strings.NewReader(test.reqBody))
			req.Header = test.reqHeaders
			ps := api.UnnamedParams{"hello"}

			var handler api.Handler
			if test.raw {
				handler = newRawMockAPIDesc(test.access, nil)
			} else {
				handler = newMockAPIDesc(test.access)
			}

			var (
				beginReq *model.Request
			)

			traceMock.
				EXPECT().RequestSpanStart(gomock.Any(), gomock.Any()).Do(
				func(req *model.Request, _ uint32) {
					beginReq = req
				}).MaxTimes(1)

			traceMock.
				EXPECT().
				RequestSpanEnd(gomock.Any()).MaxTimes(1)

			traceMock.EXPECT().WaitAndClear().AnyTimes()
			traceMock.EXPECT().WaitUntilDone().AnyTimes()
			traceMock.EXPECT().MarkDone().MaxTimes(1)

			handler.Handle(server.NewIncomingContext(w, req, ps, api.CallMeta{}))

			if diff := cmp.Diff(test.want, beginReq, opts...); diff != "" {
				t.Errorf("beginReq mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRawEndpointOverflow tests that raw endpoint capturing
// is limited to the max capture size.
func TestRawEndpointOverflow(t *testing.T) {
	model.EnableTestMode(t)
	klock := clock.NewMock()
	klock.Set(time.Now())

	server, traceMock, _ := testServer(t, klock, true)

	var (
		reqBody           = strings.Repeat("a", 2*api.MaxRawRequestCaptureLen)
		respBody          = strings.Repeat("b", 2*api.MaxRawResponseCaptureLen)
		wantTraceReqData  = reqBody[:api.MaxRawRequestCaptureLen]
		wantTraceRespData = respBody[:api.MaxRawResponseCaptureLen]
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/path/hello", strings.NewReader(reqBody))
	ps := api.UnnamedParams{"hello"}

	handler := newRawMockAPIDesc(api.Public, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body) // consume the body
		_, _ = w.Write([]byte(respBody))
	})

	var params []trace2.BodyStreamParams

	traceMock.
		EXPECT().
		RequestSpanStart(gomock.Any(), gomock.Any()).
		MaxTimes(1)

	traceMock.
		EXPECT().
		RequestSpanEnd(gomock.Any()).
		MaxTimes(1)

	traceMock.
		EXPECT().
		BodyStream(gomock.Any()).Do(
		func(p trace2.BodyStreamParams) {
			params = append(params, p)
		}).AnyTimes()

	traceMock.EXPECT().WaitAndClear().MinTimes(1)
	traceMock.EXPECT().MarkDone().Times(1)

	handler.Handle(server.NewIncomingContext(w, req, ps, api.CallMeta{}))

	if len(params) != 2 {
		t.Fatalf("got %d BodyStream events, want %d", len(params), 2)
	}
	want := []trace2.BodyStreamParams{
		{
			EventParams: trace2.EventParams{
				TraceID: params[0].TraceID,
				SpanID:  params[0].SpanID,
			},
			IsResponse: false,
			Overflowed: true,
			Data:       []byte(wantTraceReqData),
		},
		{
			EventParams: trace2.EventParams{
				TraceID: params[0].TraceID,
				SpanID:  params[0].SpanID,
			},
			IsResponse: true,
			Overflowed: true,
			Data:       []byte(wantTraceRespData),
		},
	}
	if diff := cmp.Diff(want, params); diff != "" {
		t.Errorf("BodyStream params mismatch (-want +got):\n%s", diff)
	}
}

func testServer(t *testing.T, klock clock.Clock, mockTraces bool) (*api.Server, *mock_trace.MockLogger, *usermetrics.Registry) {
	ctrl := gomock.NewController(t)

	var tf traceprovider.Factory
	traceMock := mock_trace.NewMockLogger(ctrl)
	if mockTraces {
		tf = mock_trace.NewMockFactory(traceMock)
	} else {
		tf = &traceprovider.DefaultFactory{}
	}

	static := &config.Static{}
	runtime := &config.Runtime{}

	logger := zerolog.New(os.Stdout)
	rt := reqtrack.New(logger, nil, tf)
	metricsRegistry := usermetrics.NewRegistry(rt, len(static.BundledServices))
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	encoreMgr := encore.NewManager(static, runtime, rt)
	tsMgr := testsupport.NewManager(static, rt, logger)
	pubsubMgr := pubsub.NewManager(static, runtime, rt, tsMgr, logger, json)
	healthMgr := health.NewCheckRegistry()
	testingMgr := testsupport.NewManager(static, rt, logger)
	server := api.NewServer(static, runtime, rt, nil, encoreMgr, pubsubMgr, logger, metricsRegistry, healthMgr, testingMgr, json, klock)
	return server, traceMock, metricsRegistry
}

func newMockAPIDesc(access api.Access) *api.Desc[*mockReq, *mockResp] {
	return &api.Desc[*mockReq, *mockResp]{
		Service:        "service",
		Endpoint:       "endpoint",
		Path:           "/path/:one",
		Access:         access,
		PathParamNames: []string{"one"},
		Raw:            false,

		DecodeReq: func(req *http.Request, ps api.UnnamedParams, json jsoniter.API) (*mockReq, api.UnnamedParams, error) {
			var reqData mockReq
			if err := json.NewDecoder(req.Body).Decode(&reqData); err != nil {
				return nil, ps, err
			}
			return &reqData, ps, nil
		},
		CloneReq: func(req *mockReq) (*mockReq, error) {
			if req == nil {
				return nil, nil
			}
			clone := *req
			return &clone, nil
		},
		ReqPath: func(req *mockReq) (string, api.UnnamedParams, error) {
			return "/path/TODO", nil, nil
		},
		ReqUserPayload: func(req *mockReq) any {
			return req
		},
		AppHandler: func(ctx context.Context, req *mockReq) (*mockResp, error) {
			return &mockResp{Message: req.Body}, nil
		},
		EncodeResp: func(w http.ResponseWriter, json jsoniter.API, resp *mockResp, status int) error {
			data, err := json.Marshal(resp)
			_, _ = w.Write(data)
			return err
		},
		CloneResp: func(resp *mockResp) (*mockResp, error) {
			if resp == nil {
				return nil, nil
			}
			clone := *resp
			return &clone, nil
		},
	}
}

type rawMockReq struct{}

func newRawMockAPIDesc(access api.Access, handler http.HandlerFunc) *api.Desc[*rawMockReq, api.Void] {
	return &api.Desc[*rawMockReq, api.Void]{
		Service:        "service",
		Endpoint:       "raw",
		Path:           "/path/:one",
		Access:         access,
		PathParamNames: []string{"one"},
		Raw:            true,

		DecodeReq: func(req *http.Request, ps api.UnnamedParams, json jsoniter.API) (*rawMockReq, api.UnnamedParams, error) {
			return &rawMockReq{}, ps, nil
		},
		CloneReq: func(req *rawMockReq) (*rawMockReq, error) {
			if req == nil {
				return nil, nil
			}
			clone := *req
			return &clone, nil
		},
		ReqPath: func(req *rawMockReq) (string, api.UnnamedParams, error) {
			return "/foo", nil, nil
		},
		ReqUserPayload: func(req *rawMockReq) any {
			return nil
		},
		RawHandler: func(w http.ResponseWriter, req *http.Request) {
			if handler != nil {
				handler.ServeHTTP(w, req)
			}
		},
		EncodeResp: func(w http.ResponseWriter, json jsoniter.API, resp api.Void, status int) error {
			return nil
		},
		CloneResp: func(resp api.Void) (api.Void, error) {
			return resp, nil
		},
	}
}

// TestMiddlewareHeaders tests that middleware can set headers and they are properly
// written to the HTTP response.
func TestMiddlewareHeaders(t *testing.T) {
	model.EnableTestMode(t)

	server, _, _ := testServer(t, clock.New(), false)

	// Create a middleware that sets headers
	headerMiddleware := &api.Middleware{
		ID:      "test-middleware",
		PkgName: "test",
		Name:    "HeaderMiddleware",
		Global:  false,
		Invoke: func(req middleware.Request, next middleware.Next) middleware.Response {
			resp := next(req)

			// Set various types of headers
			resp.Header().Set("X-Custom-Header", "custom-value")
			resp.Header().Add("X-Multi-Header", "value1")
			resp.Header().Add("X-Multi-Header", "value2")
			resp.Header().Set("X-Middleware-Applied", "true")

			return resp
		},
	}

	// Create API desc with the middleware
	desc := newMockAPIDesc(api.Public)
	desc.ServiceMiddleware = []*api.Middleware{headerMiddleware}

	tests := []struct {
		name            string
		expectSuccess   bool
		expectedBody    string
		expectedHeaders map[string][]string
	}{
		{
			name:          "success_with_headers",
			expectSuccess: true,
			expectedBody:  `{"Message":"test"}`,
			expectedHeaders: map[string][]string{
				"X-Custom-Header":        {"custom-value"},
				"X-Multi-Header":         {"value1", "value2"},
				"X-Middleware-Applied":   {"true"},
				"Content-Type":           {"application/json"},
				"X-Content-Type-Options": {"nosniff"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/path/hello", strings.NewReader(`{"Body":"test"}`))
			req.Header.Set("Content-Type", "application/json")
			ps := api.UnnamedParams{"hello"}

			desc.Handle(server.NewIncomingContext(w, req, ps, api.CallMeta{}))

			// Check status code
			if test.expectSuccess && w.Code != 200 {
				t.Errorf("expected success (200), got %d", w.Code)
			}

			// Check response body
			if test.expectedBody != "" {
				if got := w.Body.String(); got != test.expectedBody {
					t.Errorf("got body %q, want %q", got, test.expectedBody)
				}
			}

			// Check headers
			for expectedHeader, expectedValues := range test.expectedHeaders {
				gotValues := w.Header().Values(expectedHeader)
				if len(gotValues) != len(expectedValues) {
					t.Errorf("header %s: got %d values %v, want %d values %v",
						expectedHeader, len(gotValues), gotValues, len(expectedValues), expectedValues)
					continue
				}
				for i, expectedValue := range expectedValues {
					if gotValues[i] != expectedValue {
						t.Errorf("header %s[%d]: got %q, want %q",
							expectedHeader, i, gotValues[i], expectedValue)
					}
				}
			}
		})
	}
}

// TestMiddlewareHeadersOnError tests that middleware headers are applied even when an error occurs.
func TestMiddlewareHeadersOnError(t *testing.T) {
	model.EnableTestMode(t)

	server, _, _ := testServer(t, clock.New(), false)

	// Create a middleware that sets headers
	headerMiddleware := &api.Middleware{
		ID:      "test-middleware",
		PkgName: "test",
		Name:    "HeaderMiddleware",
		Global:  false,
		Invoke: func(req middleware.Request, next middleware.Next) middleware.Response {
			resp := next(req)

			// Set headers regardless of success/error
			resp.Header().Set("X-Error-Header", "error-value")
			resp.Header().Set("X-Always-Present", "always")

			return resp
		},
	}

	// Create API desc with the middleware that returns an error
	desc := newMockAPIDesc(api.Public)
	desc.ServiceMiddleware = []*api.Middleware{headerMiddleware}
	desc.AppHandler = func(ctx context.Context, req *mockReq) (*mockResp, error) {
		return nil, errs.B().Code(errs.Internal).Msg("test error").Err()
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/path/hello", strings.NewReader(`{"Body":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	ps := api.UnnamedParams{"hello"}

	desc.Handle(server.NewIncomingContext(w, req, ps, api.CallMeta{}))

	// Check that error status is returned
	if w.Code != 500 {
		t.Errorf("expected error status 500, got %d", w.Code)
	}

	// Check that middleware headers are still applied
	expectedHeaders := map[string]string{
		"X-Error-Header":   "error-value",
		"X-Always-Present": "always",
	}

	for expectedHeader, expectedValue := range expectedHeaders {
		gotValue := w.Header().Get(expectedHeader)
		if gotValue != expectedValue {
			t.Errorf("header %s: got %q, want %q", expectedHeader, gotValue, expectedValue)
		}
	}
}
