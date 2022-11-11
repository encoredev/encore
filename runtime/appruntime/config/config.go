package config

import (
	"fmt"
	"reflect"
	"time"
)

type Config struct {
	Static  *Static           // Static config is code generated and compiled into the binary
	Runtime *Runtime          // Runtime config is loaded from the environment
	Secrets map[string]string // Secrets are loaded from the environment
}

type Static struct {
	// AuthData is the custom auth data type, or nil
	AuthData reflect.Type

	// The version of Encore which the app was compiled with.
	// This is string is for informational use only, and it's format should not be relied on.
	EncoreCompiler string
	AppCommit      CommitInfo // The commit which this service was built from

	PubsubTopics map[string]*StaticPubsubTopic

	Testing              bool
	TestService          string // service being tested, if any
	TestAsExternalBinary bool   // should logs be pretty printed in tests (used when building a test binary to be used outside of the Encore daemon)
}

type Runtime struct {
	AppID         string          `json:"app_id"`
	AppSlug       string          `json:"app_slug"`
	APIBaseURL    string          `json:"api_base_url"`
	EnvID         string          `json:"env_id"`
	EnvName       string          `json:"env_name"`
	EnvType       string          `json:"env_type"`
	EnvCloud      string          `json:"env_cloud"`
	DeployID      string          `json:"deploy_id"`
	DeployedAt    time.Time       `json:"deploy_time"`
	TraceEndpoint string          `json:"trace_endpoint,omitempty"`
	AuthKeys      []EncoreAuthKey `json:"auth_keys,omitempty"`
	CORS          *CORS           `json:"cors,omitempty"`

	SQLDatabases    []*SQLDatabase          `json:"sql_databases,omitempty"`
	SQLServers      []*SQLServer            `json:"sql_servers,omitempty"`
	PubsubProviders []*PubsubProvider       `json:"pubsub_providers,omitempty"`
	PubsubTopics    map[string]*PubsubTopic `json:"pubsub_topics,omitempty"`
	RedisServers    []*RedisServer          `json:"redis_servers,omitempty"`
	RedisDatabases  []*RedisDatabase        `json:"redis_databases,omitempty"`
	Metrics         *Metrics                `json:"metrics,omitempty"`

	// ShutdownTimeout is the duration before non-graceful shutdown is initiated,
	// meaning connections are closed even if outstanding requests are still in flight.
	// If zero, it shuts down immediately.
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// UnsafeAllOriginWithCredentials can be used to specify that all origins are
// allowed to call this API with credentials. It is unsafe and misuse can lead
// to security issues. Only use if you know what you're doing.
const UnsafeAllOriginWithCredentials = "UNSAFE_ALL_ORIGINS_WITH_CREDENTIALS"

type CORS struct {
	// DisableCredentials, if true, causes Encore to respond to OPTIONS requests
	// without setting Access-Control-Allow-Credentials: true.
	DisableCredentials bool `json:"disable_credentials,omitempty"`

	// AllowOriginsWithCredentials specifies the allowed origins for requests
	// that include credentials. If a request is made from an Origin in this list
	// Encore responds with Access-Control-Allow-Origin: <Origin>.
	// If DisableCredentials is true this field is not used.
	AllowOriginsWithCredentials []string `json:"allow_origins_with_credentials,omitempty"`

	// AllowOriginsWithoutCredentials specifies the allowed origins for requests
	// that don't include credentials. If nil it defaults to allowing all domains
	// (equivalent to []string{"*"}).
	AllowOriginsWithoutCredentials []string `json:"allow_origins_without_credentials,omitempty"`

	// ExtraAllowedHeaders specifies extra headers to allow, beyond
	// the default set of {"Origin", "Authorization", "Content-Type"}.
	// As a special case, if the list contains "*" all headers are allowed.
	ExtraAllowedHeaders []string `json:"raw_allowed_headers,omitempty"`
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
	NSQ   *NSQProvider             `json:"nsq,omitempty"`   // set if the provider is NSQ
	GCP   *GCPPubsubProvider       `json:"gcp,omitempty"`   // set if the provider is GCP
	AWS   *AWSPubsubProvider       `json:"aws,omitempty"`   // set if the provider is AWS
	Azure *AzureServiceBusProvider `json:"azure,omitempty"` // set if the provider is Azure
}

type AzureServiceBusProvider struct {
	Namespace string `json:"namespace"`
}
type NSQProvider struct {
	Host string `json:"host"`
}

// GCPPubsubProvider currently has no specific configuration.
type GCPPubsubProvider struct {
}

// AWSPubsubProvider currently has no specific configuration.
type AWSPubsubProvider struct {
}

type PubsubTopic struct {
	EncoreName   string `json:"encore_name"`   // the Encore name for the pubsub topic
	ProviderID   int    `json:"provider_id"`   // The index into (*Runtime).PubsubProviders.
	ProviderName string `json:"provider_name"` // the name for the pubsub topic as defined by the provider
	OrderingKey  string `json:"ordering_key"`  // the ordering key for the pubsub topic (blank if not ordered)

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
	TraceIdx int32  // The trace Idx of the subscription
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

type MetricsExporterType string

const (
	MetricsExporterTypeLogsBased MetricsExporterType = "logs_based"
)

type Metrics struct {
	CloudMonitoring *GCPCloudMonitoringProvider `json:"gcp_cloud_monitoring,omitempty"`
	LogsBased       *LogsBasedMetricsProvider   `json:"logs_based,omitempty"`
}

type LogsBasedMetricsProvider struct{}

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
}
