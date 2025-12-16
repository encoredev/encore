package run

import (
	"encoding/hex"
	"regexp"

	"golang.org/x/crypto/sha3"
)

var nsqNameRegex = regexp.MustCompile(`^[\.a-zA-Z0-9_-]+(#ephemeral)?$`)

// isValidNSQName checks if a name is valid according to NSQ requirements:
// - Must match pattern: ^[\.a-zA-Z0-9_-]+(#ephemeral)?$
// - Must be between 1 and 64 characters
func isValidNSQName(name string) bool {
	return len(name) >= 1 && len(name) <= 64 && nsqNameRegex.MatchString(name)
}

// hashNSQName creates a valid NSQ name by hashing the input.
// The hash is a SHA3-256 hash encoded as hex (64 characters).
func hashNSQName(name string) string {
	hash := sha3.Sum256([]byte(name))
	return hex.EncodeToString(hash[:])
}

// ensureValidNSQName returns the name if it's valid, otherwise returns a hashed version.
func ensureValidNSQName(name string) string {
	if isValidNSQName(name) {
		return name
	}
	return hashNSQName(name)
}
