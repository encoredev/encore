//go:build !encore_local

package app

import (
	"net"
	"os"
	"strconv"
)

func Listen() (net.Listener, error) {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}
	return net.Listen("tcp", ":"+strconv.Itoa(port))
}
