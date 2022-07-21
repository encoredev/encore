package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/api"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/model"
	"encore.dev/appruntime/reqtrack"
)

type mockReq struct {
	Body   string
	Params api.PathParams
}

func (m mockReq) Serialize(json jsoniter.API) ([][]byte, error) {
	data, err := json.Marshal(m)
	return [][]byte{data}, err
}

func (m mockReq) Clone() (mockReq, error) {
	m2 := m
	m2.Params = make(api.PathParams, len(m.Params))
	copy(m2.Params, m.Params)
	return m2, nil
}

func (m mockReq) Path() (path string, params api.PathParams, err error) {
	return "/foo", nil, nil
}

type mockResp struct {
	Message string
}

func (m mockResp) Serialize(json jsoniter.API) ([][]byte, error) {
	data, err := json.Marshal(m)
	return [][]byte{data}, err
}

func (m mockResp) Clone() (mockResp, error) {
	return m, nil
}

func TestDesc_EndToEnd(t *testing.T) {
	cfg := &config.Config{
		Static:  &config.Static{},
		Runtime: &config.Runtime{},
	}
	logger := zerolog.New(os.Stdout)
	rt := reqtrack.New(logger, nil, false)
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	server := api.NewServer(cfg, rt, logger, json)

	desc := &api.Desc[mockReq, mockResp]{
		Service:  "service",
		Endpoint: "endpoint",
		Path:     "/foo",
		Access:   api.Public,

		DecodeReq: func(req *http.Request, ps httprouter.Params, json jsoniter.API) (mockReq, error) {
			body, _ := io.ReadAll(req.Body)
			return mockReq{
				Body:   string(body),
				Params: ps,
			}, nil
		},
		AppHandler: func(ctx context.Context, req mockReq) (mockResp, error) {
			return mockResp{Message: req.Body}, nil
		},
		EncodeResp: func(w http.ResponseWriter, json jsoniter.API, resp mockResp) error {
			w.Write([]byte(resp.Message))
			return nil
		},
	}

	wantBody := "Test Body"
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader(wantBody))
	ps := httprouter.Params{{Key: "key", Value: "value"}}
	desc.Handle(server.NewContext(w, req, ps, model.AuthInfo{}))
	if w.Code != 200 {
		t.Errorf("got code %d, want 200", w.Code)
	}
	if got := w.Body.String(); got != wantBody {
		t.Errorf("got body %q, want %q", got, wantBody)
	}
}
