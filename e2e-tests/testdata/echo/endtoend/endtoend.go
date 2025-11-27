package endtoend

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"

	// "net/http"
	// "net/http/httptest"
	"os"
	"reflect"

	// "strings"
	"time"

	"encore.app/test"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"encore.dev/types/option"
	"encore.dev/types/uuid"
	"github.com/google/go-cmp/cmp"
)

var assertNumber = 0

//encore:api public method=GET path=/generated-wrappers-end-to-end-test
func GeneratedWrappersEndToEndTest(ctx context.Context) (err error) {
	rlog.Info("Starting end-to-end test of generated wrappers")
	defer func() {
		if r := recover(); r != nil {
			rlog.Error("Panic occured during end to end test", "err", r)
			err = fmt.Errorf("%v", r)
		}
	}()

	// Even on a slow machine, the client should be able to connect and run this test script in 30 seconds
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Test a simple no-op
	err = test.Noop(ctx)
	assert(err, nil, "Wanted no error from noop")

	// Test we get back the right structured error
	err = test.NoopWithError(ctx)
	assertStructuredError(err, errs.Unimplemented, "totally not implemented yet")

	// Test a simple echo
	echoRsp, err := test.SimpleBodyEcho(ctx, &test.BodyEcho{Message: "hello world"})
	assert(err, nil, "Wanted no error from simple body echo")
	assert(echoRsp.Message, "hello world", "Wanted body to be 'hello world'")

	// Check our UpdateMessage and GetMessage API's
	getRsp, err := test.GetMessage(ctx, "intra-service wrapper")
	assert(err, nil, "Wanted no error from get message")
	assert(getRsp.Message, "", "Expected no message on first request")

	err = test.UpdateMessage(ctx, "intra-service wrapper", &test.BodyEcho{Message: "updating now"})
	assert(err, nil, "Wanted no error from update message")

	getRsp, err = test.GetMessage(ctx, "intra-service wrapper")
	assert(err, nil, "Wanted no error from get message")
	assert(getRsp.Message, "updating now", "Expected data from Update request")

	// Test the rest API which uses all input types (query string, json body and header fields)
	// as well as nested structs and path segments in the URL
	restRsp, err := test.RestStyleAPI(ctx, 5, "hello", &test.RestParams{
		HeaderValue: "this is the header field",
		QueryValue:  "this is a query string field",
		BodyValue:   "this is the body field",
		Nested: struct {
			Key   string `json:"Alice"`
			Value int    `json:"bOb"`
			Ok    bool   `json:"charile"`
		}{
			Key:   "the nested key",
			Value: 8,
			Ok:    true,
		},
	})
	assert(err, nil, "Wanted no error from rest style api")
	assert(restRsp.HeaderValue, "this is the header field", "expected header value")
	assert(restRsp.QueryValue, "this is a query string field", "expected query value")
	assert(restRsp.BodyValue, "this is the body field", "expected body value")
	assert(restRsp.Nested.Key, "hello + the nested key", "expected nested key")
	assert(restRsp.Nested.Value, 5+8, "expected nested value")
	assert(restRsp.Nested.Ok, true, "expected nested ok")

	// Full marshalling test with randomised payloads
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headerBytes := make([]byte, r.Intn(128))
	queryBytes := make([]byte, r.Intn(128))
	bodyBytes := make([]byte, r.Intn(128))
	r.Read(headerBytes)
	r.Read(queryBytes)
	r.Read(bodyBytes)
	params := &test.MarshallerTest[int]{
		HeaderBoolean:   r.Float32() > 0.5,
		HeaderInt:       r.Int(),
		HeaderFloat:     r.Float64(),
		HeaderString:    "header string",
		HeaderBytes:     headerBytes,
		HeaderTime:      time.Now().Truncate(time.Second),
		HeaderJson:      json.RawMessage("{\"hello\":\"world\"}"),
		HeaderUUID:      newUUID(),
		HeaderUserID:    "432",
		HeaderOption:    option.Some("test"),
		QueryBoolean:    r.Float32() > 0.5,
		QueryInt:        r.Int(),
		QueryFloat:      r.Float64(),
		QueryString:     "query string",
		QueryBytes:      headerBytes,
		QueryTime:       time.Now().Add(time.Duration(rand.Intn(1024)) * time.Hour).Truncate(time.Second),
		QueryJson:       json.RawMessage("true"),
		QueryUUID:       newUUID(),
		QueryUserID:     "9udfa",
		QuerySlice:      []int{r.Int(), r.Int(), r.Int(), r.Int()},
		BodyBoolean:     r.Float32() > 0.5,
		BodyInt:         r.Int(),
		BodyFloat:       r.Float64(),
		BodyString:      "body string",
		BodyBytes:       bodyBytes,
		BodyTime:        time.Now().Add(time.Duration(rand.Intn(1024)) * time.Hour).Truncate(time.Second),
		BodyJson:        json.RawMessage("null"),
		BodyUUID:        newUUID(),
		BodyUserID:      "✉️",
		BodySlice:       []int{r.Int(), r.Int(), r.Int(), r.Int(), r.Int(), r.Int()},
		BodyOption:      option.Some(r.Int()),
		BodyOptionSlice: []option.Option[int]{option.Some(r.Int()), option.None[int](), option.Some(r.Int())},
	}
	mResp, err := test.MarshallerTestHandler(ctx, params)
	assert(err, nil, "Expected no error from the marshaller test")

	// We're marshalling as JSON, so we can just compare the JSON strings
	respAsJSON, err := json.Marshal(mResp)
	assert(err, nil, "unable to marshal response to JSON")
	reqAsJSON, err := json.Marshal(params)
	assert(err, nil, "unable to marshal response to JSON")
	if diff := cmp.Diff(string(respAsJSON), string(reqAsJSON)); diff != "" {
		assertNumber++
		panic(fmt.Sprintf("Assertion Failure %d: %s", assertNumber, diff))
	}

	// Test the raw endpoint (Unsupported currently in service to service calls)
	// {
	// 	req, err := http.NewRequest("PUT", "?foo=bar", strings.NewReader("this is a test body"))
	// 	assert(err, nil, "unable to create request for raw endpoint")
	// 	req.Header.Add("X-Test-Header", "test")
	//
	// 	w := httptest.NewRecorder()
	// 	err = test.RawEndpoint(w, req)
	// 	assert(err, nil, "expected no error from the raw socket")
	//
	// 	assert(w.Code, http.StatusCreated, "expected the status code to be 201")
	//
	// 	type responseType struct {
	// 		Body        string
	// 		Header      string
	// 		PathParam   string
	// 		QueryString string
	// 	}
	// 	response := &responseType{}
	//
	// 	err = json.Unmarshal(w.Body.Bytes(), response)
	// 	assert(err, nil, "expected no error when unmarshalling the response body")
	//
	// 	assert(response, &responseType{"this is a test body", "test", "hello", "bar"}, "expected the response to match")
	//
	// }

	rlog.Info("End to end wrappers test completed without error")
	return nil
}

func assert(got, want any, message string) {
	assertNumber++

	if !reflect.DeepEqual(got, want) {
		panic(fmt.Sprintf("Assertion Failure %d: %s\n\n%+v != %+v\n", assertNumber, message, got, want))
	}
}

func assertNotNil(got any, message string) {
	assertNumber++
	if got == nil {
		panic(fmt.Sprintf("Assertion Failure %d: got nil: %s", assertNumber, message))
	}
}

func assertStructuredError(err error, code errs.ErrCode, message string) {
	assertNotNil(err, "want an error")

	assertNumber++
	if apiError, ok := err.(*errs.Error); !ok {
		panic(fmt.Sprintf("Assertion Failure %d: expected *errs.Error; got %+v\n", assertNumber, reflect.TypeOf(err)))
		os.Exit(assertNumber)
	} else {
		assert(apiError.Code, code, "unexpected error code")
		assert(apiError.Message, message, "expected error message")
	}
}

func newUUID() uuid.UUID {
	id, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	return id
}
