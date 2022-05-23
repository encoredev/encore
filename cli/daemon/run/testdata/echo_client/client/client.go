package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is an API client for the slug Encore application.
type Client struct {
	Echo EchoClient
	Test TestClient
}

// BaseURL is the base URL for calling the Encore application's API.
type BaseURL string

const Local BaseURL = "http://localhost:4000"

// Environment returns a BaseURL for calling the cloud environment with the given name.
func Environment(name string) BaseURL {
	return BaseURL(fmt.Sprintf("https://%s-slug.encr.app", name))
}

// Option allows you to customise the baseClient used by the Client
type Option = func(client *baseClient) error

// New returns a Client for calling the public and authenticated APIs of your Encore application.
// You can customize the behaviour of the client using the given Option functions, such as WithHTTPClient or WithAuthToken.
func New(target BaseURL, options ...Option) (*Client, error) {
	// Parse the base URL where the Encore application is being hosted
	baseURL, err := url.Parse(string(target))
	if err != nil {
		return nil, fmt.Errorf("unable to parse base url: %w", err)
	}

	// Create a client with sensible defaults
	base := &baseClient{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
		userAgent:  "slug-Generated-Go-Client (Encore/devel)",
	}

	// Apply any given options
	for _, option := range options {
		if err := option(base); err != nil {
			return nil, fmt.Errorf("unable to apply client option: %w", err)
		}
	}

	return &Client{
		Echo: &echoClient{base},
		Test: &testClient{base},
	}, nil
}

// WithHTTPClient can be used to configure the underlying HTTP client used when making API calls.
//
// Defaults to http.DefaultClient
func WithHTTPClient(client HTTPDoer) Option {
	return func(base *baseClient) error {
		base.httpClient = client
		return nil
	}
}

// WithAuthToken allows you to set the auth token to be used for each request
func WithAuthToken(token string) Option {
	return func(base *baseClient) error {
		base.tokenGenerator = func(_ context.Context) (string, error) {
			return token, nil
		}
		return nil
	}
}

// WithAuthFunc allows you to pass a function which is called for each request to return an access token.
func WithAuthFunc(tokenGenerator func(ctx context.Context) (string, error)) Option {
	return func(base *baseClient) error {
		base.tokenGenerator = tokenGenerator
		return nil
	}
}

type EchoAppMetadata struct {
	AppID      string
	APIBaseURL string
	EnvName    string
	EnvType    string
}

type EchoBasicData struct {
	String      string
	Uint        uint
	Int         int
	Int8        int8
	Int64       int64
	Float32     float32
	Float64     float64
	StringSlice []string
	IntSlice    []int
	Time        time.Time
}

type EchoData[K any, V any] struct {
	Key   K
	Value V
}

type EchoEmptyData struct {
	OmitEmpty EchoData[string, string] `json:"OmitEmpty,omitempty"`
	NullPtr   string
	Zero      EchoData[string, string]
}

type EchoEnvResponse struct {
	Env []string
}

type EchoHeadersData struct {
	Int    int    `header:"X-Int"`
	String string `header:"X-String"`
}

type EchoNonBasicData struct {
	HeaderString string                                  `header:"X-Header-String"` // Header
	HeaderNumber int                                     `header:"X-Header-Number"`
	Struct       EchoData[EchoData[string, string], int] // Body
	StructPtr    EchoData[int, uint16]
	StructSlice  []EchoData[string, string]
	StructMap    map[string]EchoData[string, float32]
	StructMapPtr map[string]EchoData[string, string]
	AnonStruct   struct {
		AnonBird string
	}
	NamedStruct EchoData[string, float64] `json:"formatted_nest"`
	RawStruct   json.RawMessage
	QueryString string `query:"string"` // Query
	QueryNumber int    `query:"no"`
	PathString  string `query:"-"` // Path Parameters
	PathInt     int    `query:"-"`
	PathWild    string `query:"-"`
}

// EchoClient Provides you access to call public and authenticated APIs on echo. The concrete implementation is echoClient.
// It is setup as an interface allowing you to use GoMock to create mock implementations during tests.
type EchoClient interface {
	// AppMeta returns app metadata.
	AppMeta(ctx context.Context) (EchoAppMetadata, error)

	// BasicEcho echoes back the request data.
	BasicEcho(ctx context.Context, params EchoBasicData) (EchoBasicData, error)

	// Echo echoes back the request data.
	Echo(ctx context.Context, params EchoData[string, int]) (EchoData[string, int], error)

	// EmptyEcho echoes back the request data.
	EmptyEcho(ctx context.Context, params EchoEmptyData) (EchoEmptyData, error)

	// Env returns the environment.
	Env(ctx context.Context) (EchoEnvResponse, error)

	// HeadersEcho echoes back the request headers
	HeadersEcho(ctx context.Context, params EchoHeadersData) (EchoHeadersData, error)

	// MuteEcho absorbs a request
	MuteEcho(ctx context.Context, params EchoData[string, string]) error

	// NonBasicEcho echoes back the request data.
	NonBasicEcho(ctx context.Context, pathString string, pathInt int, pathWild string, params EchoNonBasicData) (EchoNonBasicData, error)

	// Noop does nothing
	Noop(ctx context.Context) error

	// Pong returns a bird tuple
	Pong(ctx context.Context) (EchoData[string, string], error)
}

type echoClient struct {
	base *baseClient
}

var _ EchoClient = (*echoClient)(nil)

// AppMeta returns app metadata.
func (c *echoClient) AppMeta(ctx context.Context) (resp EchoAppMetadata, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/echo.AppMeta", nil, nil, &resp)
	if err != nil {
		return
	}

	return
}

// BasicEcho echoes back the request data.
func (c *echoClient) BasicEcho(ctx context.Context, params EchoBasicData) (resp EchoBasicData, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/echo.BasicEcho", nil, params, &resp)
	if err != nil {
		return
	}

	return
}

// Echo echoes back the request data.
func (c *echoClient) Echo(ctx context.Context, params EchoData[string, int]) (resp EchoData[string, int], err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/echo.Echo", nil, params, &resp)
	if err != nil {
		return
	}

	return
}

// EmptyEcho echoes back the request data.
func (c *echoClient) EmptyEcho(ctx context.Context, params EchoEmptyData) (resp EchoEmptyData, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/echo.EmptyEcho", nil, params, &resp)
	if err != nil {
		return
	}

	return
}

// Env returns the environment.
func (c *echoClient) Env(ctx context.Context) (resp EchoEnvResponse, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/echo.Env", nil, nil, &resp)
	if err != nil {
		return
	}

	return
}

// HeadersEcho echoes back the request headers
func (c *echoClient) HeadersEcho(ctx context.Context, params EchoHeadersData) (resp EchoHeadersData, err error) {
	// Convert our params into the objects we need for the request
	reqEncoder := &serde{}

	headers := http.Header{
		"x-int":    {reqEncoder.FromInt(params.Int)},
		"x-string": {params.String},
	}

	if reqEncoder.LastError != nil {
		err = fmt.Errorf("unable to marshal parameters: %w", reqEncoder.LastError)
		return
	}

	// Now make the actual call to the API
	var respHeaders http.Header
	respHeaders, err = callAPI(ctx, c.base, "POST", "/echo.HeadersEcho", headers, nil, nil)
	if err != nil {
		return
	}

	// Copy the unmarshalled response body into our response struct
	respDecoder := &serde{}

	resp.Int = respDecoder.ToInt("Int", respHeaders.Get("x-int"), false)
	resp.String = respHeaders.Get("x-string")

	if respDecoder.LastError != nil {
		err = fmt.Errorf("unable to unmarshal headers: %w", respDecoder.LastError)
		return
	}

	return
}

// MuteEcho absorbs a request
func (c *echoClient) MuteEcho(ctx context.Context, params EchoData[string, string]) error {
	// Convert our params into the objects we need for the request
	queryString := url.Values{
		"key":   {params.Key},
		"value": {params.Value},
	}

	_, err := callAPI(ctx, c.base, "GET", fmt.Sprintf("/echo.MuteEcho?%s", queryString.Encode()), nil, nil, nil)
	return err
}

// NonBasicEcho echoes back the request data.
func (c *echoClient) NonBasicEcho(ctx context.Context, pathString string, pathInt int, pathWild string, params EchoNonBasicData) (resp EchoNonBasicData, err error) {
	// Convert our params into the objects we need for the request
	reqEncoder := &serde{}

	headers := http.Header{
		"x-header-number": {reqEncoder.FromInt(params.HeaderNumber)},
		"x-header-string": {params.HeaderString},
	}

	queryString := url.Values{
		"no":     {reqEncoder.FromInt(params.QueryNumber)},
		"string": {params.QueryString},
	}

	if reqEncoder.LastError != nil {
		err = fmt.Errorf("unable to marshal parameters: %w", reqEncoder.LastError)
		return
	}

	// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
	body := struct {
		Struct       EchoData[EchoData[string, string], int] `json:"Struct"`
		StructPtr    EchoData[int, uint16]                   `json:"StructPtr"`
		StructSlice  []EchoData[string, string]              `json:"StructSlice"`
		StructMap    map[string]EchoData[string, float32]    `json:"StructMap"`
		StructMapPtr map[string]EchoData[string, string]     `json:"StructMapPtr"`
		AnonStruct   struct {
			AnonBird string
		} `json:"AnonStruct"`
		NamedStruct EchoData[string, float64] `json:"formatted_nest"`
		RawStruct   json.RawMessage           `json:"RawStruct"`
	}{
		AnonStruct:   params.AnonStruct,
		NamedStruct:  params.NamedStruct,
		RawStruct:    params.RawStruct,
		Struct:       params.Struct,
		StructMap:    params.StructMap,
		StructMapPtr: params.StructMapPtr,
		StructPtr:    params.StructPtr,
		StructSlice:  params.StructSlice,
	}

	// We only want the response body to marshal into these fields and none of the header fields,
	// so we'll construct a new struct with only those fields.
	respBody := struct {
		Struct       EchoData[EchoData[string, string], int] `json:"Struct"`
		StructPtr    EchoData[int, uint16]                   `json:"StructPtr"`
		StructSlice  []EchoData[string, string]              `json:"StructSlice"`
		StructMap    map[string]EchoData[string, float32]    `json:"StructMap"`
		StructMapPtr map[string]EchoData[string, string]     `json:"StructMapPtr"`
		AnonStruct   struct {
			AnonBird string
		} `json:"AnonStruct"`
		NamedStruct EchoData[string, float64] `json:"formatted_nest"`
		RawStruct   json.RawMessage           `json:"RawStruct"`
		QueryString string                    `json:"QueryString"`
		QueryNumber int                       `json:"QueryNumber"`
		PathString  string                    `json:"PathString"`
		PathInt     int                       `json:"PathInt"`
		PathWild    string                    `json:"PathWild"`
	}{}

	// Now make the actual call to the API
	var respHeaders http.Header
	respHeaders, err = callAPI(ctx, c.base, "POST", fmt.Sprintf("/NonBasicEcho/%s/%d/%s?%s", pathString, pathInt, pathWild, queryString.Encode()), headers, body, &respBody)
	if err != nil {
		return
	}

	// Copy the unmarshalled response body into our response struct
	respDecoder := &serde{}

	resp.HeaderString = respHeaders.Get("x-header-string")
	resp.HeaderNumber = respDecoder.ToInt("HeaderNumber", respHeaders.Get("x-header-number"), false)
	resp.Struct = respBody.Struct
	resp.StructPtr = respBody.StructPtr
	resp.StructSlice = respBody.StructSlice
	resp.StructMap = respBody.StructMap
	resp.StructMapPtr = respBody.StructMapPtr
	resp.AnonStruct = respBody.AnonStruct
	resp.NamedStruct = respBody.NamedStruct
	resp.RawStruct = respBody.RawStruct
	resp.QueryString = respBody.QueryString
	resp.QueryNumber = respBody.QueryNumber
	resp.PathString = respBody.PathString
	resp.PathInt = respBody.PathInt
	resp.PathWild = respBody.PathWild

	if respDecoder.LastError != nil {
		err = fmt.Errorf("unable to unmarshal headers: %w", respDecoder.LastError)
		return
	}

	return
}

// Noop does nothing
func (c *echoClient) Noop(ctx context.Context) error {
	_, err := callAPI(ctx, c.base, "GET", "/echo.Noop", nil, nil, nil)
	return err
}

// Pong returns a bird tuple
func (c *echoClient) Pong(ctx context.Context) (resp EchoData[string, string], err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "GET", "/echo.Pong", nil, nil, &resp)
	if err != nil {
		return
	}

	return
}

type TestBodyEcho struct {
	Message string
}

type TestMarshallerTest[A any] struct {
	HeaderBoolean bool            `header:"x-boolean"`
	HeaderInt     int             `header:"x-int"`
	HeaderFloat   float64         `header:"x-float"`
	HeaderString  string          `header:"x-string"`
	HeaderBytes   []byte          `header:"x-bytes"`
	HeaderTime    time.Time       `header:"x-time"`
	HeaderJson    json.RawMessage `header:"x-json"`
	HeaderUUID    string          `header:"x-uuid"`
	HeaderUserID  string          `header:"x-user-id"`
	QueryBoolean  bool            `qs:"boolean"`
	QueryInt      int             `qs:"int"`
	QueryFloat    float64         `qs:"float"`
	QueryString   string          `qs:"string"`
	QueryBytes    []byte          `qs:"bytes"`
	QueryTime     time.Time       `qs:"time"`
	QueryJson     json.RawMessage `qs:"json"`
	QueryUUID     string          `qs:"uuid"`
	QueryUserID   string          `qs:"user-id"`
	QuerySlice    []A             `qs:"slice"`
	BodyBoolean   bool            `json:"boolean"`
	BodyInt       int             `json:"int"`
	BodyFloat     float64         `json:"float"`
	BodyString    string          `json:"string"`
	BodyBytes     []byte          `json:"bytes"`
	BodyTime      time.Time       `json:"time"`
	BodyJson      json.RawMessage `json:"json"`
	BodyUUID      string          `json:"uuid"`
	BodyUserID    string          `json:"user-id"`
	BodySlice     []A             `json:"slice"`
}

type TestRestParams struct {
	HeaderValue string `header:"Some-Key"`
	QueryValue  string `query:"Some-Key"`
	BodyValue   string `json:"Some-Key"`
	Nested      struct {
		Key   string `json:"Alice"`
		Value int    `json:"bOb"`
		Ok    bool   `json:"charile"`
	}
}

// TestClient Provides you access to call public and authenticated APIs on test. The concrete implementation is testClient.
// It is setup as an interface allowing you to use GoMock to create mock implementations during tests.
type TestClient interface {
	// GetMessage allows us to test an API which takes no parameters,
	// but returns data. It also tests two API's on the same path with different HTTP methods
	GetMessage(ctx context.Context) (TestBodyEcho, error)

	// MarshallerTestHandler allows us to test marshalling of all the inbuilt types in all
	// the field types. It simply echos all the responses back to the client
	MarshallerTestHandler(ctx context.Context, params TestMarshallerTest[int]) (TestMarshallerTest[int], error)

	// Noop allows us to test if a simple HTTP request can be made
	Noop(ctx context.Context) error

	// NoopWithError allows us to test if the structured errors are returned
	NoopWithError(ctx context.Context) error

	// RawEndpoint allows us to test the clients' ability to send raw requests
	// under auth
	RawEndpoint(ctx context.Context, id string, request *http.Request) (*http.Response, error)

	// RestStyleAPI tests all the ways we can get data into and out of the application
	// using Encore request handlers
	RestStyleAPI(ctx context.Context, objType int, name string, params TestRestParams) (TestRestParams, error)

	// SimpleBodyEcho allows us to exercise the body marshalling from JSON
	// and being returned purely as a body
	SimpleBodyEcho(ctx context.Context, params TestBodyEcho) (TestBodyEcho, error)

	// TestAuthHandler allows us to test the clients ability to add tokens to requests
	TestAuthHandler(ctx context.Context) (TestBodyEcho, error)

	// UpdateMessage allows us to test an API which takes parameters,
	// but doesn't return anything
	UpdateMessage(ctx context.Context, params TestBodyEcho) error
}

type testClient struct {
	base *baseClient
}

var _ TestClient = (*testClient)(nil)

// GetMessage allows us to test an API which takes no parameters,
// but returns data. It also tests two API's on the same path with different HTTP methods
func (c *testClient) GetMessage(ctx context.Context) (resp TestBodyEcho, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "GET", "/last_message", nil, nil, &resp)
	if err != nil {
		return
	}

	return
}

// MarshallerTestHandler allows us to test marshalling of all the inbuilt types in all
// the field types. It simply echos all the responses back to the client
func (c *testClient) MarshallerTestHandler(ctx context.Context, params TestMarshallerTest[int]) (resp TestMarshallerTest[int], err error) {
	// Convert our params into the objects we need for the request
	reqEncoder := &serde{}

	headers := http.Header{
		"x-boolean": {reqEncoder.FromBool(params.HeaderBoolean)},
		"x-bytes":   {reqEncoder.FromBytes(params.HeaderBytes)},
		"x-float":   {reqEncoder.FromFloat64(params.HeaderFloat)},
		"x-int":     {reqEncoder.FromInt(params.HeaderInt)},
		"x-json":    {reqEncoder.FromJSON(params.HeaderJson)},
		"x-string":  {params.HeaderString},
		"x-time":    {reqEncoder.FromTime(params.HeaderTime)},
		"x-user-id": {params.HeaderUserID},
		"x-uuid":    {params.HeaderUUID},
	}

	queryString := url.Values{
		"boolean": {reqEncoder.FromBool(params.QueryBoolean)},
		"bytes":   {reqEncoder.FromBytes(params.QueryBytes)},
		"float":   {reqEncoder.FromFloat64(params.QueryFloat)},
		"int":     {reqEncoder.FromInt(params.QueryInt)},
		"json":    {reqEncoder.FromJSON(params.QueryJson)},
		"slice":   reqEncoder.FromIntList(params.QuerySlice),
		"string":  {params.QueryString},
		"time":    {reqEncoder.FromTime(params.QueryTime)},
		"user-id": {params.QueryUserID},
		"uuid":    {params.QueryUUID},
	}

	if reqEncoder.LastError != nil {
		err = fmt.Errorf("unable to marshal parameters: %w", reqEncoder.LastError)
		return
	}

	// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
	body := struct {
		BodyBoolean bool            `json:"boolean"`
		BodyInt     int             `json:"int"`
		BodyFloat   float64         `json:"float"`
		BodyString  string          `json:"string"`
		BodyBytes   []byte          `json:"bytes"`
		BodyTime    time.Time       `json:"time"`
		BodyJson    json.RawMessage `json:"json"`
		BodyUUID    string          `json:"uuid"`
		BodyUserID  string          `json:"user-id"`
		BodySlice   []int           `json:"slice"`
	}{
		BodyBoolean: params.BodyBoolean,
		BodyBytes:   params.BodyBytes,
		BodyFloat:   params.BodyFloat,
		BodyInt:     params.BodyInt,
		BodyJson:    params.BodyJson,
		BodySlice:   params.BodySlice,
		BodyString:  params.BodyString,
		BodyTime:    params.BodyTime,
		BodyUUID:    params.BodyUUID,
		BodyUserID:  params.BodyUserID,
	}

	// We only want the response body to marshal into these fields and none of the header fields,
	// so we'll construct a new struct with only those fields.
	respBody := struct {
		QueryBoolean bool            `json:"QueryBoolean"`
		QueryInt     int             `json:"QueryInt"`
		QueryFloat   float64         `json:"QueryFloat"`
		QueryString  string          `json:"QueryString"`
		QueryBytes   []byte          `json:"QueryBytes"`
		QueryTime    time.Time       `json:"QueryTime"`
		QueryJson    json.RawMessage `json:"QueryJson"`
		QueryUUID    string          `json:"QueryUUID"`
		QueryUserID  string          `json:"QueryUserID"`
		QuerySlice   []int           `json:"QuerySlice"`
		BodyBoolean  bool            `json:"boolean"`
		BodyInt      int             `json:"int"`
		BodyFloat    float64         `json:"float"`
		BodyString   string          `json:"string"`
		BodyBytes    []byte          `json:"bytes"`
		BodyTime     time.Time       `json:"time"`
		BodyJson     json.RawMessage `json:"json"`
		BodyUUID     string          `json:"uuid"`
		BodyUserID   string          `json:"user-id"`
		BodySlice    []int           `json:"slice"`
	}{}

	// Now make the actual call to the API
	var respHeaders http.Header
	respHeaders, err = callAPI(ctx, c.base, "POST", fmt.Sprintf("/test.MarshallerTestHandler?%s", queryString.Encode()), headers, body, &respBody)
	if err != nil {
		return
	}

	// Copy the unmarshalled response body into our response struct
	respDecoder := &serde{}

	resp.HeaderBoolean = respDecoder.ToBool("HeaderBoolean", respHeaders.Get("x-boolean"), false)
	resp.HeaderInt = respDecoder.ToInt("HeaderInt", respHeaders.Get("x-int"), false)
	resp.HeaderFloat = respDecoder.ToFloat64("HeaderFloat", respHeaders.Get("x-float"), false)
	resp.HeaderString = respHeaders.Get("x-string")
	resp.HeaderBytes = respDecoder.ToBytes("HeaderBytes", respHeaders.Get("x-bytes"), false)
	resp.HeaderTime = respDecoder.ToTime("HeaderTime", respHeaders.Get("x-time"), false)
	resp.HeaderJson = respDecoder.ToJSON("HeaderJson", respHeaders.Get("x-json"), false)
	resp.HeaderUUID = respHeaders.Get("x-uuid")
	resp.HeaderUserID = respHeaders.Get("x-user-id")
	resp.QueryBoolean = respBody.QueryBoolean
	resp.QueryInt = respBody.QueryInt
	resp.QueryFloat = respBody.QueryFloat
	resp.QueryString = respBody.QueryString
	resp.QueryBytes = respBody.QueryBytes
	resp.QueryTime = respBody.QueryTime
	resp.QueryJson = respBody.QueryJson
	resp.QueryUUID = respBody.QueryUUID
	resp.QueryUserID = respBody.QueryUserID
	resp.QuerySlice = respBody.QuerySlice
	resp.BodyBoolean = respBody.BodyBoolean
	resp.BodyInt = respBody.BodyInt
	resp.BodyFloat = respBody.BodyFloat
	resp.BodyString = respBody.BodyString
	resp.BodyBytes = respBody.BodyBytes
	resp.BodyTime = respBody.BodyTime
	resp.BodyJson = respBody.BodyJson
	resp.BodyUUID = respBody.BodyUUID
	resp.BodyUserID = respBody.BodyUserID
	resp.BodySlice = respBody.BodySlice

	if respDecoder.LastError != nil {
		err = fmt.Errorf("unable to unmarshal headers: %w", respDecoder.LastError)
		return
	}

	return
}

// Noop allows us to test if a simple HTTP request can be made
func (c *testClient) Noop(ctx context.Context) error {
	_, err := callAPI(ctx, c.base, "POST", "/test.Noop", nil, nil, nil)
	return err
}

// NoopWithError allows us to test if the structured errors are returned
func (c *testClient) NoopWithError(ctx context.Context) error {
	_, err := callAPI(ctx, c.base, "POST", "/test.NoopWithError", nil, nil, nil)
	return err
}

// RawEndpoint allows us to test the clients' ability to send raw requests
// under auth
func (c *testClient) RawEndpoint(ctx context.Context, id string, request *http.Request) (*http.Response, error) {
	path, err := url.Parse(fmt.Sprintf("/raw/%s", id))
	if err != nil {
		return nil, fmt.Errorf("unable to parse api url: %w", err)
	}
	request = request.WithContext(ctx)
	if request.Method == "" {
		request.Method = "PUT"
	}
	request.URL = path

	return c.base.Do(request)
}

// RestStyleAPI tests all the ways we can get data into and out of the application
// using Encore request handlers
func (c *testClient) RestStyleAPI(ctx context.Context, objType int, name string, params TestRestParams) (resp TestRestParams, err error) {
	// Convert our params into the objects we need for the request
	headers := http.Header{"some-key": {params.HeaderValue}}

	queryString := url.Values{"Some-Key": {params.QueryValue}}

	// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
	body := struct {
		BodyValue string `json:"Some-Key"`
		Nested    struct {
			Key   string `json:"Alice"`
			Value int    `json:"bOb"`
			Ok    bool   `json:"charile"`
		} `json:"Nested"`
	}{
		BodyValue: params.BodyValue,
		Nested:    params.Nested,
	}

	// We only want the response body to marshal into these fields and none of the header fields,
	// so we'll construct a new struct with only those fields.
	respBody := struct {
		QueryValue string `json:"QueryValue"`
		BodyValue  string `json:"Some-Key"`
		Nested     struct {
			Key   string `json:"Alice"`
			Value int    `json:"bOb"`
			Ok    bool   `json:"charile"`
		} `json:"Nested"`
	}{}

	// Now make the actual call to the API
	var respHeaders http.Header
	respHeaders, err = callAPI(ctx, c.base, "PUT", fmt.Sprintf("/rest/object/%d/%s?%s", objType, name, queryString.Encode()), headers, body, &respBody)
	if err != nil {
		return
	}

	// Copy the unmarshalled response body into our response struct
	resp.HeaderValue = respHeaders.Get("some-key")
	resp.QueryValue = respBody.QueryValue
	resp.BodyValue = respBody.BodyValue
	resp.Nested = respBody.Nested

	return
}

// SimpleBodyEcho allows us to exercise the body marshalling from JSON
// and being returned purely as a body
func (c *testClient) SimpleBodyEcho(ctx context.Context, params TestBodyEcho) (resp TestBodyEcho, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/test.SimpleBodyEcho", nil, params, &resp)
	if err != nil {
		return
	}

	return
}

// TestAuthHandler allows us to test the clients ability to add tokens to requests
func (c *testClient) TestAuthHandler(ctx context.Context) (resp TestBodyEcho, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/test.TestAuthHandler", nil, nil, &resp)
	if err != nil {
		return
	}

	return
}

// UpdateMessage allows us to test an API which takes parameters,
// but doesn't return anything
func (c *testClient) UpdateMessage(ctx context.Context, params TestBodyEcho) error {
	_, err := callAPI(ctx, c.base, "PUT", "/last_message", nil, params, nil)
	return err
}

// HTTPDoer is an interface which can be used to swap out the default
// HTTP client (http.DefaultClient) with your own custom implementation.
// This can be used to inject middleware or mock responses during unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// baseClient holds all the information we need to make requests to an Encore application
type baseClient struct {
	tokenGenerator func(ctx context.Context) (string, error) // The function which will add the bearer token to the requests
	httpClient     HTTPDoer                                  // The HTTP client which will be used for all API requests
	baseURL        *url.URL                                  // The base URL which API requests will be made against
	userAgent      string                                    // What user agent we will use in the API requests
}

// Do sends the req to the Encore application adding the authorization token as required.
func (b *baseClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", b.userAgent)

	// If a authorization token generator is present, call it and add the returned token to the request
	if b.tokenGenerator != nil {
		if token, err := b.tokenGenerator(req.Context()); err != nil {
			return nil, fmt.Errorf("unable to create authorization token for api request: %w", err)
		} else if token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	}

	// Merge the base URL and the API URL
	req.URL = b.baseURL.ResolveReference(req.URL)
	req.Host = req.URL.Host

	// Finally, make the request via the configured HTTP Client
	return b.httpClient.Do(req)
}

// callAPI is used by each generated API method to actually make request and decode the responses
func callAPI(ctx context.Context, client *baseClient, method, path string, headers http.Header, body, resp any) (http.Header, error) {
	// Encode the API body
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, method, path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add any headers to the request
	for header, values := range headers {
		for _, value := range values {
			req.Header.Add(header, value)
		}
	}

	// Make the request via the base client
	rawResponse, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = rawResponse.Body.Close()
	}()
	if rawResponse.StatusCode >= 400 {
		// Read the full body sent back
		body, err := ioutil.ReadAll(rawResponse.Body)
		if err != nil {
			return nil, &APIError{
				Code:    ErrUnknown,
				Message: fmt.Sprintf("got error response without readable body: %s", rawResponse.Status),
			}
		}

		// Attempt to decode the error response as a structured APIError
		apiError := &APIError{}
		if err := json.Unmarshal(body, apiError); err != nil {
			// If the error is not a parsable as an APIError, then return an error with the raw body
			return nil, &APIError{
				Code:    ErrUnknown,
				Message: fmt.Sprintf("got error response: %s", string(body)),
			}
		}
		return nil, apiError
	}

	// Decode the response
	if resp != nil {
		if err := json.NewDecoder(rawResponse.Body).Decode(resp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
	}
	return rawResponse.Header, nil
}

// APIError is the error type returned by the API
type APIError struct {
	Code    ErrCode `json:"code"`
	Message string  `json:"message"`
	Details any     `json:"details"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type ErrCode int

const (
	// ErrOK indicates the operation was successful.
	ErrOK ErrCode = 0

	// ErrCanceled indicates the operation was canceled (typically by the caller).
	//
	// Encore will generate this error code when cancellation is requested.
	ErrCanceled ErrCode = 1

	// ErrUnknown error. An example of where this error may be returned is
	// if a Status value received from another address space belongs to
	// an error-space that is not known in this address space. Also
	// errors raised by APIs that do not return enough error information
	// may be converted to this error.
	//
	// Encore will generate this error code in the above two mentioned cases.
	ErrUnknown ErrCode = 2

	// ErrInvalidArgument indicates client specified an invalid argument.
	// Note that this differs from FailedPrecondition. It indicates arguments
	// that are problematic regardless of the state of the system
	// (e.g., a malformed file name).
	//
	// This error code will not be generated by the gRPC framework.
	ErrInvalidArgument ErrCode = 3

	// ErrDeadlineExceeded means operation expired before completion.
	// For operations that change the state of the system, this error may be
	// returned even if the operation has completed successfully. For
	// example, a successful response from a server could have been delayed
	// long enough for the deadline to expire.
	//
	// The gRPC framework will generate this error code when the deadline is
	// exceeded.
	ErrDeadlineExceeded ErrCode = 4

	// ErrNotFound means some requested entity (e.g., file or directory) was
	// not found.
	//
	// This error code will not be generated by the gRPC framework.
	ErrNotFound ErrCode = 5

	// ErrAlreadyExists means an attempt to create an entity failed because one
	// already exists.
	//
	// This error code will not be generated by the gRPC framework.
	ErrAlreadyExists ErrCode = 6

	// ErrPermissionDenied indicates the caller does not have permission to
	// execute the specified operation. It must not be used for rejections
	// caused by exhausting some resource (use ResourceExhausted
	// instead for those errors). It must not be
	// used if the caller cannot be identified (use Unauthenticated
	// instead for those errors).
	//
	// This error code will not be generated by the gRPC core framework,
	// but expect authentication middleware to use it.
	ErrPermissionDenied ErrCode = 7

	// ErrResourceExhausted indicates some resource has been exhausted, perhaps
	// a per-user quota, or perhaps the entire file system is out of space.
	//
	// This error code will be generated by the gRPC framework in
	// out-of-memory and server overload situations, or when a message is
	// larger than the configured maximum size.
	ErrResourceExhausted ErrCode = 8

	// ErrFailedPrecondition indicates operation was rejected because the
	// system is not in a state required for the operation's execution.
	// For example, directory to be deleted may be non-empty, an rmdir
	// operation is applied to a non-directory, etc.
	//
	// A litmus test that may help a service implementor in deciding
	// between FailedPrecondition, Aborted, and Unavailable:
	//  (a) Use Unavailable if the client can retry just the failing call.
	//  (b) Use Aborted if the client should retry at a higher-level
	//      (e.g., restarting a read-modify-write sequence).
	//  (c) Use FailedPrecondition if the client should not retry until
	//      the system state has been explicitly fixed. E.g., if an "rmdir"
	//      fails because the directory is non-empty, FailedPrecondition
	//      should be returned since the client should not retry unless
	//      they have first fixed up the directory by deleting files from it.
	//  (d) Use FailedPrecondition if the client performs conditional
	//      REST Get/Update/Delete on a resource and the resource on the
	//      server does not match the condition. E.g., conflicting
	//      read-modify-write on the same resource.
	//
	// This error code will not be generated by the gRPC framework.
	ErrFailedPrecondition ErrCode = 9

	// ErrAborted indicates the operation was aborted, typically due to a
	// concurrency issue like sequencer check failures, transaction aborts,
	// etc.
	//
	// See litmus test above for deciding between FailedPrecondition,
	// ErrAborted, and Unavailable.
	ErrAborted ErrCode = 10

	// ErrOutOfRange means operation was attempted past the valid range.
	// E.g., seeking or reading past end of file.
	//
	// Unlike InvalidArgument, this error indicates a problem that may
	// be fixed if the system state changes. For example, a 32-bit file
	// may be rotated to a 64-bit file without error.
	//
	// There is a fair bit of overlap between FailedPrecondition and
	// ErrOutOfRange. We recommend using OutOfRange (the more specific
	// error) when it applies so that callers who are iterating through
	// a space can easily look for an OutOfRange error to detect when
	// they are done.
	//
	// This error code will not be generated by the gRPC framework.
	ErrOutOfRange ErrCode = 11

	// ErrUnimplemented indicates operation is not implemented or not
	// supported/enabled in this service.
	//
	// This is not an error, but a feature not available.
	//
	// This error code will not be generated by the gRPC framework.
	ErrUnimplemented ErrCode = 12

	// ErrInternal means some invariant expected by the underlying system has
	// been broken. This is not a per-message error, it is a global
	// conditions check.
	//
	// This error code will not be generated by the gRPC framework.
	ErrInternal ErrCode = 13

	// ErrUnavailable indicates the service is currently unavailable.
	// This is most likely a transient condition, which can be corrected by
	// retrying with a backoff.
	//
	// See litmus test above for deciding between FailedPrecondition,
	// Aborted, and Unavailable.
	ErrUnavailable ErrCode = 14

	// ErrDataLoss indicates unrecoverable data loss or corruption.
	//
	// This error code is only defined in the gRPC library, and only for
	// unrecoverable data loss (i.e., data loss resulting from errors
	// like hard disk corruption or bandwidth exceeded).
	//
	// This error code will not be generated by the gRPC framework.
	ErrDataLoss ErrCode = 15

	// ErrUnauthenticated indicates the request does not have valid
	// authentication credentials for the operation.
	//
	// The gRPC framework will generate this error code when the
	// authentication metadata is invalid or a Credentials callback fails,
	// but also expect authentication middleware to generate it.
	ErrUnauthenticated ErrCode = 16
)

// String returns the string representation of the error code
func (c ErrCode) String() string {
	switch c {
	case ErrOK:
		return "ok"
	case ErrCanceled:
		return "canceled"
	case ErrUnknown:
		return "unknown"
	case ErrInvalidArgument:
		return "invalid_argument"
	case ErrDeadlineExceeded:
		return "deadline_exceeded"
	case ErrNotFound:
		return "not_found"
	case ErrAlreadyExists:
		return "already_exists"
	case ErrPermissionDenied:
		return "permission_denied"
	case ErrResourceExhausted:
		return "resource_exhausted"
	case ErrFailedPrecondition:
		return "failed_precondition"
	case ErrAborted:
		return "aborted"
	case ErrOutOfRange:
		return "out_of_range"
	case ErrUnimplemented:
		return "unimplemented"
	case ErrInternal:
		return "internal"
	case ErrUnavailable:
		return "unavailable"
	case ErrDataLoss:
		return "data_loss"
	case ErrUnauthenticated:
		return "unauthenticated"
	default:
		return "unknown"
	}
}

// MarshalJSON converts the error code to a human-readable string
func (c ErrCode) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", c)), nil
}

// UnmarshalJSON converts the human-readable string to an error code
func (c *ErrCode) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "\"ok\"":
		*c = ErrOK
	case "\"canceled\"":
		*c = ErrCanceled
	case "\"unknown\"":
		*c = ErrUnknown
	case "\"invalid_argument\"":
		*c = ErrInvalidArgument
	case "\"deadline_exceeded\"":
		*c = ErrDeadlineExceeded
	case "\"not_found\"":
		*c = ErrNotFound
	case "\"already_exists\"":
		*c = ErrAlreadyExists
	case "\"permission_denied\"":
		*c = ErrPermissionDenied
	case "\"resource_exhausted\"":
		*c = ErrResourceExhausted
	case "\"failed_precondition\"":
		*c = ErrFailedPrecondition
	case "\"aborted\"":
		*c = ErrAborted
	case "\"out_of_range\"":
		*c = ErrOutOfRange
	case "\"unimplemented\"":
		*c = ErrUnimplemented
	case "\"internal\"":
		*c = ErrInternal
	case "\"unavailable\"":
		*c = ErrUnavailable
	case "\"data_loss\"":
		*c = ErrDataLoss
	case "\"unauthenticated\"":
		*c = ErrUnauthenticated
	default:
		*c = ErrUnknown
	}
	return nil
}

// serde is used to serialize request data into strings and deserialize response data from strings
type serde struct {
	LastError error // The last error that occurred
}

func (e *serde) FromInt(s int) (v string) {
	return strconv.FormatInt(int64(s), 10)
}

func (e *serde) ToInt(field string, s string, required bool) (v int) {
	if !required && s == "" {
		return
	}
	x, err := strconv.ParseInt(s, 10, 64)
	e.setErr("invalid parameter", field, err)
	return int(x)
}

func (e *serde) FromBool(s bool) (v string) {
	return strconv.FormatBool(s)
}

func (e *serde) FromFloat64(s float64) (v string) {
	return strconv.FormatFloat(s, uint8(0x66), -1, 64)
}

func (e *serde) FromBytes(s []byte) (v string) {
	return base64.URLEncoding.EncodeToString(s)
}

func (e *serde) FromTime(s time.Time) (v string) {
	return s.Format(time.RFC3339)
}

func (e *serde) FromJSON(s json.RawMessage) (v string) {
	return string(s)
}

func (e *serde) FromIntList(s []int) (v []string) {
	for _, x := range s {
		v = append(v, e.FromInt(x))
	}
	return v
}

func (e *serde) ToBool(field string, s string, required bool) (v bool) {
	if !required && s == "" {
		return
	}
	v, err := strconv.ParseBool(s)
	e.setErr("invalid parameter", field, err)
	return v
}

func (e *serde) ToFloat64(field string, s string, required bool) (v float64) {
	if !required && s == "" {
		return
	}
	x, err := strconv.ParseFloat(s, 64)
	e.setErr("invalid parameter", field, err)
	return x
}

func (e *serde) ToBytes(field string, s string, required bool) (v []byte) {
	if !required && s == "" {
		return
	}
	v, err := base64.URLEncoding.DecodeString(s)
	e.setErr("invalid parameter", field, err)
	return v
}

func (e *serde) ToTime(field string, s string, required bool) (v time.Time) {
	if !required && s == "" {
		return
	}
	v, err := time.Parse(time.RFC3339, s)
	e.setErr("invalid parameter", field, err)
	return v
}

func (e *serde) ToJSON(field string, s string, required bool) (v json.RawMessage) {
	if !required && s == "" {
		return
	}
	return json.RawMessage(s)
}

// setErr sets the last error within the object if one is not already set
func (e *serde) setErr(msg, field string, err error) {
	if err != nil && e.LastError == nil {
		e.LastError = fmt.Errorf("%s: %s: %w", field, msg, err)
	}
}
