//go:build !linux && !darwin

package watcher

// BumpRLimitSoftToHardLimit bumps the soft limit of the rlimit to the hard limit.
//
// To go higher the user will need to update their kernel settings which can be viewed:
//
//	sysctl -a | grep kern.maxfile
func BumpRLimitSoftToHardLimit() {
	// no-op
}
