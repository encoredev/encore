package login

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/browser"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
	"encr.dev/internal/env"
)

// DeviceAuth logs in the suser with the device auth flow.
func DeviceAuth() (*conf.Config, error) {
	// Generate PKCE challenge.
	randData, err := genRandData()
	if err != nil {
		return nil, fmt.Errorf("could not generate random data: %v", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString([]byte(randData))
	challengeHash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])

	resp, err := platform.BeginDeviceAuthFlow(context.Background(), platform.BeginAuthorizationFlowParams{
		CodeChallenge: codeChallenge,
		ClientID:      "encore_cli",
	})
	if err != nil {
		return nil, err
	}

	var (
		bold  = color.New(color.Bold)
		faint = color.New(color.Faint)
	)

	fmt.Printf("Your pairing code is %s\n", bold.Sprint(resp.UserCode))
	faint.Println("This pairing code verifies your authentication with Encore.")

	inputCh := make(chan struct{}, 1)

	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spin.Prefix = "Waiting for login confirmation..."

	if !env.IsSSH() && browser.CanOpen() {
		fmt.Fprintf(os.Stdout, "Press Enter to open the browser or visit %s (^C to quit)\n",
			resp.VerificationURI)

		// Asynchronously wait for input.
		w := waitForEnterPress()
		defer w.Stop()
		go func() {
			select {
			case <-w.pressed:
				inputCh <- struct{}{}
			case <-w.quit:
			}
		}()

	} else {
		// On Windows we need a proper \r\n newline to ensure the URL detection doesn't extend to the next line.
		// fmt.Fprintln and family prints just a simple \n, so don't use that.
		fmt.Fprintf(os.Stdout, "To authenticate with Encore, please go to: %s%s", resp.VerificationURI, cmdutil.Newline)
		spin.Start()
	}

	resultCh := make(chan deviceAuthResult, 1)
	go pollForDeviceAuthResult(codeVerifier, resp, resultCh)

	for {
		select {
		case <-inputCh:
			// The user hit Enter; show a spinner and try to open the browser.
			spin.Start()
			if !browser.Open(resp.VerificationURI) {
				spin.FinalMSG = fmt.Sprintf("Failed to open browser, please go to %s manually.", resp.VerificationURI)
				spin.Stop()

				// Create a new spinner so the message above stays around.
				spin = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
				spin.Prefix = "Waiting for login confirmation..."
				spin.Start()
			}

		case res := <-resultCh:
			if res.err != nil {
				spin.FinalMSG = fmt.Sprintf("Failed to log in: %v", res.err)
				spin.Stop()
				return nil, res.err
			}

			spin.Stop()
			return res.cfg, nil
		}
	}
}

type deviceAuthResult struct {
	cfg *conf.Config
	err error
}

func pollForDeviceAuthResult(codeVerifier string, data *platform.BeginAuthorizationFlowResponse, resultCh chan<- deviceAuthResult) {
PollLoop:
	for {
		interval := data.Interval
		if interval <= 0 {
			interval = 5
		}
		time.Sleep(time.Duration(interval) * time.Second)

		resp, err := platform.PollDeviceAuthFlow(context.Background(), platform.PollDeviceAuthFlowParams{
			DeviceCode:   data.DeviceCode,
			CodeVerifier: codeVerifier,
		})
		if err != nil {
			if e, ok := err.(platform.Error); ok {
				switch e.Code {
				case "auth_pending":
					// Not yet authorized, continue polling.
					continue PollLoop

				case "rate_limited":
					// Spurious error; sleep a bit extra before retrying to be safe.
					time.Sleep(5 * time.Second)
					continue PollLoop
				}
			}
			resultCh <- deviceAuthResult{err: err}
			return
		}

		cfg := &conf.Config{Token: *resp.Token, Actor: resp.Actor, Email: resp.Email, AppSlug: resp.AppSlug}
		resultCh <- deviceAuthResult{cfg: cfg}
		return
	}
}

type enterPressWaiter struct {
	quit    chan struct{} // close to abort the waiter
	pressed chan struct{} // closed when enter has been pressed
	runDone chan struct{} // closed when the run goroutine has exited
}

func waitForEnterPress() *enterPressWaiter {
	w := &enterPressWaiter{
		quit:    make(chan struct{}),
		pressed: make(chan struct{}, 1),
		runDone: make(chan struct{}),
	}
	go w.run()
	return w
}

func (w *enterPressWaiter) run() {
	defer close(w.runDone)
	fmt.Fscanln(os.Stdin)
	select {
	case w.pressed <- struct{}{}:
	case <-w.quit:
	}
}

func (w *enterPressWaiter) Stop() {
	close(w.quit)
	os.Stdin.SetReadDeadline(time.Now()) // interrupt the pending read

	// Asynchronously wait for the run goroutine to exit before
	// we reset the read deadline.
	go func() {
		<-w.runDone
		os.Stdin.SetReadDeadline(time.Time{}) // reset read deadline
	}()
}
