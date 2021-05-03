// Package login handles login and authentication with Encore's platform.
package login

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"

	"encr.dev/cli/internal/conf"
	"encr.dev/cli/internal/version"
	"encr.dev/cli/internal/wgtunnel"
	"golang.org/x/oauth2"
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

	req := map[string]string{
		"challenge":    encodedChallenge,
		"state":        state,
		"redirect_url": url,
	}
	var resp struct {
		OK    bool
		Error struct {
			Code   string
			Detail interface{}
		}
		Data struct {
			AuthURL string `json:"auth_url"`
		}
	}
	if err := apiReq("/login/oauth:create-session", req, &resp); err != nil {
		return nil, fmt.Errorf("could not contact auth server: %v", err)
	} else if !resp.OK {
		return nil, fmt.Errorf("auth failure: code: %s", resp.Error.Code)
	}

	flow := &Flow{
		URL:     resp.Data.AuthURL,
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

	reqData := map[string]string{
		"challenge": f.challenge,
		"code":      code,
	}
	var resp struct {
		OK    bool
		Error struct {
			Code   string
			Detail interface{}
		}
		Data struct {
			Email string
			Token *oauth2.Token
		}
	}
	if err := apiReq("/login/oauth:exchange-token", reqData, &resp); err != nil {
		http.Error(w, "Could not exchange token: "+err.Error(), http.StatusBadGateway)
		return
	} else if !resp.OK {
		http.Error(w, "Could not exchange token: "+resp.Error.Code, http.StatusBadGateway)
		return
	} else if resp.Data.Token == nil {
		http.Error(w, "Invalid response: missing token", http.StatusBadGateway)
		return
	}

	tok := resp.Data.Token
	conf := &conf.Config{Token: *tok, Email: resp.Data.Email}
	pub, priv, err := wgtunnel.GenKey()
	if err == nil {
		conf.WireGuard.PublicKey = pub.String()
		conf.WireGuard.PrivateKey = priv.String()
	}
	select {
	case f.LoginCh <- conf:
		http.Redirect(w, req, "https://www.encore.dev/auth/success", http.StatusFound)
	default:
		http.Error(w, "Unexpected request", http.StatusBadRequest)
	}
}

func genRandData() (string, error) {
	data := make([]byte, 32)
	_, err := rand.Read(data[:])
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func apiReq(endpoint string, reqParams, respParams interface{}) error {
	var body io.Reader
	if reqParams != nil {
		reqData, err := json.Marshal(reqParams)
		if err != nil {
			return err
		}
		body = bytes.NewReader(reqData)
	}

	req, err := http.NewRequest("POST", "https://api.encore.dev"+endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Add a very limited amount of information for diagnostics
	req.Header.Set("X-Encore-Version", version.Version)
	req.Header.Set("X-Encore-GOOS", runtime.GOOS)
	req.Header.Set("X-Encore-GOARCH", runtime.GOARCH)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if respParams != nil {
		err := json.NewDecoder(resp.Body).Decode(respParams)
		return err
	}
	return nil
}
