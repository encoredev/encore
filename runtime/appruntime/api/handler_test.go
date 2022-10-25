package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/api"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metrics"
	"encore.dev/appruntime/model"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/beta/errs"
)

type mockReq struct {
	Body string
}

type mockResp struct {
	Message string
}

func newMockAPIDesc(access api.Access) *api.Desc[*mockReq, *mockResp] {
	return &api.Desc[*mockReq, *mockResp]{
		Service:  "service",
		Endpoint: "endpoint",
		Path:     "/foo",
		Access:   access,

		DecodeReq: func(req *http.Request, ps httprouter.Params, json jsoniter.API) (*mockReq, error) {
			var reqData mockReq
			if err := json.NewDecoder(req.Body).Decode(&reqData); err != nil {
				return nil, err
			}
			return &reqData, nil
		},
		CloneReq: func(req *mockReq) (*mockReq, error) {
			if req == nil {
				return nil, nil
			}
			clone := *req
			return &clone, nil
		},
		SerializeReq: func(json jsoniter.API, req *mockReq) ([][]byte, error) {
			data, _ := json.Marshal(req)
			return [][]byte{data}, nil
		},
		ReqPath: func(req *mockReq) (string, api.PathParams, error) {
			return "/foo", nil, nil
		},
		ReqUserPayload: func(req *mockReq) any {
			return req
		},
		AppHandler: func(ctx context.Context, req *mockReq) (*mockResp, error) {
			return &mockResp{Message: req.Body}, nil
		},
		EncodeResp: func(w http.ResponseWriter, json jsoniter.API, resp *mockResp) error {
			data, err := json.Marshal(resp)
			w.Write(data)
			return err
		},
		SerializeResp: func(json jsoniter.API, resp *mockResp) ([][]byte, error) {
			data, _ := json.Marshal(resp)
			return [][]byte{data}, nil
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

func TestDesc_EndToEnd(t *testing.T) {
	cfg := &config.Config{
		Static:  &config.Static{},
		Runtime: &config.Runtime{},
	}
	logger := zerolog.New(os.Stdout)
	testMetricsExporter := metrics.NewTestMetricsExporter(logger)
	metrics := metrics.NewManager(testMetricsExporter)
	rt := reqtrack.New(logger, nil, false)
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	encoreMgr := encore.NewManager(cfg, rt)
	server := api.NewServer(cfg, rt, nil, encoreMgr, logger, metrics, json)

	tests := []struct {
		name     string
		access   api.Access
		reqBody  string
		respBody string
		status   int
	}{
		{
			name:     "echo",
			access:   api.Public,
			reqBody:  `{"Body": "foo"}`,
			respBody: `{"Message":"foo"}`,
			status:   200,
		},
		{
			name:     "invalid",
			access:   api.Public,
			reqBody:  `invalid json`,
			respBody: ``,
			status:   400,
		},
		{
			name:     "unauthenticated",
			access:   api.RequiresAuth,
			reqBody:  `{}`,
			respBody: ``,
			status:   401,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", strings.NewReader(test.reqBody))
			ps := httprouter.Params{{Key: "key", Value: "value"}}
			desc := newMockAPIDesc(test.access)
			desc.Handle(server.NewIncomingContext(w, req, ps, model.AuthInfo{}))
			if w.Code != test.status {
				t.Errorf("got code %d, want %d", w.Code, test.status)
				return
			}
			if test.respBody != "" {
				if got := w.Body.String(); got != test.respBody {
					t.Errorf("got body %q, want %q", got, test.respBody)
				}
			}
		})
	}

	testMetricsExporter.AssertCounter(t, "e_requests_total", 2, map[string]string{
		"service":  "service",
		"endpoint": "endpoint",
	})
	testMetricsExporter.AssertObservation(
		t,
		"e_request_durations_milliseconds",
		"duration",
		func(value float64) bool {
			return value >= 0
		},
		map[string]string{
			"service":     "service",
			"endpoint":    "endpoint",
			"status_code": strconv.Itoa(200),
		},
	)
	testMetricsExporter.AssertCounter(t, "e_errors_total", 1, map[string]string{
		"service":  "service",
		"endpoint": "endpoint",
		"code":     errs.InvalidArgument.String(),
	})
	testMetricsExporter.AssertObservation(
		t,
		"e_request_durations_milliseconds",
		"duration",
		func(value float64) bool {
			return value >= 0
		},
		map[string]string{
			"service":     "service",
			"endpoint":    "endpoint",
			"status_code": strconv.Itoa(400),
		},
	)
}
