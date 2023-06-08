package experiments

import (
	"os"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"encore.dev/appruntime/exported/config"
)

// Set is a set of experiments enabled within this app
type Set struct {
	enabled map[Name]struct{}
}

// NewForRuntime constructs a new Experiments object from both the static and runtime configs.
//
// It is used by the Encore runtime to construct the Experiments set in a running application.
//
// Unknown experiments are ignored.
func NewForRuntime(static *config.Static, runtime *config.Runtime) *Set {
	e := &Set{make(map[Name]struct{})}

	// Note we don't check for valid experiments here, because the static and runtime configs
	// are already validated by the compiler, and from the platform side
	// we might have enabled experiments that are not yet known to the compiler which this
	// binary was compiled with.
	if static != nil {
		for _, exp := range static.EnabledExperiments {
			e.enabled[Name(exp)] = struct{}{}
		}
	}

	if runtime != nil {
		for _, exp := range runtime.EnabledExperiments {
			e.enabled[Name(exp)] = struct{}{}
		}
	}

	return e
}

// NewForCompiler creates an experiment set which represents the enabled experiments
// within a particular run of Encore.
//
// All errors reported by NewSet are due to unknown experiment names.
// The error type is of type *UnknownExperimentError.
func NewForCompiler(fromAppFile []Name, environ []string) (*Set, error) {
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

// List returns a list of all experiments enabled in this set.
func (s *Set) List() []Name {
	if s == nil {
		return nil
	}
	names := maps.Keys(s.enabled)
	slices.Sort(names)
	return names
}

// StringList returns a list of all experiments enabled in this set.
func (s *Set) StringList() []string {
	names := s.List()
	rtn := make([]string, len(names))
	for i, n := range names {
		rtn[i] = string(n)
	}
	return rtn
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
