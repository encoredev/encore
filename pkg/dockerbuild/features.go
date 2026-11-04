package dockerbuild

type FeatureFlag string

const (
	NewRuntimeConfig FeatureFlag = "new_runtime_config"

	// SQLGCPIAMAuth reports that the image's runtime can log in to a database
	// with Cloud SQL's IAM database authentication, as the runtime config's
	// SQL roles ask with SQL_AUTH_METHOD_GCP_IAM. A runtime without it reads
	// such a role as one with no password.
	SQLGCPIAMAuth FeatureFlag = "sql_gcp_iam_auth"
)
