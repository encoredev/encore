// Package cloud contains some helpers for referring to different cloud implementations
package cloud

// Provider represents the cloud provider this application is running in.
//
// Additional cloud providers may be added in the future.
type Provider = string

const (
	AWS   Provider = "aws"
	GCP   Provider = "gcp"
	Azure Provider = "azure"

	// Encore is Encore's own cloud offering, and the default provider for new Environments.
	Encore Provider = "encore"

	// Local is used when an application is running from the Encore CLI by using either 'encore run' or 'encore test'
	Local Provider = "local"
)
