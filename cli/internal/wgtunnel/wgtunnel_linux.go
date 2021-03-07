// +build linux

package wgtunnel

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"encr.dev/cli/internal/env"
)

const (
	basePath = "/var/run/wireguard"
	iface    = "encore"
)

type tunnel struct {
	name   string
	device string
	cmd    *exec.Cmd
	stop   func()

	quit chan struct{}
	err  error // can be read after quit is closed
}

func start(cc *ClientConfig, sc *ServerConfig) error {
	if hasIface() {
		if err := delIface(); err != nil {
			return err
		}
	}

	if err := createIface(); err != nil {
		return err
	}
	if err := configure(cc, sc); err != nil {
		delIface()
		return err
	}
	return nil
}

func stop() error {
	return delIface()
}

func status() (bool, error) {
	return hasIface(), nil
}

func hasIface() bool {
	out, _ := exec.Command("ip", "link", "show", "dev", iface).Output()
	return len(out) != 0
}

func delIface() error {
	out, err := exec.Command("ip", "link", "delete", "dev", iface).CombinedOutput()
	if err == nil {
		return nil
	} else if bytes.Contains(out, []byte("Cannot find device")) {
		return nil
	}
	return fmt.Errorf("could not delete device: %s", out)
}

func createIface() error {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return err
	}

	// Use the kernel module if possible
	out, err := exec.Command("ip", "link", "add", iface, "type", "wireguard").CombinedOutput()
	if err == nil {
		return nil
	}
	// Do we have the kernel module installed?
	if _, err := os.Stat("/sys/module/wireguard"); err == nil {
		return fmt.Errorf("could not setup WireGuard device: %s", out)
	}
	fmt.Println("encore: missing WireGuard kernel module. Falling back to slow userspace implementation.")
	fmt.Println("encore: Install WireGuard kernel module to hide this message.")

	out, err = exec.Command(env.Exe("tools", "wireguard-go"), iface).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not setup WireGuard device: %v: %s", err, out)
	}
	return nil
}

func configure(cc *ClientConfig, sc *ServerConfig) error {
	ops := []func(string, *ClientConfig, *ServerConfig) error{
		setConf, addAddr, setMtus, addPeers,
	}
	for _, op := range ops {
		if err := op(iface, cc, sc); err != nil {
			return err
		}
	}
	return nil
}

func addAddr(device string, cc *ClientConfig, sc *ServerConfig) error {
	out, err := exec.Command("ip", "-4", "address", "add", cc.Addr, "dev", device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not add address: %v: %s", err, out)
	}
	return nil
}

var (
	routeRe = regexp.MustCompile(`(?m)interface: ([^ ]+)$`)
	mtuRe   = regexp.MustCompile(`mtu ([0-9]+)`)
	devRe   = regexp.MustCompile(`dev ([^ ]+)`)
)

func addPeers(device string, cc *ClientConfig, sc *ServerConfig) error {
	for _, r := range sc.Peers {
		for _, s := range r.Subnets {
			if err := addRoute(device, cc, s); err != nil {
				return fmt.Errorf("adding route for %s: %v", s, err)
			}
		}
	}
	return nil
}

func addRoute(device string, cc *ClientConfig, subnet net.IPNet) error {
	// Determine if this is already routed via this interface
	out, err := exec.Command("ip", "-4", "route", "show", "dev", device, "match", subnet.String()).Output()
	if err == nil && len(out) > 0 {
		// Already routed
		return nil
	}
	out, err = exec.Command("ip", "-4", "route", "add", subnet.String(), "dev", device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not add route: %v: %s", err, out)
	}
	return nil
}

func setMtus(device string, cc *ClientConfig, sc *ServerConfig) error {
	for _, r := range sc.Peers {
		if err := setMtu(device, cc, r); err != nil {
			return fmt.Errorf("setting mtu: %v", err)
		}
	}
	return nil
}

func setMtu(device string, cc *ClientConfig, r ServerPeer) error {
	var mtu int
	if out, err := exec.Command("ip", "route", "get", r.Endpoint.IP.String()).CombinedOutput(); err == nil {
		if m := mtuRe.FindSubmatch(out); m != nil {
			mtu, _ = strconv.Atoi(string(m[1]))
		}
		if mtu == 0 {
			// Try again by looking for the link device and looking up that
			if d := devRe.FindSubmatch(out); d != nil {
				if out, err := exec.Command("ip", "link", "show", "dev", string(d[1])).CombinedOutput(); err == nil {
					if m := mtuRe.FindSubmatch(out); m != nil {
						mtu, _ = strconv.Atoi(string(m[1]))
					}
				}
			}
		}
	}

	// If we still don't have an mtu, fall back to the default
	if mtu == 0 {
		if out, err := exec.Command("ip", "route", "show", "default").CombinedOutput(); err == nil {
			if m := mtuRe.FindSubmatch(out); m != nil {
				mtu, _ = strconv.Atoi(string(m[1]))
			}
		}
	}

	if mtu == 0 {
		mtu = 1500
	}
	mtu = mtu - 80

	out, err := exec.Command("ip", "link", "set", "mtu", strconv.Itoa(mtu), "up", "dev", device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not set MTU: %v: %s", err, out)
	}
	return nil
}

func run() error {
	return fmt.Errorf("Run() is not implemented on linux")
}
