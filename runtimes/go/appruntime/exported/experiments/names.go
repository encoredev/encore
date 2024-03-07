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

	// LocalMultiProcess forces each Encore service to run as it's own independent process locally
	// without being able to share memory which emulates the behaviour that will be seen in production
	// in a multi process setup.
	LocalMultiProcess Name = "local-multi-process"

	// AuthDataRoundTrip forces auth data to be round-tripped through the wireformat
	// when internal API calls are made.
	AuthDataRoundTrip Name = "auth-data-round-trip"

	// TypeScript enables building the app with TypeScript support.
	TypeScript Name = "typescript"

	// StreamTraces enables streaming traces to the Encore platform as they're happening,
	// as opposed to waiting for the request to finish before starting the upload.
	StreamTraces Name = "stream-traces"
)

// Valid reports whether the given name is a known experiment.
func (x Name) Valid() bool {
	switch x {
	case LocalSecretsOverride,
		Metrics,
		V2,
		BetaRuntime,
		LocalMultiProcess,
		AuthDataRoundTrip,
		TypeScript,
		StreamTraces:
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
