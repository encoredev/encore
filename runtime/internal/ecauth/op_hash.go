package ecauth

import (
	"crypto/hmac"
	"encoding/binary"
	"encoding/hex"

	jsoniter "github.com/json-iterator/go"
	"golang.org/x/crypto/sha3"

	"encore.dev/beta/errs"
)

var json = jsoniter.Config{SortMapKeys: true}.Froze()

type ObjectType string

const (
	PubsubMsg ObjectType = "pubsub-msg"
)

type ActionType string

const (
	Create ActionType = "create"
	Read   ActionType = "read"
	Update ActionType = "update"
	Delete ActionType = "delete"
)

// An OperationHash is a hash that is used to verify that an operation is allowed.
type OperationHash string

// NewOperationHash creates a new operation hash.
//
// An operation hash is the result of combining the object type and action type
// Additional context can be added to the hash by passing in additional byte slices.
func NewOperationHash(object ObjectType, action ActionType, payload Payload, additionalContext ...[]byte) (OperationHash, error) {
	hash := sha3.New256()

	switch {
	case object == "":
		return "", errs.B().Code(errs.InvalidArgument).Msg("object is required").Err()
	case action == "":
		return "", errs.B().Code(errs.InvalidArgument).Msg("action is required").Err()
	}

	hash.Write([]byte(object))
	hash.Write([]byte{0})
	hash.Write([]byte(action))

	// If there is a payload, add it to the hash.
	if payload != nil {
		payloadBytes := payload.DeterministicBytes()

		hash.Write([]byte{0})
		hash.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(payloadBytes))))
		hash.Write(payloadBytes)
	}

	// Add additional context to the hash.
	for _, c := range additionalContext {
		hash.Write([]byte{0})
		hash.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(c))))
		hash.Write(c)
	}

	hashBytes := hash.Sum(nil)

	return OperationHash(hex.EncodeToString(hashBytes)), nil
}

// Verify verifies that the operation hash matches the given object and action.
func (h OperationHash) Verify(object ObjectType, action ActionType, payload Payload, additionalContext ...[]byte) (bool, error) {
	hash, err := NewOperationHash(object, action, payload, additionalContext...)
	if err != nil {
		return false, err
	}

	return hmac.Equal([]byte(h), []byte(hash)), nil
}

// HashString returns the hex encoded hash
func (h OperationHash) HashString() string {
	return string(h)
}
