package app

import (
	"net"
	"net/netip"
	"os"
	"strconv"

	"encore.dev/appruntime/shared/encoreenv"
)

func Listen() (net.Listener, error) {
	listenAddr := encoreenv.Get("ENCORE_LISTEN_ADDR")
	if listenAddr != "" {
		addrPort, err := netip.ParseAddrPort(listenAddr)
		if err != nil {
			return nil, err
		}
		return net.Listen("tcp", addrPort.String())
	}

	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}
	return net.Listen("tcp", ":"+strconv.Itoa(port))
}
