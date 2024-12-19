package config

import (
	"fmt"
	"time"

	"go.encore.dev/platform-sdk/pkg/auth"
)

type Static struct {
	// The version of Encore which the app was compiled with.
	// This is string is for informational use only, and it's format should not be relied on.
	EncoreCompiler string
	AppCommit      CommitInfo // The commit which this service was built from

	CORSAllowHeaders  []string // Headers to be allowed by cors
	CORSExposeHeaders []string // Headers to be exposed by cors
	PubsubTopics      map[string]*StaticPubsubTopic

	Testing         bool
	TestServiceMap  map[string]string // map of service names to their filesystem root
	TestAppRootPath string            // the root path of the app when running tests

	// PrettyPrintLogs indicates whether logs should be pretty-printed.
	// It's set when building a separate test binary, for example.
	PrettyPrintLogs bool

	// BundledServices are the services bundled in this binary.
	BundledServices []string

	// EnabledExperiments is a list of experiments that are enabled for this app
	// which where enabled at compile time.
	EnabledExperiments []string `json:"experiments,omitempty"`

	// EmbeddedEnvs is a set of embedded environment variables.
	EmbeddedEnvs map[string]string
}

type Runtime struct {
	AppID             string          `json:"app_id"`
	AppSlug           string          `json:"app_slug"`
	APIBaseURL        string          `json:"api_base_url"`
	EnvID             string          `json:"env_id"`
	EnvName           string          `json:"env_name"`
	EnvType           string          `json:"env_type"`
	EnvCloud          string          `json:"env_cloud"`
	DeployID          string          `json:"deploy_id"` // Overridden by ENCORE_DEPLOY_ID env var if set
	DeployedAt        time.Time       `json:"deploy_time"`
	TraceEndpoint     string          `json:"trace_endpoint,omitempty"`
	TraceSamplingRate *float64        `json:"trace_sampling_rate,omitempty"`
	AuthKeys          []EncoreAuthKey `json:"auth_keys,omitempty"`
	CORS              *CORS           `json:"cors,omitempty"`
	EncoreCloudAPI    *EncoreCloudAPI `json:"ec_api,omitempty"` // If nil, the app is not running in Encore Cloud

	SQLDatabases     []*SQLDatabase          `json:"sql_databases,omitempty"`
	SQLServers       []*SQLServer            `json:"sql_servers,omitempty"`
	PubsubProviders  []*PubsubProvider       `json:"pubsub_providers,omitempty"`
	PubsubTopics     map[string]*PubsubTopic `json:"pubsub_topics,omitempty"`
	RedisServers     []*RedisServer          `json:"redis_servers,omitempty"`
	RedisDatabases   []*RedisDatabase        `json:"redis_databases,omitempty"`
	BucketProviders  []*BucketProvider       `json:"bucket_providers,omitempty"`
	Buckets          map[string]*Bucket      `json:"buckets,omitempty"`
	Metrics          *Metrics                `json:"metrics,omitempty"`
	Gateways         []Gateway               `json:"gateways,omitempty"`          // Gateways defines the gateways which should be served by the container
	HostedServices   []string                `json:"hosted_services,omitempty"`   // List of services to be hosted within this container (zero length means all services, unless there's a gateway running)
	ServiceDiscovery map[string]Service      `json:"service_discovery,omitempty"` // ServiceDiscovery lists where all the services are being hosted if not in this container

	// ServiceAuth defines which authentication method can be used
	// when talking to this runtime for internal service-to-service
	// calls.
	//
	// An empty slice means that no service-to-service calls can be made
	ServiceAuth []ServiceAuth `json:"service_auth,omitempty"`

	// ShutdownTimeout is the duration before non-graceful shutdown is initiated,
	// meaning connections are closed even if outstanding requests are still in flight.
	// If zero, it shuts down immediately.
	//
	// Deprecated: Use GracefulShutdown.Total instead.
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// GracefulShutdown defines the timings for the graceful shutdown process.
	GracefulShutdown *GracefulShutdownTimings `json:"graceful_shutdown,omitempty"`

	// DynamicExperiments is a list of experiments that are enabled for this app
	// which impact runtime behaviour, but which were not enabled at compile time.
	//
	// Experiments which impact compilation should be handled by the compiler
	// and added to the static config.
	DynamicExperiments []string `json:"dynamic_experiments,omitempty"`

	// Log configuration to set for the application.
	// If empty it defaults to "trace".
	LogConfig string `json:"log_config"`
}

// GracefulShutdownTimings defines the timings for the graceful shutdown process.
type GracefulShutdownTimings struct {
	// Total is how long we allow the total shutdown to take
	// before we simply kill the process using [os.Exit]
	//
	// If not set, it will default to [Runtime.ShutdownTimeout] during
	// the migration period to this config.
	//
	// If [Runtime.ShutdownTimeout] is also not set, it will default to
	// 500ms.
	Total *time.Duration `json:"total,omitempty"`

	// ShutdownHooks is how long before [Total] runs out that we cancel
	// the context that is passed to the shutdown hooks.
	//
	// If not set, it will default to 1 second.
	//
	// It is expected that ShutdownHooks is a larger value than Handlers.
	ShutdownHooks *time.Duration `json:"shutdown_hooks,omitempty"`

	// Handlers is how long before [Total] runs out that we cancel
	// the context that is passed to API and PubSub Subscription handlers.
	//
	// If not set, it will default to 1 second.
	//
	// For example, if [Total] is 10 seconds and [Handlers] is 2 seconds,
	// then we will cancel the context passed to handlers 8 seconds after
	// a graceful shutdown is initiated.
	Handlers *time.Duration `json:"handlers,omitempty"`
}

// Gateway defines the configuration of a gateway which should be served
// by the container
type Gateway struct {
	// Name is the name of the gateway
	Name string `json:"name"`
	// Host is the hostname of the gateway
	Host string `json:"host"`
}

// Service defines the service discovery configuration for a service
type Service struct {
	// Name is the name of the service
	Name string `json:"name"`
	// URL is the base URL of the service (including protocol and port)
	URL string `json:"url"`
	// Protocol is the protocol that the service talks
	Protocol SvcProtocol `json:"protocol"`

	// ServiceAuth is the authentication configuration required for
	// internal service to service calls being made to this service.
	ServiceAuth ServiceAuth `json:"service_auth"`
}

type SvcProtocol string

const (
	Http SvcProtocol = "http"
)

type ServiceAuth struct {
	// Method is the name of the authentication method.
	Method string `json:"method"`
}

// UnsafeAllOriginWithCredentials can be used to specify that all origins are
// allowed to call this API with credentials. It is unsafe and misuse can lead
// to security issues. Only use if you know what you're doing.
const UnsafeAllOriginWithCredentials = "UNSAFE_ALL_ORIGINS_WITH_CREDENTIALS"

type CORS struct {
	// Debug enables debug logging of all requests passing through the CORS system
	Debug bool `json:"debug"`

	// DisableCredentials, if true, causes Encore to respond to OPTIONS requests
	// without setting Access-Control-Allow-Credentials: true.
	DisableCredentials bool `json:"disable_credentials,omitempty"`

	// AllowOriginsWithCredentials specifies the allowed origins for requests
	// that include credentials. If a request is made from an Origin in this list
	// Encore responds with Access-Control-Allow-Origin: <Origin>.
	// If DisableCredentials is true this field is not used.
	//
	// The URLs in this list may include wildcards (e.g. "https://*.example.com"
	// or "https://*-myapp.example.com").
	AllowOriginsWithCredentials []string `json:"allow_origins_with_credentials,omitempty"`

	// AllowOriginsWithoutCredentials specifies the allowed origins for requests
	// that don't include credentials. If nil it defaults to allowing all domains
	// (equivalent to []string{"*"}).
	AllowOriginsWithoutCredentials []string `json:"allow_origins_without_credentials,omitempty"`

	// ExtraAllowedHeaders specifies extra headers to allow, beyond
	// the default set always recognized by Encore.
	// As a special case, if the list contains "*" all headers are allowed.
	ExtraAllowedHeaders []string `json:"raw_allowed_headers,omitempty"`

	// ExtraExposedHeaders specifies extra headers to expose, beyond
	// the default set always recognized by Encore.
	// As a special case, if the list contains "*" all headers are allowed.
	ExtraExposedHeaders []string `json:"raw_exposed_headers,omitempty"`

	// AllowAccessWhenOnPrivateNetwork, if true, allows requests to Encore apps running
	// on private networks from websites.
	//
	// See: https://wicg.github.io/private-network-access/
	AllowPrivateNetworkAccess bool `json:"allow_private_network_access,omitempty"`
}

type CommitInfo struct {
	Revision    string `json:"revision"`
	Uncommitted bool   `json:"uncommitted"`
}

func (ci CommitInfo) AsRevisionString() string {
	if ci.Uncommitted {
		return fmt.Sprintf("%s-modified", ci.Revision)
	}
	return ci.Revision
}

func (r *Runtime) Copy() *Runtime {
	cfg := *r
	cfg.AuthKeys = make([]EncoreAuthKey, len(r.AuthKeys))
	for i, authKey := range r.AuthKeys {
		cfg.AuthKeys[i] = authKey.Copy()
	}
	copy(cfg.SQLDatabases, r.SQLDatabases)

	return &cfg
}

// EncoreCloudAPI is the configuration for the Encore Cloud API
// which the runtime uses to communicate with infrastructure
// services when running on Encore Cloud.
type EncoreCloudAPI struct {
	Server string `json:"server"` // The Encore Cloud server we're using

	// The auth keys to use when authenticating with the Encore Cloud API server, either for signing requests or
	// authenticating requests from the Encore Cloud API server.
	//
	// Note: these are not the same as the auth keys used to authenticate requests from Encore's central platform.
	AuthKeys []auth.Key `json:"auth_keys"`
}

type EncoreAuthKey struct {
	KeyID uint32 `json:"kid"`
	Data  []byte `json:"data"`
}

func (eak EncoreAuthKey) Copy() EncoreAuthKey {
	c := eak
	copy(c.Data, eak.Data)
	return c
}

type PubsubProvider struct {
	NSQ         *NSQProvider               `json:"nsq,omitempty"`          // set if the provider is NSQ
	GCP         *GCPPubsubProvider         `json:"gcp,omitempty"`          // set if the provider is GCP
	AWS         *AWSPubsubProvider         `json:"aws,omitempty"`          // set if the provider is AWS
	Azure       *AzureServiceBusProvider   `json:"azure,omitempty"`        // set if the provider is Azure
	EncoreCloud *EncoreCloudPubsubProvider `json:"encore_cloud,omitempty"` // set if the provider is Encore Cloud
}

type AzureServiceBusProvider struct {
	Namespace string `json:"namespace"`
}
type NSQProvider struct {
	Host string `json:"host"`
}

type EncoreCloudPubsubProvider struct{}

// GCPPubsubProvider currently has no specific configuration.
type GCPPubsubProvider struct {
}

// AWSPubsubProvider currently has no specific configuration.
type AWSPubsubProvider struct {
}

type PubsubTopic struct {
	EncoreName   string   `json:"encore_name"`       // the Encore name for the pubsub topic
	ProviderID   int      `json:"provider_id"`       // The index into (*Runtime).PubsubProviders.
	ProviderName string   `json:"provider_name"`     // the name for the pubsub topic as defined by the provider
	Limiter      *Limiter `json:"limiter,omitempty"` // the rate limiter for the topic

	// Subscriptions contains the subscriptions to this topic,
	// keyed by the Encore name.
	Subscriptions map[string]*PubsubSubscription `json:"subscriptions"`

	// GCP contains GCP-specific configuration.
	// It is set if the provider is GCP.
	GCP *PubsubTopicGCPData `json:"gcp,omitempty"`
}

type PubsubSubscription struct {
	ID           string `json:"id"`            // the subscription ID
	EncoreName   string `json:"encore_name"`   // the Encore name for the subscription
	ProviderName string `json:"provider_name"` // the name for the pubsub subscription as defined by the provider
	PushOnly     bool   `json:"push_only"`     // if true the application will not actively subscribe to the pub, but instead will rely on HTTP push messages

	// GCP contains GCP-specific configuration.
	// It is set if the subscription exists in GCP.
	GCP *PubsubSubscriptionGCPData `json:"gcp,omitempty"`
}

type PubsubTopicGCPData struct {
	// ProjectID is the GCP project id where the topic exists.
	ProjectID string `json:"project_id"`
}

type PubsubSubscriptionGCPData struct {
	// ProjectID is the GCP project id where the subscription exists.
	ProjectID string `json:"project_id"`

	// PushServiceAccount is the service account used to authenticate
	// messages being delivered over push.
	// If empty pushes are not accepted.
	PushServiceAccount string `json:"push_service_account"`
}

type StaticPubsubTopic struct {
	Subscriptions map[string]*StaticPubsubSubscription
}

type StaticPubsubSubscription struct {
	Service  string // the service that subscription is in
	SvcNum   uint16 // the service number the subscription is in
	TraceIdx uint32 // The trace Idx of the subscription
}

type SQLServer struct {
	// Host is the host to connect to.
	// Valid formats are "hostname", "hostname:port", and "/path/to/unix.socket".
	Host string `json:"host"`

	// ServerCACert is the PEM-encoded server CA cert, or "" if not required.
	ServerCACert string `json:"server_ca_cert,omitempty"`
	// ClientCert is the PEM-encoded client cert, or "" if not required.
	ClientCert string `json:"client_cert,omitempty"`
	// ClientKey is the PEM-encoded client key, or "" if not required.
	ClientKey string `json:"client_key,omitempty"`
}

type SQLDatabase struct {
	ServerID     int    `json:"server_id"`     // the index into (*Runtime).SQLServers
	EncoreName   string `json:"encore_name"`   // the Encore name for the database
	DatabaseName string `json:"database_name"` // the actual database name as known by the SQL server.
	User         string `json:"user"`
	Password     string `json:"password"`

	// MinConnections is the minimum number of open connections to use
	// for this database. If zero it defaults to 2.
	MinConnections int `json:"min_connections"`

	// MaxConnections is the maximum number of open connections to use
	// for this database. If zero it defaults to 30.
	MaxConnections int `json:"max_connections"`
}

type RedisServer struct {
	// Host is the host to connect to.
	// Valid formats are "hostname", "hostname:port", and "/path/to/unix.socket".
	Host string `json:"host"`

	// User and password specify the authentication behavior to redis.
	// If both are provided, it uses Redis v6's ACL support.
	// If a password but no username is provided, it uses Redis's AUTH string support.
	// If neither is supplied it uses no authentication.
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`

	// EnableTLS specifies whether or not to use TLS to connect.
	// If ServerCACert, ClientCert, or ClientKey are provided it is
	// automatically enabled regardless of the value.
	EnableTLS bool `json:"enable_tls"`
	// ServerCACert is the PEM-encoded server CA cert, or "" if not required.
	ServerCACert string `json:"server_ca_cert,omitempty"`
	// ClientCert is the PEM-encoded client cert, or "" if not required.
	ClientCert string `json:"client_cert,omitempty"`
	// ClientKey is the PEM-encoded client key, or "" if not required.
	ClientKey string `json:"client_key,omitempty"`
}

type RedisDatabase struct {
	ServerID   int    `json:"server_id"`   // the index into (*Runtime).RedisServers
	EncoreName string `json:"encore_name"` // the Encore name for the database

	// Database is the database index to use, from 0-15.
	Database int `json:"database"`

	// MinConnections is the minimum number of open connections to use
	// for this database. It defaults to 1.
	MinConnections int `json:"min_connections"`

	// MaxConnections is the maximum number of open connections to use
	// for this database. If zero it defaults to 10*GOMAXPROCS.
	MaxConnections int `json:"max_connections"`

	// KeyPrefix specifies a prefix to add to all cache keys
	// for this database. It exists to enable multiple cache clusters
	// to use the same physical Redis database for local development
	// without having to coordinate and persist database index ids.
	KeyPrefix string `json:"key_prefix"`
}

type BucketProvider struct {
	S3  *S3BucketProvider  `json:"s3,omitempty"`  // set if the provider is S3
	GCS *GCSBucketProvider `json:"gcs,omitempty"` // set if the provider is GCS
}

type S3BucketProvider struct {
	Region string `json:"region"`
	// The endpoint to use. If nil, the default endpoint for the region is used.
	// Must be set for non-AWS endpoints.
	Endpoint *string `json:"endpoint"`

	// The access key to use. If either is nil, the default credentials are used.
	AccessKeyID     *string `json:"access_key_id"`
	SecretAccessKey *string `json:"secret_access_key"`
}

type GCSBucketProvider struct {
	Endpoint  string `json:"endpoint"`
	Anonymous bool   `json:"anonymous"`

	// Additional options for signed URLs when running in local dev mode.
	// Only use with anonymous mode.
	LocalSign *GCSLocalSignOptions `json:"local_sign"`
}

type GCSLocalSignOptions struct {
	BaseUrl    string `json:"base_url"`
	AccessId   string `json:"access_id"`
	PrivateKey string `json:"private_key"`
}

type Bucket struct {
	ProviderID int    `json:"cluster_id"`  // the index into (*Runtime).BucketProviders
	EncoreName string `json:"encore_name"` // the Encore name for the bucket
	CloudName  string `json:"cloud_name"`  // the cloud name for the bucket
	KeyPrefix  string `json:"key_prefix"`  // the prefix to use for all keys in the bucket

	// The public base url for the bucket.
	// Only set if the bucket is public.
	PublicBaseURL string `json:"public_base_url"`
}

type Metrics struct {
	CollectionInterval time.Duration                  `json:"collection_interval,omitempty"`
	EncoreCloud        *GCPCloudMonitoringProvider    `json:"encore_cloud,omitempty"`
	CloudMonitoring    *GCPCloudMonitoringProvider    `json:"gcp_cloud_monitoring,omitempty"`
	CloudWatch         *AWSCloudWatchMetricsProvider  `json:"aws_cloud_watch,omitempty"`
	LogsBased          *LogsBasedMetricsProvider      `json:"logs_based,omitempty"`
	Prometheus         *PrometheusRemoteWriteProvider `json:"prometheus,omitempty"`
	Datadog            *DatadogProvider               `json:"datadog,omitempty"`
}

type GCPCloudMonitoringProvider struct {
	// ProjectID is the GCP project id to send metrics to.
	ProjectID string

	// MonitoredResourceType is the enum value for the monitored resource this application is monitoring.
	// See https://cloud.google.com/monitoring/api/resources for valid values.
	MonitoredResourceType string
	// MonitoredResourceLabels are the labels to specify for the monitored resource.
	// Each monitored resource type has a pre-defined set of labels that must be set.
	// See https://cloud.google.com/monitoring/api/resources for expected labels.
	MonitoredResourceLabels map[string]string

	// MetricNames contains the mapping between metric names in Encore and metric
	// names in GCP.
	MetricNames map[string]string
}

type AWSCloudWatchMetricsProvider struct {
	// Namespace is the namespace to use for metrics.
	Namespace string
}

type PrometheusRemoteWriteProvider struct {
	// The URL of the endpoint to send samples to.
	RemoteWriteURL string
}

type DatadogProvider struct {
	Site   string
	APIKey string
}

type LogsBasedMetricsProvider struct{}

// Limiter represents a rate limiter that can be used for certain types of operations
//
// The fields are mutually exclusive, which ever is not nil is the limiter that will be used,
// if all are nil, then no limit is enforced.
type Limiter struct {
	TokenBucket *TokenBucketLimiter `json:"token_bucket"` // A token bucket limiter
}

type TokenBucketLimiter struct {
	PerSecondRate float64 `json:"rate"` // The rate at which to allow requests to pass through.
	BucketSize    int     `json:"size"` // The size of the token bucket (starts full)
}
