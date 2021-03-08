// +build linux

package wgtunnel

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
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

	iface, err := createIface()
	if err != nil {
		return err
	}

	if err := configure(iface, cc, sc); err != nil {
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

func createIface() (device string, err error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return "", err
	}

	// Use the kernel module if possible
	out, err := exec.Command("ip", "link", "add", iface, "type", "wireguard").CombinedOutput()
	if err == nil {
		return iface, nil
	}
	// Do we have the kernel module installed?
	if _, err := os.Stat("/sys/module/wireguard"); err == nil {
		return "", fmt.Errorf("could not setup WireGuard device: %s", out)
	}
	fmt.Println("encore: missing WireGuard kernel module. Falling back to slow userspace implementation.")
	fmt.Println("encore: Install WireGuard kernel module to hide this message.")

	namePath := filepath.Join(basePath, iface+".name")
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(exe, "vpn", "__run")
	cmd.Env = append(os.Environ(), "WG_TUN_NAME_FILE="+namePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return "", err
	}

	tunnelErr := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		tunnelErr <- fmt.Errorf("%s: %s", err, output.String())
	}()

	// Wait for the file to get populated
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			if iface, err := os.ReadFile(namePath); err == nil {
				device = string(bytes.TrimSpace(iface))
				return device, nil
			}
		case <-timeout:
			cmd.Process.Kill()
			return "", fmt.Errorf("could not determine device name after 5s")
		case err := <-tunnelErr:
			return "", fmt.Errorf("wireguard exited: %v", err)
		}
	}
}

func configure(device string, cc *ClientConfig, sc *ServerConfig) error {
	ops := []func(string, *ClientConfig, *ServerConfig) error{
		setConf, addAddr, setMtus, addPeers,
	}
	for _, op := range ops {
		if err := op(device, cc, sc); err != nil {
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
	tun, err := tun.CreateTUN("utun", device.DefaultMTU)
	if err != nil {
		return err
	}
	name, err := tun.Name()
	if err != nil {
		return err
	}
	fileUAPI, err := ipc.UAPIOpen(name)
	if err != nil {
		return fmt.Errorf("uapi open: %v", err)
	}
	uapi, err := ipc.UAPIListen(name, fileUAPI)
	if err != nil {
		return fmt.Errorf("failed to listen on uapi socket: %v", err)
	}

	logger := device.NewLogger(
		device.LogLevelError,
		"vpn: ",
	)

	device := device.NewDevice(tun, logger)

	term := make(chan os.Signal, 1)
	errs := make(chan error)

	signal.Notify(term, syscall.SIGTERM)
	signal.Notify(term, os.Interrupt)

	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				errs <- err
				return
			}
			go device.IpcHandle(conn)
		}
	}()

	select {
	case <-term:
	case <-device.Wait():
	case err = <-errs:
	}

	uapi.Close()
	device.Close()
	return err
}
