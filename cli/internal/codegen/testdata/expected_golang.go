package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is an API client for the app Encore application.
type Client struct {
	Products ProductsClient
	Svc      SvcClient
}

// BaseURL is the base URL for calling the Encore application's API.
type BaseURL string

const Local BaseURL = "http://localhost:4000"

// Environment returns a BaseURL for calling the cloud environment with the given name.
func Environment(name string) BaseURL {
	return BaseURL(fmt.Sprintf("https://%s-app.encr.app", name))
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
		userAgent:  "app-Generated-Go-Client (Encore/devel)",
	}

	// Apply any given options
	for _, option := range options {
		if err := option(base); err != nil {
			return nil, fmt.Errorf("unable to apply client option: %w", err)
		}
	}

	return &Client{
		Products: &productsClient{base},
		Svc:      &svcClient{base},
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

type AuthenticationUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProductsCreateProductRequest struct {
	IdempotencyKey string `header:"Idempotency-Key"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
}

type ProductsProduct struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	CreatedBy   AuthenticationUser `json:"created_by"`
}

type ProductsProductListing struct {
	Products     []ProductsProduct `json:"products"`
	PreviousPage struct {
		Cursor string `json:"cursor,omitempty"`
		Exists bool   `json:"exists"`
	} `json:"previous"`
	NextPage struct {
		Cursor string `json:"cursor,omitempty"`
		Exists bool   `json:"exists"`
	} `json:"next"`
}

// ProductsClient Provides you access to call public and authenticated APIs on products. The concrete implementation is productsClient.
// It is setup as an interface allowing you to use GoMock to create mock implementations during tests.
type ProductsClient interface {
	Create(ctx context.Context, params ProductsCreateProductRequest) (ProductsProduct, error)
	List(ctx context.Context) (ProductsProductListing, error)
}

type productsClient struct {
	base *baseClient
}

var _ ProductsClient = (*productsClient)(nil)

func (c *productsClient) Create(ctx context.Context, params ProductsCreateProductRequest) (resp ProductsProduct, err error) {
	// Convert our params into the objects we need for the request
	headers := map[string][]string{"Idempotency-Key": {params.IdempotencyKey}}

	// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
	body := struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}{
		Description: params.Description,
		Name:        params.Name,
	}

	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/products.Create", headers, body, &resp)
	if err != nil {
		return
	}

	return
}

func (c *productsClient) List(ctx context.Context) (resp ProductsProductListing, err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "GET", "/products.List", nil, nil, &resp)
	if err != nil {
		return
	}

	return
}

type SvcAllInputTypes[A any] struct {
	A    time.Time `header:"X-Alice"`               // Specify this comes from a header field
	B    []int     `query:"Bob"`                    // Specify this comes from a query string
	C    bool      `json:"Charlies-Bool,omitempty"` // This can come from anywhere, but if it comes from the payload in JSON it must be called Charile
	Dave A         // This generic type complicates the whole thing 🙈
}

type SvcFoo = int

type SvcGetRequest struct {
	Baz int `qs:"boo"`
}

// HeaderOnlyStruct contains all types we support in headers
type SvcHeaderOnlyStruct struct {
	Boolean bool            `header:"x-boolean"`
	Int     int             `header:"x-int"`
	Float   float64         `header:"x-float"`
	String  string          `header:"x-string"`
	Bytes   []byte          `header:"x-bytes"`
	Time    time.Time       `header:"x-time"`
	Json    json.RawMessage `header:"x-json"`
	UUID    string          `header:"x-uuid"`
	UserID  string          `header:"x-user-id"`
}

type SvcRequest struct {
	Foo SvcFoo `encore:"optional"` // Foo is good
	Baz string `json:"boo"`        // Baz is better

	// This is a multiline
	// comment on the raw message!
	Raw json.RawMessage
}

// Tuple is a generic type which allows us to
// return two values of two different types
type SvcTuple[A any, B any] struct {
	A A
	B B
}

type SvcWrappedRequest = SvcWrapper[SvcRequest]

type SvcWrapper[T any] struct {
	Value T
}

// SvcClient Provides you access to call public and authenticated APIs on svc. The concrete implementation is svcClient.
// It is setup as an interface allowing you to use GoMock to create mock implementations during tests.
type SvcClient interface {
	// DummyAPI is a dummy endpoint.
	DummyAPI(ctx context.Context, params SvcRequest) error
	Get(ctx context.Context, params SvcGetRequest) error
	GetRequestWithAllInputTypes(ctx context.Context, params SvcAllInputTypes[int]) (SvcHeaderOnlyStruct, error)
	HeaderOnlyRequest(ctx context.Context, params SvcHeaderOnlyStruct) error
	RESTPath(ctx context.Context, a string, b int) error
	RequestWithAllInputTypes(ctx context.Context, params SvcAllInputTypes[string]) (SvcAllInputTypes[float64], error)

	// TupleInputOutput tests the usage of generics in the client generator
	// and this comment is also multiline, so multiline comments get tested as well.
	TupleInputOutput(ctx context.Context, params SvcTuple[string, SvcWrappedRequest]) (SvcTuple[bool, SvcFoo], error)
	Webhook(ctx context.Context, a string, b string, request *http.Request) (*http.Response, error)
}

type svcClient struct {
	base *baseClient
}

var _ SvcClient = (*svcClient)(nil)

// DummyAPI is a dummy endpoint.
func (c *svcClient) DummyAPI(ctx context.Context, params SvcRequest) error {
	_, err := callAPI(ctx, c.base, "POST", "/svc.DummyAPI", nil, params, nil)
	return err
}

func (c *svcClient) Get(ctx context.Context, params SvcGetRequest) error {
	// Convert our params into the objects we need for the request
	reqEncoder := &ℯ𝓃𝑐ℴ𝑟ℯMarshaller{}

	queryString := url.Values{"boo": {reqEncoder.FromInt(params.Baz)}}

	if reqEncoder.LastError != nil {
		return fmt.Errorf("unable to marshal parameters: %w", reqEncoder.LastError)
	}

	_, err := callAPI(ctx, c.base, "GET", fmt.Sprintf("/svc.Get?%s", queryString.Encode()), nil, nil, nil)
	return err
}

func (c *svcClient) GetRequestWithAllInputTypes(ctx context.Context, params SvcAllInputTypes[int]) (resp SvcHeaderOnlyStruct, err error) {
	// Convert our params into the objects we need for the request
	reqEncoder := &ℯ𝓃𝑐ℴ𝑟ℯMarshaller{}

	headers := map[string][]string{"X-Alice": {reqEncoder.FromTime(params.A)}}

	queryString := url.Values{
		"Bob":  reqEncoder.FromIntList(params.B),
		"c":    {reqEncoder.FromBool(params.C)},
		"dave": {reqEncoder.FromInt(params.Dave)},
	}

	if reqEncoder.LastError != nil {
		err = fmt.Errorf("unable to marshal parameters: %w", reqEncoder.LastError)
		return
	}

	// Now make the actual call to the API
	var respHeaders http.Header
	respHeaders, err = callAPI(ctx, c.base, "GET", fmt.Sprintf("/svc.GetRequestWithAllInputTypes?%s", queryString.Encode()), headers, nil, nil)
	if err != nil {
		return
	}

	// Copy the unmarshalled response body into our response struct
	respDecoder := &ℯ𝓃𝑐ℴ𝑟ℯMarshaller{}

	resp.Boolean = respDecoder.ToBool("Boolean", respHeaders.Get("x-boolean"), false)
	resp.Int = respDecoder.ToInt("Int", respHeaders.Get("x-int"), false)
	resp.Float = respDecoder.ToFloat64("Float", respHeaders.Get("x-float"), false)
	resp.String = respHeaders.Get("x-string")
	resp.Bytes = respDecoder.ToBytes("Bytes", respHeaders.Get("x-bytes"), false)
	resp.Time = respDecoder.ToTime("Time", respHeaders.Get("x-time"), false)
	resp.Json = respDecoder.ToJSON("Json", respHeaders.Get("x-json"), false)
	resp.UUID = respHeaders.Get("x-uuid")
	resp.UserID = respHeaders.Get("x-user-id")
	if respDecoder.LastError != nil {
		err = fmt.Errorf("unable to unmarshal headers: %w", respDecoder.LastError)
		return
	}

	return
}

func (c *svcClient) HeaderOnlyRequest(ctx context.Context, params SvcHeaderOnlyStruct) error {
	// Convert our params into the objects we need for the request
	reqEncoder := &ℯ𝓃𝑐ℴ𝑟ℯMarshaller{}

	headers := map[string][]string{
		"x-boolean": {reqEncoder.FromBool(params.Boolean)},
		"x-bytes":   {reqEncoder.FromBytes(params.Bytes)},
		"x-float":   {reqEncoder.FromFloat64(params.Float)},
		"x-int":     {reqEncoder.FromInt(params.Int)},
		"x-json":    {reqEncoder.FromJSON(params.Json)},
		"x-string":  {params.String},
		"x-time":    {reqEncoder.FromTime(params.Time)},
		"x-user-id": {params.UserID},
		"x-uuid":    {params.UUID},
	}

	if reqEncoder.LastError != nil {
		return fmt.Errorf("unable to marshal parameters: %w", reqEncoder.LastError)
	}

	_, err := callAPI(ctx, c.base, "GET", "/svc.HeaderOnlyRequest", headers, nil, nil)
	return err
}

func (c *svcClient) RESTPath(ctx context.Context, a string, b int) error {
	_, err := callAPI(ctx, c.base, "POST", fmt.Sprintf("/path/%s/%d", a, b), nil, nil, nil)
	return err
}

func (c *svcClient) RequestWithAllInputTypes(ctx context.Context, params SvcAllInputTypes[string]) (resp SvcAllInputTypes[float64], err error) {
	// Convert our params into the objects we need for the request
	reqEncoder := &ℯ𝓃𝑐ℴ𝑟ℯMarshaller{}

	headers := map[string][]string{"X-Alice": {reqEncoder.FromTime(params.A)}}

	queryString := url.Values{"Bob": reqEncoder.FromIntList(params.B)}

	if reqEncoder.LastError != nil {
		err = fmt.Errorf("unable to marshal parameters: %w", reqEncoder.LastError)
		return
	}

	// Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)
	body := struct {
		C    bool   `json:"Charlies-Bool,omitempty"`
		Dave string `json:"Dave"`
	}{
		C:    params.C,
		Dave: params.Dave,
	}

	// We only want the response body to marshal into these fields and none of the header fields,
	// so we'll construct a new struct with only those fields.
	respBody := struct {
		B    []int   `json:"B"`
		C    bool    `json:"Charlies-Bool,omitempty"`
		Dave float64 `json:"Dave"`
	}{}

	// Now make the actual call to the API
	var respHeaders http.Header
	respHeaders, err = callAPI(ctx, c.base, "POST", fmt.Sprintf("/svc.RequestWithAllInputTypes?%s", queryString.Encode()), headers, body, nil)
	if err != nil {
		return
	}

	// Copy the unmarshalled response body into our response struct
	respDecoder := &ℯ𝓃𝑐ℴ𝑟ℯMarshaller{}

	resp.A = respDecoder.ToTime("A", respHeaders.Get("X-Alice"), false)
	resp.B = respBody.B
	resp.C = respBody.C
	resp.Dave = respBody.Dave
	if respDecoder.LastError != nil {
		err = fmt.Errorf("unable to unmarshal headers: %w", respDecoder.LastError)
		return
	}

	return
}

// TupleInputOutput tests the usage of generics in the client generator
// and this comment is also multiline, so multiline comments get tested as well.
func (c *svcClient) TupleInputOutput(ctx context.Context, params SvcTuple[string, SvcWrappedRequest]) (resp SvcTuple[bool, SvcFoo], err error) {
	// Now make the actual call to the API
	_, err = callAPI(ctx, c.base, "POST", "/svc.TupleInputOutput", nil, params, &resp)
	if err != nil {
		return
	}

	return
}

func (c *svcClient) Webhook(ctx context.Context, a string, b string, request *http.Request) (*http.Response, error) {
	path, err := url.Parse(fmt.Sprintf("/webhook/%s/%s", a, b))
	if err != nil {
		return nil, fmt.Errorf("unable to parse api url: %w", err)
	}
	request = request.WithContext(ctx)
	request.URL = path

	return c.base.Do(request)
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
func callAPI(ctx context.Context, client *baseClient, method, path string, headers map[string][]string, body, resp any) (http.Header, error) {
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
		return nil, fmt.Errorf("got error response: %s", rawResponse.Status)
	}

	// Decode the response
	if resp != nil {
		if err := json.NewDecoder(rawResponse.Body).Decode(resp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
	}
	return rawResponse.Header, nil
}

// ℯ𝓃𝑐ℴ𝑟ℯMarshaller is used to marshal requests to strings and unmarshal responses from strings
type ℯ𝓃𝑐ℴ𝑟ℯMarshaller struct {
	LastError error // The last error that occurred
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) FromInt(s int) (v string) {
	return strconv.FormatInt(int64(s), 10)
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) FromTime(s time.Time) (v string) {
	return s.Format(time.RFC3339)
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) FromIntList(s []int) (v []string) {
	for _, x := range s {
		v = append(v, e.FromInt(x))
	}
	return v
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) FromBool(s bool) (v string) {
	return strconv.FormatBool(s)
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) ToBool(field string, s string, required bool) (v bool) {
	if !required && s == "" {
		return
	}
	v, err := strconv.ParseBool(s)
	e.setErr("invalid parameter", field, err)
	return v
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) ToInt(field string, s string, required bool) (v int) {
	if !required && s == "" {
		return
	}
	x, err := strconv.ParseInt(s, 10, 64)
	e.setErr("invalid parameter", field, err)
	return int(x)
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) ToFloat64(field string, s string, required bool) (v float64) {
	if !required && s == "" {
		return
	}
	x, err := strconv.ParseFloat(s, 64)
	e.setErr("invalid parameter", field, err)
	return x
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) ToBytes(field string, s string, required bool) (v []byte) {
	if !required && s == "" {
		return
	}
	v, err := base64.URLEncoding.DecodeString(s)
	e.setErr("invalid parameter", field, err)
	return v
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) ToTime(field string, s string, required bool) (v time.Time) {
	if !required && s == "" {
		return
	}
	v, err := time.Parse(time.RFC3339, s)
	e.setErr("invalid parameter", field, err)
	return v
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) ToJSON(field string, s string, required bool) (v json.RawMessage) {
	if !required && s == "" {
		return
	}
	return json.RawMessage(s)
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) FromFloat64(s float64) (v string) {
	return strconv.FormatFloat(s, uint8(0x66), -1, 64)
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) FromBytes(s []byte) (v string) {
	return base64.URLEncoding.EncodeToString(s)
}

func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) FromJSON(s json.RawMessage) (v string) {
	return string(s)
}

// setErr sets the last error within the object if one is not already set
func (e *ℯ𝓃𝑐ℴ𝑟ℯMarshaller) setErr(msg, field string, err error) {
	if err != nil && e.LastError == nil {
		e.LastError = fmt.Errorf("%s: %s: %w", field, msg, err)
	}
}
