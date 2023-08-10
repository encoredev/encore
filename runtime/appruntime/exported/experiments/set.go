package experiments

import (
	"slices"

	"encore.dev/appruntime/exported/config"
)

// Set is a set of experiments enabled within this app
type Set struct {
	enabled map[Name]struct{}
}

// FromConfig constructs a new Experiments object from both the static and runtime configs.
//
// It is used by the Encore runtime to construct the Experiments set in a running application.
//
// Unknown experiments are ignored.
func FromConfig(static *config.Static, runtime *config.Runtime) *Set {
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
		for _, exp := range runtime.DynamicExperiments {
			e.enabled[Name(exp)] = struct{}{}
		}
	}

	return e
}

// List returns a list of all experiments enabled in this set.
func (s *Set) List() []Name {
	if s == nil {
		return nil
	}
	names := make([]Name, 0, len(s.enabled))
	for key := range s.enabled {
		names = append(names, key)
	}
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
