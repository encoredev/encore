// Package login handles login and authentication with Encore's platform.
package login

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"encr.dev/cli/internal/browser"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
	"encr.dev/internal/env"
)

func DecideFlow() (*conf.Config, error) {
	if env.IsSSH() || !browser.CanOpen() {
		return DeviceAuth()
	}
	return Interactive()
}

func WithAuthKey(authKey string) (*conf.Config, error) {
	params := &platform.ExchangeAuthKeyParams{
		AuthKey: authKey,
	}
	resp, err := platform.ExchangeAuthKey(context.Background(), params)
	if err != nil {
		return nil, err
	} else if resp.Token == nil {
		return nil, fmt.Errorf("invalid response: missing token")
	}

	tok := resp.Token
	conf := &conf.Config{Token: *tok, Actor: resp.Actor, AppSlug: resp.AppSlug}

	return conf, nil
}

func genRandData() (string, error) {
	data := make([]byte, 32)
	_, err := rand.Read(data[:])
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
