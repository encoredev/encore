package run

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/tailscale/hujson"

	"encr.dev/parser/encoding"
	"encr.dev/proto/encore/parser/meta/v1"
)

type ApiCallParams struct {
	AppID         string
	Service       string
	Endpoint      string
	Path          string
	Method        string
	Payload       []byte
	AuthPayload   []byte `json:"auth_payload,omitempty"`
	AuthToken     string `json:"auth_token,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func CallAPI(ctx context.Context, run *Run, p *ApiCallParams) (map[string]any, error) {
	log := log.With().Str("app_id", p.AppID).Str("path", p.Path).Str("service", p.Service).Str("endpoint", p.Endpoint).Logger()
	if run == nil {
		log.Error().Str("app_id", p.AppID).Msg("dash: cannot make api call: app not running")
		return nil, fmt.Errorf("app not running")
	}
	proc := run.ProcGroup()
	if proc == nil {
		log.Error().Str("app_id", p.AppID).Msg("dash: cannot make api call: app not running")
		return nil, fmt.Errorf("app not running")
	}

	baseURL := "http://" + run.ListenAddr
	req, err := prepareRequest(ctx, baseURL, proc.Meta, p)
	if err != nil {
		log.Error().Err(err).Msg("dash: unable to prepare request")
		return nil, err
	}

	if p.CorrelationID != "" {
		req.Header.Set("X-Correlation-ID", p.CorrelationID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("dash: api call failed")
		return nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Encode the body back into a Go style struct
	if resp.StatusCode == http.StatusOK {
		body = handleResponse(proc.Meta, p, resp.Header, body)
	}

	log.Info().Int("status", resp.StatusCode).Msg("dash: api call completed")
	return map[string]interface{}{
		"status":      resp.Status,
		"status_code": resp.StatusCode,
		"body":        body,
		"trace_id":    resp.Header.Get("X-Encore-Trace-Id"),
	}, nil
}

// findRPC finds the RPC with the given service and endpoint name.
// If it cannot be found it reports nil.
func findRPC(md *v1.Data, service, endpoint string) *v1.RPC {
	for _, svc := range md.Svcs {
		if svc.Name == service {
			for _, rpc := range svc.Rpcs {
				if rpc.Name == endpoint {
					return rpc
				}
			}
			break
		}
	}
	return nil
}

// prepareRequest prepares a request for sending based on the given ApiCallParams.
func prepareRequest(ctx context.Context, baseURL string, md *v1.Data, p *ApiCallParams) (*http.Request, error) {
	reqSpec := newHTTPRequestSpec()
	rpc := findRPC(md, p.Service, p.Endpoint)
	if rpc == nil {
		return nil, fmt.Errorf("unknown service/endpoint: %s/%s", p.Service, p.Endpoint)
	}

	rpcEncoding, err := encoding.DescribeRPC(md, rpc, nil)
	if err != nil {
		return nil, fmt.Errorf("describe rpc: %v", err)
	}

	// Add request encoding
	{
		reqEnc := rpcEncoding.RequestEncodingForMethod(p.Method)
		if reqEnc == nil {
			return nil, fmt.Errorf("unsupported method: %s (supports: %s)", p.Method, strings.Join(rpc.HttpMethods, ","))
		}
		if len(p.Payload) > 0 {
			if err := addToRequest(reqSpec, p.Payload, reqEnc.ParameterEncodingMapByName()); err != nil {
				return nil, fmt.Errorf("encode request params: %v", err)
			}
		}
	}

	// Add auth encoding, if any
	if h := md.AuthHandler; h != nil {
		auth, err := encoding.DescribeAuth(md, h.Params, nil)
		if err != nil {
			return nil, fmt.Errorf("describe auth: %v", err)
		}
		if auth.LegacyTokenFormat {
			reqSpec.Header.Set("Authorization", "Bearer "+p.AuthToken)
		} else {
			if err := addToRequest(reqSpec, p.AuthPayload, auth.ParameterEncodingMapByName()); err != nil {
				return nil, fmt.Errorf("encode auth params: %v", err)
			}
		}
	}

	var body io.Reader = nil
	if reqSpec.Body != nil {
		data, _ := json.Marshal(reqSpec.Body)
		body = bytes.NewReader(data)
		if reqSpec.Header["Content-Type"] == nil {
			reqSpec.Header.Set("Content-Type", "application/json")
		}
	}

	reqURL := baseURL + p.Path
	if len(reqSpec.Query) > 0 {
		reqURL += "?" + reqSpec.Query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, p.Method, reqURL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range reqSpec.Header {
		req.Header[k] = v
	}
	for _, c := range reqSpec.Cookies {
		req.AddCookie(c)
	}
	return req, nil
}

func handleResponse(md *v1.Data, p *ApiCallParams, headers http.Header, body []byte) []byte {
	rpc := findRPC(md, p.Service, p.Endpoint)
	if rpc == nil {
		return body
	}

	encodingOptions := &encoding.Options{}
	rpcEncoding, err := encoding.DescribeRPC(md, rpc, encodingOptions)
	if err != nil {
		return body
	}

	decoded := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body
	}

	members := make([]hujson.ObjectMember, 0)
	if rpcEncoding.ResponseEncoding != nil {
		for i, m := range rpcEncoding.ResponseEncoding.HeaderParameters {
			value := headers.Get(m.Name)

			var beforeExtra []byte
			if i == 0 {
				beforeExtra = []byte("\n    // HTTP Headers\n    ")
			}

			members = append(members, hujson.ObjectMember{
				Name:  hujson.Value{Value: hujson.String(m.Name), BeforeExtra: beforeExtra},
				Value: hujson.Value{Value: hujson.String(value)},
			})
		}

		for i, m := range rpcEncoding.ResponseEncoding.BodyParameters {
			value, ok := decoded[m.Name]
			if !ok {
				value = []byte("null")
			}

			var beforeExtra []byte
			if i == 0 {
				if len(rpcEncoding.ResponseEncoding.HeaderParameters) > 0 {
					beforeExtra = []byte("\n\n    // JSON Payload\n    ")
				} else {
					beforeExtra = []byte("\n    ")
				}
			}

			// nosemgrep: trailofbits.go.invalid-usage-of-modified-variable.invalid-usage-of-modified-variable
			hValue, err := hujson.Parse(value)
			if err != nil {
				hValue = hujson.Value{Value: hujson.Literal(value)}
			}

			members = append(members, hujson.ObjectMember{
				Name:  hujson.Value{Value: hujson.String(m.Name), BeforeExtra: beforeExtra},
				Value: hValue,
			})
		}
	}

	value := hujson.Value{Value: &hujson.Object{Members: members}}
	value.Format()
	return value.Pack()
}

// httpRequestSpec specifies how the HTTP request should be generated.
type httpRequestSpec struct {
	// Body are the fields to encode as the JSON body.
	// If nil, no body is added.
	Body map[string]json.RawMessage

	// Header are the HTTP headers to set in the request.
	Header http.Header

	// Query are the query string fields to set.
	Query url.Values

	// Cookies are the cookies to send.
	Cookies []*http.Cookie
}

func newHTTPRequestSpec() *httpRequestSpec {
	return &httpRequestSpec{
		Body:   nil, // to distinguish between no body and "{}".
		Header: make(http.Header),
		Query:  make(url.Values),
	}
}

// addToRequest decodes rawPayload and adds it to the request according to the given parameter encodings.
// The body argument is where body parameters are added; other parameter locations are added
// directly to the request object itself.
func addToRequest(req *httpRequestSpec, rawPayload []byte, params map[string][]*encoding.ParameterEncoding) error {
	payload, err := hujson.Parse(rawPayload)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}
	vals, ok := payload.Value.(*hujson.Object)
	if !ok {
		return fmt.Errorf("invalid payload: expected JSON object, got %s", payload.Pack())
	}

	seenKeys := make(map[string]int)

	for _, kv := range vals.Members {
		lit, _ := kv.Name.Value.(hujson.Literal)
		key := lit.String()
		val := kv.Value
		val.Standardize()

		if matches := params[key]; len(matches) > 0 {
			// Get the index of this particular match, in case we have conflicts.
			idx := seenKeys[key]
			seenKeys[key]++
			if idx < len(matches) {
				param := matches[idx]
				switch param.Location {
				case encoding.Body:
					if req.Body == nil {
						req.Body = make(map[string]json.RawMessage)
					}
					req.Body[param.WireFormat] = val.Pack()

				case encoding.Query:
					switch v := val.Value.(type) {
					case hujson.Literal:
						req.Query.Add(param.WireFormat, v.String())
					case *hujson.Array:
						for _, elem := range v.Elements {
							if lit, ok := elem.Value.(hujson.Literal); ok {
								req.Query.Add(param.WireFormat, lit.String())
							} else {
								return fmt.Errorf("unsupported value type for query string array element: %T", elem.Value)
							}
						}
					default:
						return fmt.Errorf("unsupported value type for query string: %T", v)
					}

				case encoding.Header:
					switch v := val.Value.(type) {
					case hujson.Literal:
						req.Header.Add(param.WireFormat, v.String())
					default:
						return fmt.Errorf("unsupported value type for query string: %T", v)
					}

				case encoding.Cookie:
					switch v := val.Value.(type) {
					case hujson.Literal:
						// nosemgrep
						req.Cookies = append(req.Cookies, &http.Cookie{
							Name:  param.WireFormat,
							Value: v.String(),
						})
					default:
						return fmt.Errorf("unsupported value type for cookie: %T", v)
					}

				default:
					return fmt.Errorf("unsupported parameter location %v", param.Location)
				}
			}
		}
	}

	return nil
}
