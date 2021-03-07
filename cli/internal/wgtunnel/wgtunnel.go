// Package wgtunnel sets up and configures Encore's WireGuard tunnel for
// authenticating against private environments.
package wgtunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"encr.dev/cli/internal/conf"
	"golang.org/x/oauth2"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GenKey generates a public/private key pair for the WireGuard tunnel.
func GenKey() (pub, priv wgtypes.Key, err error) {
	priv, err = wgtypes.GeneratePrivateKey()
	if err == nil {
		pub = priv.PublicKey()
	}
	return
}

// RegisterDevice registers the public key with Encore
// and returns the allocated IP address for use with WireGuard.
func RegisterDevice(ctx context.Context, pubKey wgtypes.Key) (ip string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("wgtunnel.RegisterDevice: %v", err)
		}
	}()

	reqData, _ := json.Marshal(map[string]string{"public_key": pubKey.String()})
	url := "https://api.encore.dev/user/devices:register"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := oauth2.NewClient(ctx, &conf.TokenSource{})
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed: %s: %s", resp.Status, body)
	}

	var respData struct {
		OK   bool
		Data struct {
			IPAddress string `json:"ip_address"`
		} `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", err
	} else if !respData.OK {
		return "", fmt.Errorf("request failed with code: %v", respData.Error.Code)
	}
	return respData.Data.IPAddress, nil
}

// ClientConfig is the configuration necessary to setup WireGuard.
type ClientConfig struct {
	// Addr is our WireGuard address.
	Addr string
	// PrivKey is our private key.
	PrivKey wgtypes.Key
}

// ServerPeer is the required configuration to configure a WireGuard peer.
type ServerPeer struct {
	// Endpoint is the WireGuard endpoint for the server.
	Endpoint net.UDPAddr
	// PublicKey is the server's public key.
	PublicKey wgtypes.Key
	// Subnets are the network subnet that should be routed
	// through WireGuard.
	Subnets []net.IPNet
}

// ServerConfig is the configuration to set up WireGuard peers.
type ServerConfig struct {
	Peers []ServerPeer
}

// DefaultServerConfig is the well-known default configuration of Encore's API Gateway.
var DefaultServerConfig = &ServerConfig{
	Peers: []ServerPeer{
		{
			Endpoint: net.UDPAddr{
				IP:   net.ParseIP("159.65.210.129"),
				Port: 51820,
			},
			PublicKey: mustParseKey("mQzDYCJufL+rNqbS1fBtxx3vxLX/4VaKKUDNS/yhQBs="),
			Subnets: []net.IPNet{
				{
					IP:   net.ParseIP("100.26.25.109"),
					Mask: net.IPv4Mask(255, 255, 255, 255),
				},
				{
					IP:   net.ParseIP("18.214.237.181"),
					Mask: net.IPv4Mask(255, 255, 255, 255),
				},
				{
					IP:   net.ParseIP("54.170.142.107"),
					Mask: net.IPv4Mask(255, 255, 255, 255),
				},
				{
					IP:   net.ParseIP("54.74.172.84"),
					Mask: net.IPv4Mask(255, 255, 255, 255),
				},
			},
		},
	},
}

// Start starts the WireGuard tunnel in the background.
func Start(cc *ClientConfig, sc *ServerConfig) error {
	if sc == nil {
		sc = DefaultServerConfig
	}
	return start(cc, sc)
}

// Stop stops the WireGuard tunnel.
func Stop() error {
	return stop()
}

// Status reports whether the tunnel is running.
func Status() (running bool, err error) {
	return status()
}

func setConf(device string, cc *ClientConfig, sc *ServerConfig) error {
	cfg := wgtypes.Config{
		PrivateKey:   &cc.PrivKey,
		ReplacePeers: true,
	}
	keepAlive := 25 * time.Second
	for _, r := range sc.Peers {
		cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
			PublicKey:                   r.PublicKey,
			Endpoint:                    &r.Endpoint,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  r.Subnets,
			PersistentKeepaliveInterval: &keepAlive,
		})
	}

	cl, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer cl.Close()
	return cl.ConfigureDevice(device, cfg)
}

func mustParseKey(s string) wgtypes.Key {
	k, err := wgtypes.ParseKey(s)
	if err != nil {
		panic(err)
	}
	return k
}

// Run synchronously runs the tunnel.
// It is only implemented on macOS.
func Run() error {
	return run()
}
