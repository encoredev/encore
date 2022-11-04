package experiments

import (
	"os"
	"strings"

	"golang.org/x/exp/slices"
)

const envName = "ENCORE_EXPERIMENT"

type Name string

const (
	/* Current experiments are listed here */

	// LocalSecretsOverride is an experiment to allow for secrets
	// to be overridden with values from a ".secrets.local" file.
	LocalSecretsOverride Name = "local-secrets-override"
)

// Valid reports whether the given name is a known experiment.
func (x Name) Valid() bool {
	switch x {
	case LocalSecretsOverride:
		return true
	default:
		return false
	}
}

// Enabled returns true if this experiment enabled in the given set
func (x Name) Enabled(set *Set) bool {
	if set == nil {
		// If there's no set, then it's not enabled
		return false
	}

	// Does the release set contain this?
	return set.experiments[x]
}

type Set struct {
	experiments map[Name]bool
}

func (s *Set) List() []Name {
	var ret []Name
	for k := range s.experiments {
		ret = append(ret, k)
	}
	slices.Sort(ret)
	return ret
}

// NewSet creates an experiment set which represents the enabled experiments
// within a particular run of Encore.
//
// All errors reported by NewSet are due to unknown experiment names.
// The error type is of type *UnknownExperimentError.
func NewSet(fromAppFile []Name, environ []string) (*Set, error) {
	set := &Set{make(map[Name]bool)}

	// Add experiments enabled in the app file
	if err := set.add(fromAppFile...); err != nil {
		return nil, err
	}

	// Grab experiments from the environmental variables of this process.
	if val := os.Getenv(envName); val != "" {
		if err := set.add(parseEnvVal(val)...); err != nil {
			return nil, err
		}
	}

	// Grab experiments from the environmental variables of the caller
	const prefix = envName + "="
	for _, env := range environ {
		if strings.HasPrefix(env, prefix) {
			val := env[len(prefix):]
			if err := set.add(parseEnvVal(val)...); err != nil {
				return nil, err
			}
		}
	}

	return set, nil
}

func (s *Set) add(keys ...Name) error {
	for _, key := range keys {
		if key == "" {
			continue
		}

		if !key.Valid() {
			return &UnknownExperimentError{key}
		}
		s.experiments[key] = true
	}
	return nil
}

func parseEnvVal(val string) []Name {
	if val == "" {
		return nil
	}

	val = strings.Trim(val, `"'`)
	strs := strings.Split(val, ",")
	names := make([]Name, len(strs))
	for i, s := range strs {
		names[i] = Name(s)
	}
	return names
}

// UnknownExperimentError is an error returned when an app tries to use
// an experiment that is not known to the current version of Encore.
type UnknownExperimentError struct {
	Name Name
}

func (e *UnknownExperimentError) Error() string {
	return "unknown experiment: " + string(e.Name)
}
