package encore

import (
	"net/url"
	"time"

	"encore.dev/internal/cloud"
	"encore.dev/runtime/config"
)

var apiBaseURL *url.URL

// Meta returns metadata about the running application.
//
// Meta will never return nil.
func Meta() *AppMetadata {

	return &AppMetadata{
		AppID:      config.Cfg.Runtime.AppSlug,
		APIBaseURL: *apiBaseURL,
		Environment: EnvironmentMeta{
			Name:  config.Cfg.Runtime.EnvName,
			Type:  EnvironmentType(config.Cfg.Runtime.EnvType),
			Cloud: CloudProvider(config.Cfg.Runtime.EnvCloud),
		},
		Build: BuildMeta{
			Revision:           config.Cfg.Static.AppCommit.Revision,
			UncommittedChanges: config.Cfg.Static.AppCommit.Uncommitted,
		},
		Deploy: DeployMeta{
			ID:   config.Cfg.Runtime.DeployID,
			Time: config.Cfg.Runtime.DeployedAt,
		},
	}
}

func init() {
	// If this package is imported outside an Encore application, these will be nil, as we will not have initialised
	// or a dummy `config.LoadConfig` would have been linked in which will not return a config
	if config.Cfg != nil && config.Cfg.Runtime != nil {
		// We don't need to check for errors, as we already check it's parsable during the config.ParseConfig initialization
		apiBaseURL, _ = url.Parse(config.Cfg.Runtime.APIBaseURL)
	}
}

// AppMetadata contains metadata about the running Encore application.
type AppMetadata struct {
	// The application ID, if the application is not linked to the Encore platform this will be an empty string.
	//
	// To link to the Encore platform run `encore app link` from your terminal in the root directory of the Encore app.
	AppID string

	// The base URL which can be used to call the API of this running application.
	//
	// For local development it is "http://localhost:<port>", typically "http://localhost:4000".
	//
	// If a custom domain is used for this environment it is returned here, but note that
	// changes only take effect at the time of deployment while custom domains can be updated at any time.
	APIBaseURL url.URL

	// Information about the environment the app is running in.
	Environment EnvironmentMeta

	// Information about the running binary itself.
	Build BuildMeta

	// Information about this deployment of the binary
	Deploy DeployMeta
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

type BuildMeta struct {
	// The git commit that formed the base of this build.
	Revision string

	// true if there were uncommitted changes on top of the Commit.
	UncommittedChanges bool
}

type DeployMeta struct {
	// The deployment ID created by the Encore Platform.
	ID string

	// The time the Encore Platform deployed this build to the environment.
	Time time.Time
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

	// EnvLocal represents the local development environment when using 'encore run' or `encore test`.
	EnvLocal EnvironmentType = "local"
)

// CloudProvider represents the cloud provider this application is running in.
//
// For more information about how Cloud Providers work with Encore, see https://encore.dev/docs/deploy/own-cloud
//
// Additional cloud providers may be added in the future.
type CloudProvider = cloud.CloudProvider

const (
	CloudAWS   CloudProvider = cloud.AWS
	CloudGCP   CloudProvider = cloud.GCP
	CloudAzure CloudProvider = cloud.Azure

	// EncoreCloud is Encore's own cloud offering, and the default provider for new Environments.
	EncoreCloud CloudProvider = cloud.Encore

	// CloudLocal is used when an application is running from the Encore CLI by using either
	// 'encore run' or 'encore test'
	CloudLocal CloudProvider = cloud.Local
)
