package experiments

// Name is the name of an experiment
type Name string

const (
	/* Current experiments are listed here */

	// LocalSecretsOverride is an experiment to allow for secrets
	// to be overridden with values from a ".secrets.local" file.
	LocalSecretsOverride Name = "local-secrets-override"

	// Metrics is an experiment to enable metrics.
	Metrics Name = "metrics"

	// V2 enables the new parser and compiler.
	V2 Name = "v2"

	// BetaRuntime enables the beta runtime.
	BetaRuntime Name = "beta-runtime"

	// ExternalCalls forces all RPC calls to be made externally (over HTTP/GRPC).
	// instead of routing them internally (via the runtime).
	ExternalCalls Name = "external-calls"

	// AuthDataRoundTrip forces auth data to be round-tripped through the wireformat
	// when internal API calls are made.
	AuthDataRoundTrip Name = "auth-data-round-trip"
)

// Valid reports whether the given name is a known experiment.
func (x Name) Valid() bool {
	switch x {
	case LocalSecretsOverride,
		Metrics,
		V2,
		BetaRuntime,
		ExternalCalls,
		AuthDataRoundTrip:
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
	_, ok := set.enabled[x]
	return ok
}
