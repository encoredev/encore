package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"echo_client/golang/client"

	"github.com/google/go-cmp/cmp"
)

var assertNumber = 1

func main() {
	// Even on a slow machine, the client should be able to connect and run this test script in 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check we where given the host:port of the running echo app
	if len(os.Args) != 2 {
		fmt.Println("Usage:", filepath.Base(os.Args[0]), "<host:port>")
		fmt.Println("Got", len(os.Args), "arguments")
		os.Exit(1)
	}

	// Create the client
	api, err := client.New(
		client.BaseURL(fmt.Sprintf("http://%s", os.Args[1])),
	)
	assert(err, nil, "Wanted no error from client creation")

	// Test a simple no-op
	err = api.Test.Noop(ctx)
	assert(err, nil, "Wanted no error from noop")

	// Test we get back the right structured error
	err = api.Test.NoopWithError(ctx)
	assertStructuredError(err, client.ErrUnimplemented, "totally not implemented yet")

	// Test a simple echo
	echoRsp, err := api.Test.SimpleBodyEcho(ctx, client.TestBodyEcho{"hello world"})
	assert(err, nil, "Wanted no error from simple body echo")
	assert(echoRsp.Message, "hello world", "Wanted body to be 'hello world'")

	// Check our UpdateMessage and GetMessage API's
	getRsp, err := api.Test.GetMessage(ctx, "go")
	assert(err, nil, "Wanted no error from get message")
	assert(getRsp.Message, "", "Expected no message on first request")

	err = api.Test.UpdateMessage(ctx, "go", client.TestBodyEcho{"updating now"})
	assert(err, nil, "Wanted no error from update message")

	getRsp, err = api.Test.GetMessage(ctx, "go")
	assert(err, nil, "Wanted no error from get message")
	assert(getRsp.Message, "updating now", "Expected data from Update request")

	// Test the rest API which uses all input types (query string, json body and header fields)
	// as well as nested structs and path segments in the URL
	restRsp, err := api.Test.RestStyleAPI(ctx, 5, "hello", client.TestRestParams{
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
	headerBytes := make([]byte, 1+r.Intn(128))
	queryBytes := make([]byte, 1+r.Intn(128))
	bodyBytes := make([]byte, 1+r.Intn(128))
	r.Read(headerBytes)
	r.Read(queryBytes)
	r.Read(bodyBytes)
	params := client.TestMarshallerTest[int]{
		HeaderBoolean: r.Float32() > 0.5,
		HeaderInt:     r.Int(),
		HeaderFloat:   r.Float64(),
		HeaderString:  "header string",
		HeaderBytes:   headerBytes,
		HeaderTime:    time.Now().Truncate(time.Second),
		HeaderJson:    json.RawMessage("{\"hello\":\"world\"}"),
		HeaderUUID:    "2553e3a4-5d9f-4716-82a2-b9bdc20a3263",
		HeaderUserID:  "432",
		QueryBoolean:  r.Float32() > 0.5,
		QueryInt:      r.Int(),
		QueryFloat:    r.Float64(),
		QueryString:   "query string",
		QueryBytes:    headerBytes,
		QueryTime:     time.Now().Add(time.Duration(rand.Intn(1024)) * time.Hour).Truncate(time.Second),
		QueryJson:     json.RawMessage("true"),
		QueryUUID:     "84b7463d-6000-4678-9d94-1d526bb5217c",
		QueryUserID:   "9udfa",
		QuerySlice:    []int{r.Int(), r.Int(), r.Int(), r.Int()},
		BodyBoolean:   r.Float32() > 0.5,
		BodyInt:       r.Int(),
		BodyFloat:     r.Float64(),
		BodyString:    "body string",
		BodyBytes:     bodyBytes,
		BodyTime:      time.Now().Add(time.Duration(rand.Intn(1024)) * time.Hour).Truncate(time.Second),
		BodyJson:      json.RawMessage("null"),
		BodyUUID:      "c227acf4-1902-4c85-8027-623d47ef4c8a",
		BodyUserID:    "✉️",
		BodySlice:     []int{r.Int(), r.Int(), r.Int(), r.Int(), r.Int(), r.Int()},
	}
	mResp, err := api.Test.MarshallerTestHandler(ctx, params)
	assert(err, nil, "Expected no error from the marshaller test")

	// We're marshalling as JSON, so we can just compare the JSON strings
	respAsJSON, err := json.Marshal(mResp)
	assert(err, nil, "unable to marshal response to JSON")
	reqAsJSON, err := json.Marshal(params)
	assert(err, nil, "unable to marshal response to JSON")
	if diff := cmp.Diff(string(respAsJSON), string(reqAsJSON)); diff != "" {
		assertNumber++
		fmt.Printf("Assertion Failure %d: %s\n", assertNumber, diff)
		os.Exit(assertNumber)
	}
	assert(string(respAsJSON), string(reqAsJSON), "Expected the same response from the marshaller test")

	// Test auth handlers
	_, err = api.Test.TestAuthHandler(ctx)
	assertStructuredError(err, client.ErrUnauthenticated, "missing auth param")

	// Test with static auth data
	{
		api, err := client.New(
			client.BaseURL(fmt.Sprintf("http://%s", os.Args[1])),
			client.WithAuth(client.EchoAuthParams{
				Authorization: "Bearer tokendata",
			}),
		)
		assert(err, nil, "Wanted no error from client creation")

		resp, err := api.Test.TestAuthHandler(ctx)
		assert(err, nil, "Expected no error from second auth")
		assert(resp.Message, "user::true", "expected the user ID back")
	}

	// Test with auth data generator function
	{
		tokenToReturn := "tokendata"
		api, err := client.New(
			client.BaseURL(fmt.Sprintf("http://%s", os.Args[1])),
			client.WithAuthFunc(func(ctx context.Context) (client.EchoAuthParams, error) {
				return client.EchoAuthParams{
					Authorization: "Bearer " + tokenToReturn,
				}, nil
			}),
		)
		assert(err, nil, "Wanted no error from client creation")

		// With a valid token
		resp, err := api.Test.TestAuthHandler(ctx)
		assert(err, nil, "Expected no error from second auth")
		assert(resp.Message, "user::true", "expected the user ID back")

		// With an invalid token
		tokenToReturn = "invalid-token-value"
		_, err = api.Test.TestAuthHandler(ctx)
		assertStructuredError(err, client.ErrUnauthenticated, "invalid token")
	}

	// Test with headers and query string auth data
	{
		api, err := client.New(
			client.BaseURL(fmt.Sprintf("http://%s", os.Args[1])),
			client.WithAuth(client.EchoAuthParams{
				NewAuth: true,
				Header:  "102",
				Query:   []int{42, 100, -50, 10},
			}),
		)
		assert(err, nil, "Wanted no error from client creation")

		resp, err := api.Test.TestAuthHandler(ctx)
		assert(err, nil, "Expected no error from second auth")
		assert(resp.Message, "second_user::true", "expected the user ID back")
	}

	// Test the raw endpoint
	{
		api, err := client.New(
			client.BaseURL(fmt.Sprintf("http://%s", os.Args[1])),
			client.WithAuth(client.EchoAuthParams{
				Authorization: "Bearer tokendata",
			}),
		)
		assert(err, nil, "Wanted no error from client creation")

		req, err := http.NewRequest("PUT", "?foo=bar", strings.NewReader("this is a test body"))
		assert(err, nil, "unable to create request for raw endpoint")
		req.Header.Add("X-Test-Header", "test")

		rsp, err := api.Test.RawEndpoint(ctx, []string{"hello"}, req)
		assert(err, nil, "expected no error from the raw socket")
		defer rsp.Body.Close()

		assert(rsp.StatusCode, http.StatusCreated, "expected the status code to be 201")

		type responseType struct {
			Body        string
			Header      string
			PathParam   string
			QueryString string
		}
		response := &responseType{}

		bytes, err := io.ReadAll(rsp.Body)
		assert(err, nil, "expected no error from reading the response body")

		err = json.Unmarshal(bytes, response)
		assert(err, nil, "expected no error when unmarshalling the response body")

		assert(response, &responseType{"this is a test body", "test", "hello", "bar"}, "expected the response to match")
	}

	{
		bodyStr := "test body"
		req, err := http.NewRequest("GET", "?foo=bar", strings.NewReader(bodyStr))
		assert(err, nil, "expected no error creating request")
		resp, err := api.Di.Three(ctx, req)
		assert(err, nil, "expected no error from DI raw endpoint")
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		assert(string(body), bodyStr, "expected response body to echo incoming request body")

	}

	// Test path encoding
	resp, err := api.Test.PathMultiSegments(ctx, true, 342, "foo/blah/should/get/escaped", "503f4487-1e15-4c37-9a80-7b70f86387bb", []string{"foo/bar", "blah", "seperate/segments = great success"})
	assert(err, nil, "expected no error from the path multi segments endpoint")
	assert(resp.Boolean, true, "expected the boolean to be true")
	assert(resp.Int, 342, "expected the int to be 342")
	assert(resp.String, "foo/blah/should/get/escaped", "invalid string field returned")
	assert(resp.UUID, "503f4487-1e15-4c37-9a80-7b70f86387bb", "invalid UUID returned")
	assert(resp.Wildcard, "foo/bar/blah/seperate/segments = great success", "invalid wildcard field returned")

	// Test validation
	err = api.Validation.TestOne(ctx, client.ValidationRequest{Msg: "pass"})
	assert(err, nil, "expected no error from validation")
	err = api.Validation.TestOne(ctx, client.ValidationRequest{Msg: "fail"})
	assertStructuredError(err, client.ErrInvalidArgument, "validation failed: bad message")
	{
		api, err := client.New(
			client.BaseURL(fmt.Sprintf("http://%s", os.Args[1])),
			client.WithAuth(client.EchoAuthParams{
				Header: "fail-validation",
			}),
		)
		assert(err, nil, "expected no error from client init")
		err = api.Test.Noop(ctx)
		assertStructuredError(err, client.ErrInvalidArgument, "validation failed: auth validation fail")
	}

	// Test middleware
	{
		err = api.Middleware.Error(ctx)
		assertStructuredError(err, client.ErrInternal, "middleware error")
		resp, err := api.Middleware.ResponseRewrite(ctx, client.MiddlewarePayload{Msg: "foo"})
		assert(err, nil, "expected no error")
		assert(resp.Msg, "middleware(req=foo, resp=handler(foo))", "unexpected response")

		resp, err = api.Middleware.ResponseGen(ctx, client.MiddlewarePayload{Msg: "foo"})
		assert(resp.Msg, "middleware generated", "unexpected response")
	}

	// Client test completed
	os.Exit(0)
}

func assert(got, want any, message string) {
	assertNumber++

	if !reflect.DeepEqual(got, want) {
		fmt.Printf("Assertion Failure %d: %s\n\n%+v != %+v\n", assertNumber, message, got, want)
		os.Exit(assertNumber)
	}
}

func assertNotNil(got any, message string) {
	assertNumber++
	if got == nil {
		fmt.Printf("Assertion Failure %d: got nil: %s", assertNumber, message)
		os.Exit(assertNumber)
	}
}

func assertStructuredError(err error, code client.ErrCode, message string) {
	assertNotNil(err, "want an error")

	assertNumber++
	if apiError, ok := err.(*client.APIError); !ok {
		fmt.Printf("Assertion Failure %d: expected *client.APIError; got %+v\n", assertNumber, reflect.TypeOf(err))
		os.Exit(assertNumber)
	} else {
		assert(apiError.Code, code, "unexpected error code")
		assert(apiError.Message, message, "expected error message")
	}
}
