package login

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/briandowns/spinner"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/browser"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
	"encr.dev/internal/env"
)

// interactive keeps the state of an ongoing login flow.
type interactive struct {
	result chan *conf.Config // Successful logins are sent on this

	state           string
	challenge       string
	pubKey, privKey string
	srv             *http.Server
	ln              net.Listener
}

// Interactive begins an interactive login attempt.
func Interactive() (*conf.Config, error) {
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
	defer ln.Close()
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

	flow := &interactive{
		result: make(chan *conf.Config),

		state:     state,
		challenge: challenge,
	}
	flow.srv = &http.Server{Handler: http.HandlerFunc(flow.oauthHandler)}
	go flow.srv.Serve(ln)

	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)

	if env.IsSSH() || !browser.Open(authURL) {
		// On Windows we need a proper \r\n newline to ensure the URL detection doesn't extend to the next line.
		// fmt.Fprintln and family prints just a simple \n, so don't use that.
		fmt.Fprint(os.Stdout, "Log in to Encore using your browser here: ", authURL, cmdutil.Newline)
	} else {
		spin.Prefix = "Waiting for login to complete "
		spin.Start()
		defer spin.Stop()
	}

	select {
	case res := <-flow.result:
		return res, nil
	case <-time.After(10 * time.Minute):
		return nil, errors.New("Timed out waiting for login confirmation")
	}
}

func (f *interactive) oauthHandler(w http.ResponseWriter, req *http.Request) {
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

	conf := &conf.Config{Token: *resp.Token, Actor: resp.Actor, Email: resp.Email, AppSlug: resp.AppSlug}
	select {
	case f.result <- conf:
		http.Redirect(w, req, "https://www.encore.dev/auth/success", http.StatusFound)
	default:
		http.Error(w, "Unexpected request", http.StatusBadRequest)
	}
}
