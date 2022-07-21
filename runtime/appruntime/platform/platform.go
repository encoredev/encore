// Package platform handles communication with the Encore Platform.
package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/model"
	"encore.dev/appruntime/trace"
)

func NewClient(cfg *config.Config) *Client {
	return &Client{cfg}
}

type Client struct {
	cfg *config.Config
}

func (c *Client) SendTrace(ctx context.Context, id model.TraceID, data io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Runtime.TraceEndpoint, data)
	if err != nil {
		return err
	}
	req.Header.Set("X-Encore-App-ID", c.cfg.Runtime.AppID)
	req.Header.Set("X-Encore-Env-ID", c.cfg.Runtime.EnvID)
	req.Header.Set("X-Encore-Deploy-ID", c.cfg.Runtime.DeployID)
	req.Header.Set("X-Encore-App-Commit", c.cfg.Static.AppCommit.AsRevisionString())
	req.Header.Set("X-Encore-Trace-ID", base64.RawStdEncoding.EncodeToString(id[:]))
	req.Header.Set("X-Encore-Trace-Version", strconv.Itoa(int(trace.CurrentVersion)))
	c.addAuthKey(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %s: %s", resp.Status, body)
	}
	return nil
}

func (c *Client) addAuthKey(req *http.Request) {
	k := c.cfg.Runtime.AuthKeys[0]
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	mac := hmac.New(sha256.New, k.Data)
	fmt.Fprintf(mac, "%s\x00%s", date, req.URL.Path)

	bytes := make([]byte, 4, 4+sha256.Size)
	binary.BigEndian.PutUint32(bytes[0:4], k.KeyID)
	bytes = mac.Sum(bytes)
	auth := base64.RawStdEncoding.EncodeToString(bytes)
	req.Header.Set("X-Encore-Auth", auth)
}

// ValidatePlatformRequest validates whether a request originated from the platform.
func (c *Client) ValidatePlatformRequest(req *http.Request, macSig string) (bool, error) {
	macBytes, err := base64.RawStdEncoding.DecodeString(macSig)
	if err != nil {
		return false, nil
	}

	// Pull out key ID from hmac prefix
	const keyIDLen = 4
	if len(macBytes) < keyIDLen {
		return false, nil
	}

	keyID := binary.BigEndian.Uint32(macBytes[:keyIDLen])
	mac := macBytes[keyIDLen:]
	for _, k := range c.cfg.Runtime.AuthKeys {
		if k.KeyID == keyID {
			return checkAuthKey(k, req, mac), nil
		}
	}

	return false, nil
}

func checkAuthKey(key config.EncoreAuthKey, req *http.Request, gotMac []byte) bool {
	dateStr := req.Header.Get("Date")
	if dateStr == "" {
		return false
	}
	date, err := http.ParseTime(dateStr)
	if err != nil {
		return false
	}
	const threshold = 15 * time.Minute
	if diff := time.Since(date); diff > threshold || diff < -threshold {
		return false
	}

	mac := hmac.New(sha256.New, key.Data)
	fmt.Fprintf(mac, "%s\x00%s", dateStr, req.URL.Path)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, gotMac)
}
