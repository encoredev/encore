// Package conf writes and reads the Encore configuration file for the user.
package conf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go4.org/syncutil"
	"golang.org/x/oauth2"
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")
var ErrNotLoggedIn = errors.New("not logged in: run 'encore auth login' first")

// These can be overwritten using
// `go build -ldflags "-X encr.dev/cli/internal/conf.defaultPlatformURL=https://api.encore.dev"`.
var (
	defaultPlatformURL     = "https://api.encore.dev"
	defaultConfigDirectory = "encore"
)

// APIBaseURL is the base URL for communicating with the Encore Platform.
var APIBaseURL = (func() string {
	if u := os.Getenv("ENCORE_PLATFORM_API_URL"); u != "" {
		return u
	}
	return defaultPlatformURL
})()

// WSBaseURL is the base URL for communicating with the Encore Platform over WebSocket.
var WSBaseURL = (func() string {
	return strings.Replace(APIBaseURL, "http", "ws", -1) // "https" becomes "wss"
})()

// Dir reports the directory where Encore's configuration is stored.
func Dir() (string, error) {
	dir := os.Getenv("ENCORE_CONFIG_DIR")
	if dir == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(d, defaultConfigDirectory)
	}
	return dir, nil
}

// Config represents the stored Encore configuration.
type Config struct {
	oauth2.Token
	Actor     string `json:"actor,omitempty"`    // The ID of either the user or app authenticated
	Email     string `json:"email,omitempty"`    // non-zero if logged in as a user
	AppSlug   string `json:"app_slug,omitempty"` // non-zero if logged in as an app
	WireGuard struct {
		PublicKey  string `json:"pub,omitempty"`
		PrivateKey string `json:"priv,omitempty"`
	} `json:"wg,omitempty"`
}

// Write persists the configuration for the user.
func Write(cfg *Config) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("conf.Write: %v", err)
		}
	}()

	dir, err := Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, ".auth_token")
	if data, err := json.Marshal(cfg); err != nil {
		return err
	} else if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	} else if err := ioutil.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return nil
}

func Logout() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, ".auth_token")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	DefaultTokenSource = &TokenSource{}
	AuthClient = oauth2.NewClient(nil, DefaultTokenSource)
	return nil
}

func CurrentUser() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, fmt.Errorf("conf.CurrentUser: %w", err)
	}
	conf, err := readConf(dir)
	if err != nil {
		return nil, fmt.Errorf("conf.CurrentUser: %w", err)
	}
	return conf, nil
}

func OriginalUser(configDir string) (cfg *Config, err error) {
	if runtime.GOOS == "windows" {
		// Windows does not have the notion of a root user, so just use CurrentUser
		return CurrentUser()
	}

	if configDir == "" {
		var err error
		configDir, err = Dir()
		if err != nil {
			return nil, err
		}
	}

	return readConf(configDir)
}

func readConf(configDir string) (*Config, error) {
	path := filepath.Join(configDir, ".auth_token")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conf Config
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

// TokenSource implements oauth2.TokenSource by looking up the
// current logged in user's API Token.
// The zero value is ready to be used.
type TokenSource struct {
	setup     syncutil.Once
	ts        oauth2.TokenSource
	lastToken string
}

// Token implements oauth2.TokenSource.
func (ts *TokenSource) Token() (*oauth2.Token, error) {
	err := ts.setup.Do(func() error {
		cfg, err := CurrentUser()
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotLoggedIn
		} else if err != nil {
			return fmt.Errorf("could not get Encore auth token: %v", err)
		}

		oauth2Cfg := &oauth2.Config{
			Endpoint: oauth2.Endpoint{
				TokenURL: APIBaseURL + "/login/oauth:refresh-token",
			},
		}
		ts.lastToken = cfg.AccessToken
		ts.ts = oauth2Cfg.TokenSource(context.Background(), &cfg.Token)
		return nil
	})
	if err != nil {
		return nil, err
	}
	token, err := ts.ts.Token()
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && re.Response.StatusCode == 422 {
			// The refresh token is invalid. Log the user out to reset the token.
			_ = Logout()
			return nil, ErrInvalidRefreshToken
		}
	} else if ts.lastToken != token.AccessToken {
		// The token has changed, so update the config.
		cfg, err := CurrentUser()
		if err != nil {
			return nil, err
		}
		cfg.Token = *token
		if err := Write(cfg); err != nil {
			return nil, err
		}
		ts.lastToken = token.AccessToken
	}
	return token, err
}

var DefaultTokenSource = &TokenSource{}

// AuthClient is an *http.Client that authenticates requests
// using the logged-in user.
var AuthClient = oauth2.NewClient(nil, DefaultTokenSource)
