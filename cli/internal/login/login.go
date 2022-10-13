// Package login handles login and authentication with Encore's platform.
package login

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"

	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
)

// Flow keeps the state of an ongoing login flow.
type Flow struct {
	URL     string            // Local URL the flow is listening on
	LoginCh chan *conf.Config // Successful logins are sent on this

	state           string
	challenge       string
	pubKey, privKey string
	srv             *http.Server
	ln              net.Listener
}

// Begin begins a new login attempt.
func Begin() (f *Flow, err error) {
	// Generate initial request state
	state, err1 := genRandData()
	challenge, err2 := genRandData()
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("could not generate random data: %v/%v", err1, err2)
	}

	challengeHash := sha256.Sum256([]byte(challenge))
	encodedChallenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			ln.Close()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://localhost:%d/oauth", addr.Port)

	req := &platform.CreateOAuthSessionParams{
		Challenge:   encodedChallenge,
		State:       state,
		RedirectURL: url,
	}
	authURL, err := platform.CreateOAuthSession(context.Background(), req)
	if err != nil {
		return nil, err
	}

	flow := &Flow{
		URL:     authURL,
		LoginCh: make(chan *conf.Config),

		state:     state,
		challenge: challenge,
	}
	flow.srv = &http.Server{Handler: http.HandlerFunc(flow.oauthHandler)}
	go flow.srv.Serve(ln)
	return flow, nil
}

// Close closes the login flow.
func (f *Flow) Close() {
	f.srv.Close()
}

func (f *Flow) oauthHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/oauth" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	code := req.FormValue("code")
	reqState := req.FormValue("state")
	if code == "" || reqState != f.state {
		http.Error(w, "Bad Request (bad code or state)", http.StatusBadRequest)
		return
	}

	params := &platform.ExchangeOAuthTokenParams{
		Challenge: f.challenge,
		Code:      code,
	}
	resp, err := platform.ExchangeOAuthToken(req.Context(), params)
	if err != nil {
		http.Error(w, "Could not exchange token: "+err.Error(), http.StatusBadGateway)
		return
	} else if resp.Token == nil {
		http.Error(w, "Invalid response: missing token", http.StatusBadGateway)
		return
	}

	conf := &conf.Config{Token: *resp.Token, Email: resp.Email, AppSlug: resp.AppSlug}
	select {
	case f.LoginCh <- conf:
		http.Redirect(w, req, "https://www.encore.dev/auth/success", http.StatusFound)
	default:
		http.Error(w, "Unexpected request", http.StatusBadRequest)
	}
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
	conf := &conf.Config{Token: *tok, AppSlug: resp.AppSlug}

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
