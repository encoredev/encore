package encore

import (
	"net/url"
	"time"

	"encore.dev/runtime/config"
)

// Meta returns metadata about the running application.
//
// Meta will never return nil.
func Meta() *AppMetadata {
	return &AppMetadata{
		AppID:      config.Cfg.Runtime.AppSlug,
		ApiBaseUrl: config.Cfg.Runtime.APIBaseURL,
		Environment: EnvironmentMeta{
			Name:  config.Cfg.Runtime.EnvName,
			Type:  EnvironmentType(config.Cfg.Runtime.EnvType),
			Cloud: CloudProvider(config.Cfg.Runtime.EnvCloud),
		},
		Deployment: DeploymentMeta{
			Revision:           config.Cfg.Static.AppCommit.Revision,
			UncommittedChanges: config.Cfg.Static.AppCommit.Uncommitted,
			DeploymentID:       config.Cfg.Runtime.DeployID,
			Deployed:           config.Cfg.Runtime.Deployed,
		},
	}
}

// AppMetadata contains metadata about the running Encore application.
type AppMetadata struct {
	// The application ID, if the application is not linked to the Encore platform this will be an empty string.
	//
	// To link to the Encore platform run `encore app link` from your terminal in the root directory of the Encore app.
	AppID string

	// The base URL which can be used to call the API of this running application
	//
	// For local development it is "http://localhost:<port>", typically "http://localhost:4000".
	//
	// If a custom domain is used for this environment it is returned here, but note that
	// changes only take effect at the time of deployment while custom domains can be updated at any time.
	ApiBaseUrl url.URL

	// Information about the environment the app is running in
	Environment EnvironmentMeta

	// Information about the running build
	Deployment DeploymentMeta
}

type EnvironmentMeta struct {
	// The name of environment that this application.
	// For local development it is "local".
	Name string

	// The type of environment is this application running in
	// For local development this will be EnvLocal
	Type EnvironmentType

	// The cloud that this environment is running on
	// For local development this is CloudLocal
	Cloud CloudProvider
}

type DeploymentMeta struct {
	// The git commit that formed the base of this build.
	Revision string

	// true if there were uncommitted changes on top of the Commit.
	UncommittedChanges bool

	// The deployment ID created by the Encore Platform
	DeploymentID string

	// The time the Encore Platform deployed this build to the environment
	Deployed time.Time
}

// EnvironmentType represents the type of environment.
//
// For more information on environment types see https://encore.dev/docs/deploy/environments#environment-types
//
// Additional environment types may be added in the future.
type EnvironmentType string

const (
	// EnvProduction represents a production environment.
	EnvProduction EnvironmentType = "production"

	// EnvDevelopment represents a long-lived cloud-hosted, non-production environment, such as test environments.
	EnvDevelopment EnvironmentType = "development"

	// EnvEphemeral represents short-lived cloud-hosted, non-production environments, such as preview environments
	// that only exist while a particular pull request is open.
	EnvEphemeral EnvironmentType = "ephemeral"

	// EnvLocal represents the local development environment when using 'encore run'.
	EnvLocal EnvironmentType = "local"

	// EnvUnitTest represents when code is being run from 'encore test'.
	// It will never be used for long-running processes which can serve API requests.
	EnvUnitTest EnvironmentType = "unit-test"
)

// CloudProvider represents the cloud provider this application is running in.
//
// For more information about how Cloud Providers work with Encore, see https://encore.dev/docs/deploy/own-cloud
//
// Additional cloud providers may be added in the future.
type CloudProvider string

const (
	CloudAWS   CloudProvider = "aws"
	CloudGCP   CloudProvider = "gcp"
	CloudAzure CloudProvider = "azure"

	// EncoreCloud is Encore's own cloud offering, and the default provider for new Environments.
	// It is not recommended to use this cloud for production systems.
	EncoreCloud CloudProvider = "encore-cloud"

	// CloudLocal is used when an application is running from the Encore CLI by using either
	// 'encore run' or 'encore test'
	CloudLocal CloudProvider = "local"
)
