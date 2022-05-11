package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is an API client for the app Encore application.
type Client struct {
	Svc SvcClient
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
		userAgent:  "app-Generated-Client (Encore/devel)",
	}

	// Apply any given options
	for _, option := range options {
		if err := option(base); err != nil {
			return nil, fmt.Errorf("unable to apply client option: %w", err)
		}
	}

	return &Client{Svc: &svcClient{base}}, nil
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

type SvcFoo = int

type SvcGetRequest struct {
	Baz int `qs:"boo"`
}

type SvcRequest struct {
	Foo SvcFoo `encore:"optional" json:",omitempty"` // Foo is good
	Baz string `json:"boo"`                          // Baz is better

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

type SvcWrapper[T any] T

// SvcClient Provides you access to call public and authenticated APIs on svc. The concrete implementation is svcClient.
// It is setup as an interface allowing you to use GoMock to create mock implementations during tests.
type SvcClient interface {
	// DummyAPI is a dummy endpoint.
	DummyAPI(ctx context.Context, params SvcRequest) error
	Get(ctx context.Context, params SvcGetRequest) error
	RESTPath(ctx context.Context, a string, b int) error

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
	return callAPI(ctx, c.base, "POST", "/svc.DummyAPI", params, nil)
}

func (c *svcClient) Get(ctx context.Context, params SvcGetRequest) error {
	queryString := url.Values{"boo": []string{fmt.Sprint(params.Baz)}}
	return callAPI(ctx, c.base, "GET", fmt.Sprintf("/svc.Get?%s", queryString.Encode()), nil, nil)
}

func (c *svcClient) RESTPath(ctx context.Context, a string, b int) error {
	return callAPI(ctx, c.base, "GET", fmt.Sprintf("/path/%s/%d", a, b), nil, nil)
}

// TupleInputOutput tests the usage of generics in the client generator
// and this comment is also multiline, so multiline comments get tested as well.
func (c *svcClient) TupleInputOutput(ctx context.Context, params SvcTuple[string, SvcWrappedRequest]) (resp SvcTuple[bool, SvcFoo], err error) {
	err = callAPI(ctx, c.base, "POST", "/svc.TupleInputOutput", params, &resp)
	return resp, err
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
func callAPI(ctx context.Context, client *baseClient, method, path string, body, resp any) error {
	// Encode the API body
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, method, path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Make the request via the base client
	rawResponse, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = rawResponse.Body.Close()
	}()
	if rawResponse.StatusCode >= 400 {
		return fmt.Errorf("got error response: %s", rawResponse.Status)
	}

	// Decode the response
	if resp != nil {
		if err := json.NewDecoder(rawResponse.Body).Decode(resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
