// +build darwin

package wgtunnel

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
)

const (
	basePath   = "/var/run/wireguard"
	tunnelName = "encore"
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
	iface, err := createIface()
	if err != nil {
		return err
	}
	if err := configure(iface, cc, sc); err != nil {
		delIface(iface)
		return err
	}
	return nil
}

func stop() error {
	device, err := getIface()
	if err != nil {
		return err
	} else if device != "" {
		return delIface(device)
	}
	return nil
}

func status() (bool, error) {
	device, err := getIface()
	return device != "", err
}

func createIface() (device string, err error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return "", err
	}
	namePath := filepath.Join(basePath, tunnelName+".name")
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
			if iface, err := ioutil.ReadFile(namePath); err == nil {
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

func getIface() (device string, err error) {
	namePath := filepath.Join(basePath, tunnelName+".name")
	data, err := ioutil.ReadFile(namePath)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	deviceName := string(bytes.TrimSpace(data))
	devicePath := filepath.Join(basePath, deviceName+".sock")

	nm, err1 := os.Stat(namePath)
	dev, err2 := os.Stat(devicePath)
	if os.IsNotExist(err1) || os.IsNotExist(err2) {
		return "", nil
	} else if err1 != nil {
		return "", err1
	} else if err2 != nil {
		return "", err2
	}

	diff := nm.ModTime().Sub(dev.ModTime())
	if diff < 2*time.Second && diff > -2*time.Second {
		return deviceName, nil
	}
	return "", nil
}

func delIface(device string) error {
	namePath := filepath.Join(basePath, tunnelName+".name")
	if err := os.Remove(namePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if device != "" {
		devicePath := filepath.Join(basePath, device+".sock")
		if err := os.Remove(devicePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func configure(device string, cc *ClientConfig, sc *ServerConfig) error {
	ops := []func(string, *ClientConfig, *ServerConfig) error{
		setConf, addAddr, setMtus, enableIface, addPeers,
	}
	for _, op := range ops {
		if err := op(device, cc, sc); err != nil {
			return err
		}
	}
	return nil
}

func addAddr(device string, cc *ClientConfig, sc *ServerConfig) error {
	out, err := exec.Command("/sbin/ifconfig", device, "inet", cc.Addr, cc.Addr, "alias").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not add route: %v: %s", err, out)
	}
	return nil
}

var (
	routeRe = regexp.MustCompile(`(?m)interface: ([^ ]+)$`)
	mtuRe   = regexp.MustCompile(`mtu ([0-9]+)`)
)

func addPeers(device string, cc *ClientConfig, sc *ServerConfig) error {
	for _, r := range sc.Peers {
		for _, s := range r.Subnets {
			if err := addRoute(device, cc, s); err != nil {
				return fmt.Errorf("adding route: %v", err)
			}
		}
	}
	return nil
}

func addRoute(device string, cc *ClientConfig, subnet net.IPNet) error {
	// Determine if this is already routed via this interface
	out, err := exec.Command("/sbin/route", "-n", "get", "-inet", subnet.String()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could get route: %v: %s", err, out)
	}
	m := routeRe.FindSubmatch(out)
	if m != nil && string(m[1]) == device {
		return nil
	}

	out, err = exec.Command("/sbin/route", "-n", "add", "-inet", subnet.String(), "-interface", device).CombinedOutput()
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
	// Parse the underlying interface that will actually send the packets
	// to our destination endpoint.
	out, err := exec.Command("/sbin/route", "-n", "get", "-inet", r.Endpoint.IP.String()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could get route: %v: %s", err, out)
	}
	m := routeRe.FindSubmatch(out)
	if m == nil {
		return fmt.Errorf("could not determine routing interface")
	}
	ifaceName := string(m[1])

	out, err = exec.Command("/sbin/ifconfig", ifaceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not get iface info: %v: %s", err, out)
	}
	m = mtuRe.FindSubmatch(out)
	if m == nil {
		return fmt.Errorf("could not determine MTU")
	}
	mtu, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return fmt.Errorf("could not parse MTU: %v", err)
	}
	mtu = mtu - 80

	out, err = exec.Command("/sbin/ifconfig", device, "mtu", strconv.Itoa(mtu)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not set MTU: %v: %s", err, out)
	}
	return nil
}

func enableIface(device string, cc *ClientConfig, sc *ServerConfig) error {
	out, err := exec.Command("/sbin/ifconfig", device, "up").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not enable device: %v: %s", err, out)
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

	device := device.NewDevice(tun, conn.NewDefaultBind(), logger)

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
