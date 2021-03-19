// Package conf writes and reads the Encore configuration file for the user.
package conf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"go4.org/syncutil"
	"golang.org/x/oauth2"
)

// Config represents the stored Encore configuration.
type Config struct {
	oauth2.Token
	Email     string `json:"email"`
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

	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "encore", ".auth_token")
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
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "encore", ".auth_token")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func CurrentUser() (*Config, error) {
	dir, err := os.UserConfigDir()
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
		configDir, err = os.UserConfigDir()
		if err != nil {
			return nil, err
		}
	}

	return readConf(configDir)
}

func readConf(configDir string) (*Config, error) {
	path := filepath.Join(configDir, "encore", ".auth_token")
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
	setup syncutil.Once
	ts    oauth2.TokenSource
}

// Token implements oauth2.TokenSource.
func (ts *TokenSource) Token() (*oauth2.Token, error) {
	err := ts.setup.Do(func() error {
		cfg, err := CurrentUser()
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("not logged in: run 'encore auth login' first")
		} else if err != nil {
			return fmt.Errorf("could not get Encore auth token: %v", err)
		}

		oauth2Cfg := &oauth2.Config{
			Endpoint: oauth2.Endpoint{
				TokenURL: "https://api.encore.dev/login/oauth:refresh-token",
			},
		}
		ts.ts = oauth2Cfg.TokenSource(context.Background(), &cfg.Token)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ts.ts.Token()
}

// AuthClient creates an *http.Client that authenticates requests
// using the logged-in user.
func AuthClient() *http.Client {
	return oauth2.NewClient(nil, &TokenSource{})
}
