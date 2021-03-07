// +build windows

package wgtunnel

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.zx2c4.com/wireguard/windows/services"
)

const tunnelName = "encore-wg"

func start(cc *ClientConfig, sc *ServerConfig) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	confPath := filepath.Join(configDir, "encore", tunnelName+".conf")
	if err := os.MkdirAll(filepath.Dir(confPath), 0755); err != nil {
		return err
	} else if err := writeConf(confPath, cc, sc); err != nil {
		return fmt.Errorf("cannot write WireGuard conf: %v", err)
	}

	err = installTunnel(confPath)
	if err != nil {
		return fmt.Errorf("could not install tunnel: %v", err)
	}
	return nil
}

func stop() error {
	if running, err := status(); err != nil || running {
		return uninstallTunnel(tunnelName)
	}
	return nil
}

func status() (bool, error) {
	serviceName, err := services.ServiceNameOfTunnel(tunnelName)
	if err != nil {
		return false, err
	}

	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return false, err
	}
	m := &mgr.Mgr{Handle: h}
	list, err := m.ListServices()
	if err != nil {
		return false, err
	}
	for _, svc := range list {
		if svc == serviceName {
			return true, nil
		}
	}
	return false, nil
}

func writeConf(confPath string, cc *ClientConfig, sc *ServerConfig) error {
	var peers []string
	for _, r := range sc.Peers {
		var subnets []string
		for _, s := range r.Subnets {
			subnets = append(subnets, s.String())
		}
		peer := fmt.Sprintf(`[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = 25
`, r.PublicKey, strings.Join(subnets, ", "), r.Endpoint)
		peers = append(peers, peer)
	}

	cfg := fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s\n\n", cc.PrivKey, cc.Addr) + strings.Join(peers, "\n\n")

	err := ioutil.WriteFile(confPath, []byte(cfg), 0600)
	if err != nil {
		return err
	}
	return nil
}

func installTunnel(configPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	err = runElevatedShellCommand(exe, "vpn", "svc-install", configPath)
	if err != nil {
		return fmt.Errorf("could not install tunnel: %v", err)
	}
	return nil
}

func uninstallTunnel(name string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	err = runElevatedShellCommand(exe, "vpn", "svc-uninstall", name)
	if err != nil {
		return fmt.Errorf("could not uninstall tunnel: %v", err)
	}
	return nil
}

func runElevatedShellCommand(cmd string, args ...string) error {
	verb := "runas"
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Escape args if they contain spaces or quotes
	for i, arg := range args {
		args[i] = shellEscape(arg)
	}
	argStr := strings.Join(args, " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(cmd)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(argStr)
	var showCmd int32 = 0 // SW_NORMAL
	return windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
}

func shellEscape(arg string) string {
	if strings.ContainsAny(arg, `" `) {
		arg = "\"" + strings.Replace(arg, "\"", "\"\"", -1) + "\""
	}
	return arg
}

func run() error {
	return fmt.Errorf("Run() is not implemented on windows")
}
