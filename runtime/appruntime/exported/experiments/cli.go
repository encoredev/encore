//go:build !encore_app

// Note this file is only included by the CLI and not by the app runtime.

package experiments

import (
	"os"
	"strings"
)

// FromAppFileAndEnviron creates an experiment set which represents the enabled experiments
// within a particular run of Encore.
//
// All errors reported by NewSet are due to unknown experiment names.
// The error type is of type *UnknownExperimentError.
func FromAppFileAndEnviron(fromAppFile []Name, environ []string) (*Set, error) {
	const envName = "ENCORE_EXPERIMENT"

	set := &Set{make(map[Name]struct{})}

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
		s.enabled[key] = struct{}{}
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
