package platform

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"

	metav1 "encr.dev/proto/encore/parser/meta/v1"
)

type CreateAppParams struct {
	Name           string `json:"name"`
	InitialSecrets map[string]string
}

type App struct {
	ID          string  `json:"eid"`
	LegacyID    string  `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"` // can be blank
	MainBranch  *string `json:"main_branch"` // nil if not set
}

type Env struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Type  string `json:"type"`
	Cloud string `json:"cloud"`
}

func CreateApp(ctx context.Context, p *CreateAppParams) (*App, error) {
	var resp App
	err := call(ctx, "POST", "/apps", p, &resp, true)
	return &resp, err
}

func ListApps(ctx context.Context) ([]*App, error) {
	var resp []*App
	err := call(ctx, "GET", "/user/apps", nil, &resp, true)
	return resp, err
}

func GetApp(ctx context.Context, appSlug string) (*App, error) {
	var resp App
	err := call(ctx, "GET", "/apps/"+url.PathEscape(appSlug), nil, &resp, true)
	return &resp, err
}

func ListEnvs(ctx context.Context, appSlug string) ([]*Env, error) {
	var resp []*Env
	err := call(ctx, "GET", "/apps/"+url.PathEscape(appSlug)+"/envs", nil, &resp, true)
	return resp, err
}

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

type SecretKind string

const (
	DevelopmentSecrets SecretKind = "development"
	ProductionSecrets  SecretKind = "production"
)

func GetLocalSecretValues(ctx context.Context, appSlug string, poll bool) (secrets map[string]string, err error) {
	url := "/apps/" + url.PathEscape(appSlug) + "/secrets:values?kind=development"
	if poll {
		url += "&poll=true"
	}
	err = call(ctx, "GET", url, nil, &secrets, true)
	return secrets, err
}

type SecretVersion struct {
	Number  int       `json:"number"`
	Created time.Time `json:"created"`
}

func SetAppSecret(ctx context.Context, appSlug string, kind SecretKind, secretKey, value string) (*SecretVersion, error) {
	params := struct {
		Kind  SecretKind
		Value string
	}{Kind: kind, Value: value}
	url := fmt.Sprintf("/apps/%s/secrets/%s/versions",
		url.PathEscape(appSlug),
		url.PathEscape(secretKey),
	)
	var resp SecretVersion
	err := call(ctx, "POST", url, &params, &resp, true)
	return &resp, err
}

func GetEnvMeta(ctx context.Context, appSlug, envName string) (*metav1.Data, error) {
	url := "/apps/" + url.PathEscape(appSlug) + "/envs/" + url.PathEscape(envName) + "/meta"
	body, err := rawCall(ctx, "GET", url, nil, true)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("platform.GetEnvMeta: %v", err)
	}
	var md metav1.Data
	if err := proto.Unmarshal(data, &md); err != nil {
		return nil, fmt.Errorf("platform.GetEnvMeta: %v", err)
	}
	return &md, nil
}

func DBConnect(ctx context.Context, appSlug, envSlug, dbName string, startupData []byte) (*websocket.Conn, error) {
	path := escapef("/apps/%s/envs/%s/sqldb-connect/%s", appSlug, envSlug, dbName)
	return wsDial(ctx, path, true, map[string]string{
		"X-Startup-Message": base64.StdEncoding.EncodeToString(startupData),
	})
}

func EnvLogs(ctx context.Context, appSlug, envSlug string) (*websocket.Conn, error) {
	path := escapef("/apps/%s/envs/%s/log", appSlug, envSlug)
	return wsDial(ctx, path, true, nil)
}

func escapef(format string, args ...string) string {
	ifaces := make([]interface{}, len(args))
	for i, arg := range args {
		ifaces[i] = url.PathEscape(arg)
	}
	return fmt.Sprintf(format, ifaces...)
}
