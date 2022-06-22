//go:build !encore_local

package runtime

import (
	"net"
	"os"
	"strconv"
)

var devMode = false

func listen() (net.Listener, error) {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}
	return net.Listen("tcp", ":"+strconv.Itoa(port))
}
