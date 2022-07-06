// Package cloud contains some helpers for referring to different cloud implementations
package cloud

// CloudProvider represents the cloud provider this application is running in.
//
// Additional cloud providers may be added in the future.
type CloudProvider = string

const (
	AWS   CloudProvider = "aws"
	GCP   CloudProvider = "gcp"
	Azure CloudProvider = "azure"

	// Encore is Encore's own cloud offering, and the default provider for new Environments.
	Encore CloudProvider = "encore"

	// Local is used when an application is running from the Encore CLI by using either 'encore run' or 'encore test'
	Local CloudProvider = "local"
)
