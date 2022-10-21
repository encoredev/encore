package experiment

import (
	"os"
	"strings"
)

const envName = "ENCORE_EXPERIMENT"

type Experiment string

const (
	// None means the feature is not experimental and is always enabled
	// It is a special case that always exists
	//
	// We use an empty string, so the zero value of an experiment is None
	None Experiment = ""

	/* Current experiments would be listed here */
)

// Enabled returns true if this experiment enabled in the given set
func (x Experiment) Enabled(set *Set) bool {
	if x == None {
		// Released experiments are always enabled
		return true
	}
	if set == nil {
		// If there's no set, then it's not enabled
		return false
	}

	// Does the release set contain this?
	return set.experiments[x]
}

type Set struct {
	experiments map[Experiment]bool
}

// NewSet creates an experiment set which represents the enabled experiments
// within a particular run of encore
func NewSet(fromAppFile map[string]bool, environ []string) *Set {
	set := &Set{make(map[Experiment]bool)}

	// Add experiments enabled in the app file
	for key, value := range fromAppFile {
		if value {
			set.add(key)
		}
	}

	// Grab experiments from the environmental variables of this process
	fields := strings.Fields(os.Getenv(envName))
	for _, f := range fields {
		set.add(f)
	}

	// Grab experiments from the environmental variables of the caller
	for _, env := range environ {
		if strings.HasPrefix(env, envName+"=") {
			env = strings.TrimPrefix(env, envName+"=")
			env = strings.TrimPrefix(env, "\"")
			env = strings.TrimSuffix(env, "\"")

			fields := strings.Fields(env)
			for _, f := range fields {
				set.add(f)
			}
		}
	}

	return set
}

func (s *Set) add(key string) {
	key = strings.ToUpper(strings.TrimSpace(key))
	if key != "" {
		s.experiments[Experiment(key)] = true
	}
}
