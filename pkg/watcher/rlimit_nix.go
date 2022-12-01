//go:build linux || darwin

package watcher

import (
	"syscall"

	"github.com/rs/zerolog/log"
)

// BumpRLimitSoftToHardLimit bumps the soft limit of the rlimit to the hard limit.
//
// To go higher the user will need to update their kernel settings which can be viewed:
//
//	sysctl -a | grep kern.maxfile
func BumpRLimitSoftToHardLimit() {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Error().Err(err).Msg("failed to get rlimit")
	}
	rLimit.Cur = rLimit.Max
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Error().Err(err).Msg("failed to set rlimit")
	}
}
