// +build windows

package winsvc

import (
	"errors"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/services"
	"golang.zx2c4.com/wireguard/windows/tunnel"
)

var cachedServiceManager *mgr.Mgr

func serviceManager() (*mgr.Mgr, error) {
	if cachedServiceManager != nil {
		return cachedServiceManager, nil
	}
	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	cachedServiceManager = m
	return cachedServiceManager, nil
}

func Status(name string) (installed bool, err error) {
	serviceName, err := services.ServiceNameOfTunnel(name)
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

func Install(configPath string) error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	path, err := os.Executable()
	if err != nil {
		return nil
	}

	name, err := conf.NameFromPath(configPath)
	if err != nil {
		return err
	}

	serviceName, err := services.ServiceNameOfTunnel(name)
	if err != nil {
		return err
	}
	service, err := m.OpenService(serviceName)
	if err == nil {
		status, err := service.Query()
		if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			service.Close()
			return err
		}
		if status.State != svc.Stopped && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			service.Close()
			return errors.New("Tunnel already installed and running")
		}
		err = service.Delete()
		service.Close()
		if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			return err
		}
		for {
			service, err = m.OpenService(serviceName)
			if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				break
			}
			service.Close()
			time.Sleep(time.Second / 3)
		}
	}

	config := mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
		Dependencies: []string{"Nsi"},
		DisplayName:  "Encore WireGuard Tunnel: " + name,
		SidType:      windows.SERVICE_SID_TYPE_UNRESTRICTED,
	}
	service, err = m.CreateService(serviceName, path, config, "vpn", "svc-run", configPath)
	if err != nil {
		return err
	}

	err = service.Start()
	// go trackTunnelService(name, service) // Pass off reference to handle.
	return err
}

func Uninstall(name string) error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	serviceName, err := services.ServiceNameOfTunnel(name)
	if err != nil {
		return err
	}
	service, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	service.Control(svc.Stop)
	err = service.Delete()
	err2 := service.Close()
	if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
		return err
	}
	return err2
}

func Run(confPath string) error {
	return tunnel.Run(confPath)
}

func ShellEscape(arg string) string {
	if strings.ContainsAny(arg, `" `) {
		arg = "\"" + strings.Replace(arg, "\"", "\"\"", -1) + "\""
	}
	return arg
}
