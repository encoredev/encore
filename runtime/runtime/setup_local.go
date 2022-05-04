//go:build encore_local && encore_internal

package runtime

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/yamux"
)

var devMode = true

func listen() (net.Listener, error) {
	var in, out *os.File
	if runtime.GOOS == "windows" {
		extraFiles := os.Getenv("ENCORE_EXTRA_FILES")
		fds := strings.Split(extraFiles, ",")
		if len(fds) < 2 {
			return nil, fmt.Errorf("could not get request/response file descriptors: %q", extraFiles)
		}
		infd, err1 := strconv.Atoi(fds[0])
		outfd, err2 := strconv.Atoi(fds[1])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("could not parse request/response file descriptors: %q", extraFiles)
		}
		in = os.NewFile(uintptr(infd), "encore-stdin")
		out = os.NewFile(uintptr(outfd), "encore-stdout")
	} else {
		in = os.NewFile(uintptr(3), "encore-stdin")
		out = os.NewFile(uintptr(4), "encore-stdout")
	}

	rwc := struct {
		io.Reader
		io.WriteCloser
	}{
		Reader:      in,
		WriteCloser: out,
	}

	return yamux.Server(rwc, yamux.DefaultConfig())
}

// configureZerologOutput configures the zerolog logger's output format.
func configureZerologOutput() {
	// Use default settings for local development.
}
