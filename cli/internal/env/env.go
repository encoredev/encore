// Package env answers where Encore tools and resources are located.
package env

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rs/zerolog/log"
)

// EncoreRuntimePath reports the path to the Encore runtime.
// It can be overridden by setting ENCORE_RUNTIME_PATH.
func EncoreRuntimePath() string {
	if p := os.Getenv("ENCORE_RUNTIME_PATH"); p != "" {
		return p
	}
	r := encoreRoot
	if r == "" {
		fmt.Fprintln(os.Stderr, "fatal: encore was compiled without "+
			"specifying the location to the Encore install root.\n"+
			"You can specify the path to the Encore runtime manually by setting the ENCORE_RUNTIME_PATH environment variable.")
		os.Exit(1)
	}
	return filepath.Join(encoreRoot, "runtime")
}

// EncoreGoRoot reports the path to the Encore Go root.
// It can be overridden by setting ENCORE_GOROOT.
func EncoreGoRoot() string {
	if p := os.Getenv("ENCORE_GOROOT"); p != "" {
		return p
	}
	r := encoreRoot
	if r == "" {
		fmt.Fprintln(os.Stderr, "fatal: encore was compiled without "+
			"specifying the location to the Encore install root.\n"+
			"You can specify the path to the Encore GOROOT manually by setting the ENCORE_GOROOT environment variable.")
		os.Exit(1)
	}
	return filepath.Join(encoreRoot, "encore-go")
}

// encoreRoot is the compiled-in path to the encore root.
// It is set using go build -ldflags "-X 'encr.dev/cli/internal/env.encoreRoot=/path/to/encore'".
var encoreRoot string

// Exe reports the location of the Encore-provided exe.
func Exe(elem ...string) string {
	elem = append([]string{resourceDir}, elem...)
	path := filepath.Join(elem...)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	return path
}

var resourceDir = (func() string {
	switch runtime.GOOS {
	case "windows":
		return "C:\\Program Files\\Encore"
	case "darwin", "linux":
		return "/usr/local/encore"
	default:
		log.Fatal().Str("goos", runtime.GOOS).Msg("unsupported GOOS")
		panic("unreachable")
	}
})()
