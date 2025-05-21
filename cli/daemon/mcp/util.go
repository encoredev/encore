package mcp

import (
	"time"

	metav1 "encr.dev/proto/encore/parser/meta/v1"
)

// findServiceNameForPackage returns the service name for a given package path
func findServiceNameForPackage(md *metav1.Data, pkgPath string) string {
	for _, pkg := range md.Pkgs {
		if pkg.RelPath == pkgPath && pkg.ServiceName != "" {
			return pkg.ServiceName
		}
	}
	return ""
}

// formatDuration formats a nanosecond duration into a human-readable string
func formatDuration(nanos int64) string {
	duration := time.Duration(nanos) * time.Nanosecond
	return duration.String()
}
