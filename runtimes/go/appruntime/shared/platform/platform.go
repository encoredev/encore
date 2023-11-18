// Package platform handles communication with the Encore Platform.
package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"time"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
)

func NewClient(static *config.Static, rt *config.Runtime) *Client {
	exp := experiments.FromConfig(static, rt)
	return &Client{static, rt, exp}
}

type Client struct {
	static  *config.Static
	runtime *config.Runtime
	exp     *experiments.Set
}

func (c *Client) addAuthKey(req *http.Request) {
	k := c.runtime.AuthKeys[0]
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	mac := hmac.New(sha256.New, k.Data)
	_, _ = fmt.Fprintf(mac, "%s\x00%s", date, req.URL.Path)

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
	for _, k := range c.runtime.AuthKeys {
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
	_, _ = fmt.Fprintf(mac, "%s\x00%s", dateStr, req.URL.Path)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, gotMac)
}
