package gql

import (
	"time"

	"github.com/modern-go/reflect2"
)

type Secret struct {
	Key    string
	Groups []*SecretGroup
}

type SecretGroup struct {
	ID          string
	Key         string
	Selector    []SecretSelector
	Description string
	Etag        string
	ArchivedAt  *time.Time
}

type SecretSelector interface {
	secretSelector()
	String() string
}

type SecretSelectorEnvType struct {
	Kind string
}

func (SecretSelectorEnvType) secretSelector()   {}
func (s *SecretSelectorEnvType) String() string { return "type:" + s.Kind }

type SecretSelectorSpecificEnv struct {
	Env *Env
}

func (s *SecretSelectorSpecificEnv) String() string { return "id:" + s.Env.ID }
func (SecretSelectorSpecificEnv) secretSelector()   {}

type ConflictError struct {
	AppID     string
	Key       string
	Conflicts []GroupConflict
}

type GroupConflict struct {
	GroupID   string
	Conflicts []string
}

// TypeRegistry contains all the types that are used in the graphql schema,
// in order to ensure they are not dead-code eliminated.
var TypeRegistry = []reflect2.Type{
	reflect2.TypeOf((*SecretSelectorEnvType)(nil)),
	reflect2.TypeOf((*SecretSelectorSpecificEnv)(nil)),
}
