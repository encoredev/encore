package ecauth

import (
	"encore.dev/appruntime/exported/config"
)

// Key is a MAC key for authenticating communication between
// an Encore app and the Encore Platform. It is designed to be
// JSON marshalable, but as it contains secret material care
// must be taken when using it.
type Key = config.EncoreAuthKey

type Payload interface {
	// DeterministicBytes returns a deterministic byte slice that represents the payload.
	DeterministicBytes() []byte
}

// BytesPayload is a payload that is represented by a byte slice.
type BytesPayload []byte

func (b BytesPayload) DeterministicBytes() []byte {
	return b
}
