package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"

	"encr.dev/internal/conf"
)

type CreateOAuthSessionParams struct {
	Challenge   string `json:"challenge"`
	State       string `json:"state"`
	RedirectURL string `json:"redirect_url"`
}

func CreateOAuthSession(ctx context.Context, p *CreateOAuthSessionParams) (authURL string, err error) {
	var resp struct {
		AuthURL string `json:"auth_url"`
	}
	err = call(ctx, "POST", "/login/oauth:create-session", p, &resp, false)
	return resp.AuthURL, err
}

type BeginAuthorizationFlowParams struct {
	CodeChallenge string
	ClientID      string
}

type BeginAuthorizationFlowResponse struct {
	// DeviceCode is the device verification code.
	DeviceCode string `json:"device_code"`

	// UserCode is the end-user verification code.
	UserCode string `json:"user_code"`

	// VerificationURI is the end-user URL to use to login.
	VerificationURI string `json:"verification_uri"`

	// ExpiresIn is the lifetime in seconds of the device code and user code.
	ExpiresIn int `json:"expires_in"`

	// Interval is the number of seconds to wait between polling requests.
	// If not provided, defaults to 5.
	Interval int `json:"interval,omitempty"`
}

func BeginDeviceAuthFlow(ctx context.Context, p BeginAuthorizationFlowParams) (*BeginAuthorizationFlowResponse, error) {
	vals := url.Values{}
	vals.Set("code_challenge", p.CodeChallenge)
	vals.Set("client_id", p.ClientID)
	body := strings.NewReader(vals.Encode())

	req, err := http.NewRequestWithContext(ctx, "POST", conf.APIBaseURL+"/oauth/device-auth", body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := doPlatformReq(req, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, decodeErrorResponse(resp)
	}
	var respData BeginAuthorizationFlowResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("decoding response body: %w", err)
	}
	return &respData, nil
}

type PollDeviceAuthFlowParams struct {
	DeviceCode   string
	CodeVerifier string
}

type OAuthToken struct {
	*oauth2.Token
	Actor   string `json:"actor,omitempty"` // The ID of the user or app that authorized the token.
	Email   string `json:"email"`           // empty if logging in as an app
	AppSlug string `json:"app_slug"`        // empty if logging in as a user
}

func PollDeviceAuthFlow(ctx context.Context, p PollDeviceAuthFlowParams) (*OAuthToken, error) {
	vals := url.Values{}
	vals.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	vals.Set("device_code", p.DeviceCode)
	vals.Set("code_verifier", p.CodeVerifier)
	body := strings.NewReader(vals.Encode())

	req, err := http.NewRequestWithContext(ctx, "POST", conf.APIBaseURL+"/oauth/token", body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := doPlatformReq(req, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, decodeErrorResponse(resp)
	}

	var tok OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decoding response body: %w", err)
	}
	return &tok, nil
}

type ExchangeOAuthTokenParams struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

type OAuthData struct {
	Token   *oauth2.Token `json:"token"`
	Actor   string        `json:"actor,omitempty"` // The ID of the user or app that authorized the token.
	Email   string        `json:"email"`           // empty if logging in as an app
	AppSlug string        `json:"app_slug"`        // empty if logging in as a user
}

func ExchangeOAuthToken(ctx context.Context, p *ExchangeOAuthTokenParams) (*OAuthData, error) {
	var resp OAuthData
	err := call(ctx, "POST", "/login/oauth:exchange-token", p, &resp, false)
	return &resp, err
}

type ExchangeAuthKeyParams struct {
	AuthKey string `json:"auth_key"`
}

func ExchangeAuthKey(ctx context.Context, p *ExchangeAuthKeyParams) (*OAuthData, error) {
	var resp OAuthData
	err := call(ctx, "POST", "/login/auth-key", p, &resp, false)
	return &resp, err
}
