package platform

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	metav1 "encr.dev/proto/encore/parser/meta/v1"
	"github.com/golang/protobuf/proto"
	"golang.org/x/oauth2"
)

type CreateAppParams struct {
	Name string `json:"name"`
}

type App struct {
	Slug       string  `json:"slug"`
	MainBranch *string `json:"main_branch"` // nil if not set
}

func CreateApp(ctx context.Context, p *CreateAppParams) (*App, error) {
	var resp App
	err := call(ctx, "POST", "/apps", p, &resp)
	return &resp, err
}

func GetApp(ctx context.Context, appSlug string) (*App, error) {
	var resp App
	err := call(ctx, "GET", "/apps/"+url.PathEscape(appSlug), nil, &resp)
	return &resp, err
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
	err = call(ctx, "POST", "/login/oauth:create-session", p, &resp)
	return resp.AuthURL, err
}

type ExchangeOAuthTokenParams struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

type OAuthData struct {
	Token   *oauth2.Token `json:"token"`
	Email   string        `json:"email"`    // empty if logging in as an app
	AppSlug string        `json:"app_slug"` // empty if logging in as a user
}

func ExchangeOAuthToken(ctx context.Context, p *ExchangeOAuthTokenParams) (*OAuthData, error) {
	var resp OAuthData
	err := call(ctx, "POST", "/login/oauth:exchange-token", p, &resp)
	return &resp, err
}

type SecretKind string

const (
	DevelopmentSecrets SecretKind = "development"
	ProductionSecrets  SecretKind = "production"
)

func GetAppSecrets(ctx context.Context, appSlug string, poll bool, kind SecretKind) (secrets map[string]string, err error) {
	url := "/apps/" + url.PathEscape(appSlug) + "/secrets:values?kind=" + string(kind)
	if poll {
		url += "&poll=true"
	}
	err = call(ctx, "GET", url, nil, &secrets)
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
	err := call(ctx, "POST", url, &params, &resp)
	return &resp, err
}

func GetEnvMeta(ctx context.Context, appSlug, envName string) (*metav1.Data, error) {
	url := "/apps/" + url.PathEscape(appSlug) + "/envs/" + url.PathEscape(envName) + "/meta"
	body, err := rawCall(ctx, "GET", url, nil)
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
