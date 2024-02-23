// Code generated by the Encore v0.0.0-develop client generator. DO NOT EDIT.

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

// PreviewEnv returns a BaseURL for calling the preview environment with the given PR number.
func PreviewEnv(pr int) BaseURL {
	return Environment(fmt.Sprintf("pr%d", pr))
}

// Option allows you to customise the baseClient used by the Client
type Option = func(client *baseClient) error

// New returns a Client for calling the public and authenticated APIs of your Encore application.
// You can customize the behaviour of the client using the given Option functions, such as WithHTTPClient or WithAuthFunc.
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
		userAgent:  "app-Generated-Go-Client (Encore/v0.0.0-develop)",
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

// WithAuthToken allows you to set an authentication token to be used for each request.
//
// This token will be sent as a Bearer token in the Authorization header.
func WithAuthToken(bearerToken string) Option {
	return func(base *baseClient) error {
		base.authGenerator = func(_ context.Context) (string, error) {
			return bearerToken, nil
		}
		return nil
	}
}

// WithAuthFunc allows you to pass a function which is called for each request to return an authentication token to be used for each request.
//
// This token will be sent as a Bearer token in the Authorization header.
func WithAuthFunc(authGenerator func(ctx context.Context) (string, error)) Option {
	return func(base *baseClient) error {
		base.authGenerator = authGenerator
		return nil
	}
}

type SvcRequest struct {
	Message string
}

// SvcClient Provides you access to call public and authenticated APIs on svc. The concrete implementation is svcClient.
// It is setup as an interface allowing you to use GoMock to create mock implementations during tests.
type SvcClient interface {
	// DummyAPI is a dummy endpoint.
	DummyAPI(ctx context.Context, params SvcRequest) error

	// Private is a basic auth endpoint.
	Private(ctx context.Context, params SvcRequest) error
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

// Private is a basic auth endpoint.
func (c *svcClient) Private(ctx context.Context, params SvcRequest) error {
	_, err := callAPI(ctx, c.base, "POST", "/svc.Private", nil, params, nil)
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
	authGenerator func(ctx context.Context) (string, error) // The function which will add the authentication data to the requests
	httpClient    HTTPDoer                                  // The HTTP client which will be used for all API requests
	baseURL       *url.URL                                  // The base URL which API requests will be made against
	userAgent     string                                    // What user agent we will use in the API requests
}

// Do sends the req to the Encore application adding the authorization token as required.
func (b *baseClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", b.userAgent)

	// If a authorization data generator is present, call it and add the returned token to the request
	if b.authGenerator != nil {
		if token, err := b.authGenerator(req.Context()); err != nil {
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
		body, err := io.ReadAll(rawResponse.Body)
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
	//
	//	(a) Use Unavailable if the client can retry just the failing call.
	//	(b) Use Aborted if the client should retry at a higher-level
	//	    (e.g., restarting a read-modify-write sequence).
	//	(c) Use FailedPrecondition if the client should not retry until
	//	    the system state has been explicitly fixed. E.g., if an "rmdir"
	//	    fails because the directory is non-empty, FailedPrecondition
	//	    should be returned since the client should not retry unless
	//	    they have first fixed up the directory by deleting files from it.
	//	(d) Use FailedPrecondition if the client performs conditional
	//	    REST Get/Update/Delete on a resource and the resource on the
	//	    server does not match the condition. E.g., conflicting
	//	    read-modify-write on the same resource.
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
