package infra

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
)

type InfraConfig struct {
	Metadata         Metadata                     `json:"metadata,omitempty"`
	GracefulShutdown *GracefulShutdown            `json:"graceful_shutdown,omitempty"`
	Auth             []*Auth                      `json:"auth,omitempty"`
	ServiceDiscovery map[string]*ServiceDiscovery `json:"service_discovery,omitempty"`
	Metrics          *Metrics                     `json:"metrics,omitempty"`
	SQLServers       []*SQLServer                 `json:"sql_servers,omitempty"`
	Redis            map[string]*Redis            `json:"redis,omitempty"`
	PubSub           []*PubSub                    `json:"pubsub,omitempty"`
	Secrets          Secrets                      `json:"secrets,omitempty"`
	ObjectStorage    []*ObjectStorage             `json:"object_storage,omitempty"`

	// Log configuration for the application.
	// If empty it defaults to "trace".
	LogConfig string `json:"log_config,omitemty"`

	// Number of worker threads to use for the application.
	// If unset it defaults to a single worker thread.
	// If set to 0 it defaults to the number of CPUs.
	WorkerThreads *int `json:"worker_threads,omitempty"`

	// These fields are not defined in the json schema and should not be
	// set by the user. They're computed during the build/eject process.
	HostedServices []string `json:"hosted_services,omitempty"`
	HostedGateways []string `json:"hosted_gateways,omitempty"`
	CORS           *CORS    `json:"cors,omitempty"`
}

type ObjectStorage struct {
	Type string `json:"type"`
	GCS  *GCS   `json:"gcs,omitempty"`
	S3   *S3    `json:"s3,omitempty"`
}

func (o *ObjectStorage) GetBuckets() map[string]*Bucket {
	switch o.Type {
	case "gcs":
		return o.GCS.Buckets
	case "s3":
		return o.S3.Buckets
	default:
		panic("unsupported object storage type")
	}
}

func (o *ObjectStorage) DeleteBucket(name string) {
	switch o.Type {
	case "gcs":
		delete(o.GCS.Buckets, name)
	case "s3":
		delete(o.S3.Buckets, name)
	default:
		panic("unsupported object storage type")
	}

}

func (a *ObjectStorage) Validate(v *validator) {
	v.ValidateField("Type", OneOf(a.Type, "gcs", "s3"))
	switch a.Type {
	case "gcs":
		a.GCS.Validate(v)
	case "s3":
		a.S3.Validate(v)
	default:
		v.ValidateField("type", Err("unsupported object storage type"))
	}
}

func (p *ObjectStorage) MarshalJSON() ([]byte, error) {
	// Create a map to hold the JSON structure
	m := make(map[string]interface{})

	// Always add the type
	m["type"] = p.Type

	// Add the specific object storage configuration based on the type
	switch p.Type {
	case "gcs":
		if p.GCS != nil {
			for k, v := range structToMap(p.GCS) {
				m[k] = v
			}
		}
	case "s3":
		if p.S3 != nil {
			for k, v := range structToMap(p.S3) {
				m[k] = v
			}
		}
	default:
		return nil, errors.New("unsupported object storage type")
	}

	return json.Marshal(m)
}

// UnmarshalJSON custom unmarshaller for PubSub.
func (p *ObjectStorage) UnmarshalJSON(data []byte) error {
	// Anonymous struct to capture the "type" field first.
	var aux struct {
		Type string `json:"type,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Set the Type field.
	p.Type = aux.Type

	// Unmarshal based on the "type" field.
	switch aux.Type {
	case "gcs":
		var g GCS
		if err := json.Unmarshal(data, &g); err != nil {
			return err
		}
		p.GCS = &g
	case "s3":
		var a S3
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		p.S3 = &a
	default:
		return errors.New("unsupported object storage type")
	}

	return nil
}

type S3 struct {
	Region   string `json:"region"`
	Endpoint string `json:"endpoint,omitempty"`

	AccessKeyID     string    `json:"access_key_id,omitempty"`
	SecretAccessKey EnvString `json:"secret_access_key,omitempty"`

	Buckets map[string]*Bucket `json:"buckets,omitempty"`
}

func (a *S3) Validate(v *validator) {
	v.ValidateField("region", NotZero(a.Region))
	if a.AccessKeyID != "" {
		v.ValidatePtrEnvRef("secret_access_key", &a.SecretAccessKey, "S3 Secret Access Key", NotZero[string])
	}
	ValidateChildMap(v, "buckets", a.Buckets)
}

type GCS struct {
	Endpoint string             `json:"endpoint,omitempty"`
	Buckets  map[string]*Bucket `json:"buckets,omitempty"`
}

func (a *GCS) Validate(v *validator) {
	ValidateChildMap(v, "buckets", a.Buckets)
}

type Bucket struct {
	Name          string `json:"name,omitempty"`
	KeyPrefix     string `json:"key_prefix,omitempty"`
	PublicBaseURL string `json:"public_base_url,omitempty"`
}

func (a *Bucket) Validate(v *validator) {
	v.ValidateField("name", NotZero(a.Name))

	v.ValidateField("public_base_url", func() error {
		if a.PublicBaseURL != "" {
			if _, err := url.Parse(a.PublicBaseURL); err != nil {
				return fmt.Errorf("Not a valid URL: %v", err)
			}
		}
		return nil
	})
}

type Metadata struct {
	AppID   string `json:"app_id,omitempty"`
	EnvName string `json:"env_name,omitempty"`
	EnvType string `json:"env_type,omitempty"`
	Cloud   string `json:"cloud,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

// Copy of the CORS struct from the appfile
type CORS struct {
	Debug                          bool     `json:"debug,omitempty"`
	AllowHeaders                   []string `json:"allow_headers,omitempty"`
	ExposeHeaders                  []string `json:"expose_headers,omitempty"`
	AllowOriginsWithoutCredentials []string `json:"allow_origins_without_credentials,omitempty"`
	AllowOriginsWithCredentials    []string `json:"allow_origins_with_credentials,omitempty"`
}

func (i *InfraConfig) Validate(v *validator) {
	v.ValidateChild("graceful_shutdown", i.GracefulShutdown)
	ValidateChildList(v, "auth", i.Auth)
	ValidateChildMap(v, "service_discovery", i.ServiceDiscovery)
	ValidateChildList(v, "object_storage", i.ObjectStorage)
	v.ValidateChild("metrics", i.Metrics)
	ValidateChildList(v, "sql_servers", i.SQLServers)
	ValidateChildMap(v, "redis", i.Redis)
	ValidateChildList(v, "pubsub", i.PubSub)
	v.ValidateChild("secrets", i.Secrets)
}

type Secrets struct {
	SecretsMap map[string]EnvString
	EnvRef     *EnvRef
}

func (s Secrets) Validate(v *validator) {
	if s.EnvRef != nil {
		v.ValidateEnvRef("env_ref", *s.EnvRef, "An environment variable containing a JSON object of secrets")
		return
	}
	for name, value := range s.SecretsMap {
		v.ValidateEnvString(name, value, "Secret", nil)
	}
}

func (s *Secrets) GetSecrets() map[string]string {
	if s.EnvRef != nil {
		refs := make(map[string]string)
		envValue := os.Getenv(s.EnvRef.Env)
		if err := json.Unmarshal([]byte(envValue), &refs); err != nil {
			log.Fatalf("Error unmarshalling secrets")
		}
		return refs
	}
	return MapValues(s.SecretsMap, func(k string, v EnvString) string {
		return v.Value()
	})
}

// UnmarshalJSON is a custom JSON unmarshaller for the Secrets type.
func (s *Secrets) UnmarshalJSON(data []byte) error {
	// Try unmarshalling as an EnvRef.
	var ref EnvRef
	if err := json.Unmarshal(data, &ref); err == nil && ref.Env != "" {
		s.EnvRef = &ref
		return nil
	}

	// Try unmarshalling as a map of strings to EnvString.
	var m map[string]EnvString
	if err := json.Unmarshal(data, &m); err == nil {
		s.SecretsMap = m
		return nil
	}
	return errors.New("invalid Secrets structure")
}

// MarshalJSON is a custom JSON marshaller for the Secrets type.
func (s Secrets) MarshalJSON() ([]byte, error) {
	if s.EnvRef == nil {
		return json.Marshal(s.SecretsMap)
	}
	return json.Marshal(s.EnvRef)
}

type GracefulShutdown struct {
	Total         *int `json:"total,omitempty"`
	ShutdownHooks *int `json:"shutdown_hooks,omitempty"`
	Handlers      *int `json:"handlers,omitempty"`
}

func (g *GracefulShutdown) Validate(v *validator) {
	v.ValidateField("total", NilOr(g.Total, GreaterOrEqual(0)))
	v.ValidateField("shutdown_hooks", NilOr(g.ShutdownHooks, GreaterOrEqual(0)))
	v.ValidateField("handlers", NilOr(g.Handlers, GreaterOrEqual(0)))
}

type Auth struct {
	Type string    `json:"type,omitempty"`
	ID   int       `json:"id,omitempty"`
	Key  EnvString `json:"key,omitempty"`
}

func (a *Auth) Validate(v *validator) {
	v.ValidateField("type", OneOf(a.Type, "key"))
	v.ValidateEnvString("key", a.Key, "Service Authorization Key", NotZero[string])
}

type ServiceDiscovery struct {
	BaseURL string  `json:"base_url,omitempty"`
	Auth    []*Auth `json:"auth,omitempty"`
}

func (s *ServiceDiscovery) Validate(v *validator) {
	v.ValidateField("base_url", NotZero(s.BaseURL))
	ValidateChildList(v, "auth", s.Auth)
}

// Main Metrics struct which embeds the different metric types.
type Metrics struct {
	Type               string `json:"type,omitempty"`
	CollectionInterval int    `json:"collection_interval,omitempty"`
	Prometheus         *Prometheus
	Datadog            *Datadog
	GCPCloudMonitoring *GCPCloudMonitoring
	AWSCloudWatch      *AWSCloudWatch
}

// MarshalJSON custom marshaller to handle dynamic types in Metrics.
func (m *Metrics) MarshalJSON() ([]byte, error) {
	// Create a map to hold the JSON structure
	data := make(map[string]interface{})

	data["type"] = m.Type
	data["collection_interval"] = m.CollectionInterval

	switch m.Type {
	case "prometheus":
		if m.Prometheus != nil {
			for k, v := range structToMap(m.Prometheus) {
				data[k] = v
			}
		}
	case "datadog":
		if m.Datadog != nil {
			for k, v := range structToMap(m.Datadog) {
				data[k] = v
			}
		}
	case "gcp_cloud_monitoring":
		if m.GCPCloudMonitoring != nil {
			for k, v := range structToMap(m.GCPCloudMonitoring) {
				data[k] = v
			}
		}
	case "aws_cloudwatch":
		if m.AWSCloudWatch != nil {
			for k, v := range structToMap(m.AWSCloudWatch) {
				data[k] = v
			}
		}
	default:
		return nil, errors.New("unsupported metrics type")
	}

	return json.Marshal(data)
}

// UnmarshalJSON custom unmarshaller to handle dynamic types in Metrics.
func (m *Metrics) UnmarshalJSON(data []byte) error {
	// Anonymous struct to capture the "type" field first
	var aux struct {
		Type string `json:"type,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Set the Type field
	m.Type = aux.Type

	// Unmarshal based on the "type" field
	switch aux.Type {
	case "prometheus":
		var p Prometheus
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		m.Prometheus = &p
	case "datadog":
		var d Datadog
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		m.Datadog = &d
	case "gcp_cloud_monitoring":
		var g GCPCloudMonitoring
		if err := json.Unmarshal(data, &g); err != nil {
			return err
		}
		m.GCPCloudMonitoring = &g
	case "aws_cloudwatch":
		var a AWSCloudWatch
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		m.AWSCloudWatch = &a
	default:
		return errors.New("unsupported metrics type")
	}

	return nil
}

func (m *Metrics) Validate(v *validator) {
	switch m.Type {
	case "prometheus":
		m.Prometheus.Validate(v)
	case "datadog":
		m.Datadog.Validate(v)
	case "gcp_cloud_monitoring":
		m.GCPCloudMonitoring.Validate(v)
	case "aws_cloudwatch":
		m.AWSCloudWatch.Validate(v)
	default:
		v.ValidateField("type", Err("unsupported metrics type"))
	}
}

// Prometheus-specific metric configuration.
type Prometheus struct {
	RemoteWriteURL EnvString `json:"remote_write_url,omitempty"`
}

func (p *Prometheus) Validate(v *validator) {
	v.ValidateEnvString("remote_write_url", p.RemoteWriteURL, "Prometheus Remote Write URL", NotZero[string])
}

// Datadog-specific metric configuration.
type Datadog struct {
	Site   string    `json:"site,omitempty"`
	APIKey EnvString `json:"api_key,omitempty"`
}

func (d *Datadog) Validate(v *validator) {
	v.ValidateField("site", NotZero(d.Site))
	v.ValidateEnvString("api_key", d.APIKey, "Datadog API Key", NotZero[string])
}

// GCP Cloud Monitoring-specific metric configuration.
type GCPCloudMonitoring struct {
	ProjectID               string            `json:"project_id,omitempty"`
	MonitoredResourceType   string            `json:"monitored_resource_type,omitempty"`
	MonitoredResourceLabels map[string]string `json:"monitored_resource_labels,omitempty"`
	MetricNames             map[string]string `json:"metric_names,omitempty"`
}

func (g *GCPCloudMonitoring) Validate(v *validator) {
	v.ValidateField("project_id", NotZero(g.ProjectID))
	v.ValidateField("monitored_resource_type", NotZero(g.MonitoredResourceType))
}

// AWS CloudWatch-specific metric configuration.
type AWSCloudWatch struct {
	Namespace string `json:"namespace,omitempty"`
}

func (a *AWSCloudWatch) Validate(v *validator) {
	v.ValidateField("namespace", NotZero(a.Namespace))
}

type SQLServer struct {
	Host      string                  `json:"host,omitempty"`
	TLSConfig *TLSConfig              `json:"tls_config,omitempty"`
	Databases map[string]*SQLDatabase `json:"databases,omitempty"`
}

func (s *SQLServer) Validate(v *validator) {
	v.ValidateField("host", NotZero(s.Host))
	v.ValidateChild("tls_config", s.TLSConfig)
	ValidateChildMap(v, "databases", s.Databases)
}

type TLSConfig struct {
	CA                             string      `json:"ca,omitempty"`
	ClientCert                     *ClientCert `json:"client_cert,omitempty"`
	DisableTLSHostnameVerification bool        `json:"disable_tls_hostname_verification,omitempty"`
	DisableCAValidation            bool        `json:"disable_ca_validation,omitempty"`
}

func (t *TLSConfig) Validate(v *validator) {
	v.ValidateChild("client_cert", t.ClientCert)
}

type SQLDatabase struct {
	Name           string      `json:"name,omitempty"`
	MaxConnections int         `json:"max_connections,omitempty"`
	MinConnections int         `json:"min_connections,omitempty"`
	Username       EnvString   `json:"username,omitempty"`
	Password       EnvString   `json:"password,omitempty"`
	ClientCert     *ClientCert `json:"client_cert,omitempty"`
}

func (s *SQLDatabase) Validate(v *validator) {
	v.ValidateField("max_connections", GreaterOrEqual(s.MinConnections)(s.MaxConnections))
	v.ValidateField("min_connections", GreaterOrEqual(0)(s.MinConnections))
	v.ValidateEnvString("username", s.Username, "Database Username", NotZero[string])
	v.ValidateEnvString("password", s.Password, "Database Password", NotZero[string])
	v.ValidateChild("client_cert", s.ClientCert)
}

type Redis struct {
	Host           string     `json:"host,omitempty"`
	DatabaseIndex  int        `json:"database_index,omitempty"`
	Auth           *RedisAuth `json:"auth,omitempty"`
	KeyPrefix      *string    `json:"key_prefix,omitempty"`
	TLSConfig      *TLSConfig `json:"tls_config,omitempty"`
	MaxConnections *int       `json:"max_connections,omitempty"`
	MinConnections *int       `json:"min_connections,omitempty"`
}

func (r *Redis) Validate(v *validator) {
	v.ValidateField("host", NotZero(r.Host))
	v.ValidateField("database_index", Between(0, 15)(r.DatabaseIndex))
	v.ValidateChild("auth", r.Auth)
	v.ValidateChild("tls_config", r.TLSConfig)
	v.ValidateField("max_connections", NilOr(r.MaxConnections, GreaterOrEqual(0)))
	v.ValidateField("min_connections", NilOr(r.MinConnections, GreaterOrEqual(0)))
}

type RedisAuth struct {
	Type       string     `json:"type,omitempty"`
	Username   *EnvString `json:"username,omitempty"`
	Password   *EnvString `json:"password,omitempty"`
	AuthString *EnvString `json:"auth_string,omitempty"`
}

func (r *RedisAuth) Validate(v *validator) {
	v.ValidateField("type", NotZero(r.Type))
	switch r.Type {
	case "acl":
		v.ValidatePtrEnvRef("username", r.Username, "Redis Username", NotZero[string])
		v.ValidatePtrEnvRef("password", r.Password, "Redis Password", NotZero[string])
	case "auth_string":
		v.ValidatePtrEnvRef("auth_string", r.AuthString, "Redis Auth String", NotZero[string])
	default:
		v.ValidateField("type", Err("unsupported Redis auth type"))
	}
}

type ClientCert struct {
	Cert string    `json:"cert,omitempty"`
	Key  EnvString `json:"key,omitempty"`
}

func (c *ClientCert) Validate(v *validator) {
	v.ValidateField("cert", NotZero(c.Cert))
	v.ValidateEnvString("key", c.Key, "Client Certificate Key", NotZero[string])
}

// Main PubSub struct which embeds different PubSub types.
type PubSub struct {
	Type string `json:"type,omitempty"`
	GCP  *GCPPubsub
	AWS  *AWSSNS_SQS
	NSQ  *NSQPubsub
}

func (p *PubSub) Validate(v *validator) {
	switch p.Type {
	case "gcp_pubsub":
		p.GCP.Validate(v)
	case "aws_sns_sqs":
		p.AWS.Validate(v)
	case "nsq":
		p.NSQ.Validate(v)
	default:
		v.ValidateField("type", Err("unsupported pubsub type"))
	}
}

func (p *PubSub) DeleteTopic(name string) {
	switch p.Type {
	case "gcp_pubsub":
		p.GCP.DeleteTopic(name)
	case "aws_sns_sqs":
		p.AWS.DeleteTopic(name)
	case "nsq":
		p.NSQ.DeleteTopic(name)
	}
}

func (p *PubSub) GetTopics() map[string]PubsubTopic {
	switch p.Type {
	case "gcp_pubsub":
		return p.GCP.GetTopics()
	case "aws_sns_sqs":
		return p.AWS.GetTopics()
	case "nsq":
		return p.NSQ.GetTopics()
	default:
		panic("unsupported pubsub type")
	}
}

type PubsubTopic interface {
	GetSubscriptions() map[string]PubsubSubscription
	DeleteSubscription(name string)
}

type PubsubSubscription interface{}

type PubsubProvider interface {
	GetTopics() map[string]PubsubTopic
	DeleteTopic(name string)
}

// GCPPubsub specific configuration.
type GCPPubsub struct {
	ProjectID string               `json:"project_id,omitempty"`
	Topics    map[string]*GCPTopic `json:"topics,omitempty"`
}

func (g *GCPPubsub) Validate(v *validator) {
	ValidateChildMap(v, "topics", g.Topics)
}

func (g *GCPPubsub) GetTopics() map[string]PubsubTopic {
	return MapValues(g.Topics, func(k string, v *GCPTopic) PubsubTopic {
		return v
	})
}

func (g *GCPPubsub) DeleteTopic(name string) {
	delete(g.Topics, name)
}

type GCPTopic struct {
	Name          string             `json:"name,omitempty"`
	ProjectID     string             `json:"project_id,omitempty"`
	Subscriptions map[string]*GCPSub `json:"subscriptions,omitempty"`
}

func (g *GCPTopic) Validate(v *validator) {
	v.ValidateField("name", NotZero(g.Name))
	pubsub := Ancestor[*PubSub](v)
	v.ValidateField("project_id", AnyNonZero(g.ProjectID, pubsub.GCP.ProjectID))
	ValidateChildMap(v, "subscriptions", g.Subscriptions)
}

func (g *GCPTopic) GetSubscriptions() map[string]PubsubSubscription {
	return MapValues(g.Subscriptions, func(k string, v *GCPSub) PubsubSubscription {
		return v
	})
}

func (g *GCPTopic) DeleteSubscription(name string) {
	delete(g.Subscriptions, name)
}

type GCPSub struct {
	Name       string      `json:"name,omitempty"`
	ProjectID  string      `json:"project_id,omitempty"`
	PushConfig *PushConfig `json:"push_config,omitempty"`
}

func (g *GCPSub) Validate(v *validator) {
	v.ValidateField("name", NotZero(g.Name))
	pubsub := Ancestor[*PubSub](v)
	v.ValidateField("project_id", AnyNonZero(g.ProjectID, pubsub.GCP.ProjectID))
	v.ValidateChild("push_config", g.PushConfig)
}

type PushConfig struct {
	ServiceAccount string `json:"service_account,omitempty"`
	JWTAudience    string `json:"jwt_audience,omitempty"`
	ID             string `json:"id,omitempty"`
}

func (p *PushConfig) Validate(v *validator) {
	v.ValidateField("service_account", NotZero(p.ServiceAccount))
	v.ValidateField("jwt_audience", NotZero(p.JWTAudience))
	v.ValidateField("id", NotZero(p.ID))
}

// AWSSNS_SQS specific configuration.
type AWSSNS_SQS struct {
	Topics map[string]*AWSTopic `json:"topics,omitempty"`
}

func (a *AWSSNS_SQS) Validate(v *validator) {
	ValidateChildMap(v, "topics", a.Topics)
}

func (a *AWSSNS_SQS) GetTopics() map[string]PubsubTopic {
	return MapValues(a.Topics, func(k string, v *AWSTopic) PubsubTopic {
		return v
	})
}

func (a *AWSSNS_SQS) DeleteTopic(name string) {
	delete(a.Topics, name)
}

type AWSTopic struct {
	ARN           string             `json:"arn,omitempty"`
	Subscriptions map[string]*AWSSub `json:"subscriptions,omitempty"`
}

func (a *AWSTopic) Validate(v *validator) {
	v.ValidateField("arn", NotZero(a.ARN))
	ValidateChildMap(v, "subscriptions", a.Subscriptions)
}

func (a *AWSTopic) GetSubscriptions() map[string]PubsubSubscription {
	return MapValues(a.Subscriptions, func(k string, v *AWSSub) PubsubSubscription {
		return v
	})
}

func (a *AWSTopic) DeleteSubscription(name string) {
	delete(a.Subscriptions, name)
}

type AWSSub struct {
	URL string `json:"url,omitempty"`
}

func (a *AWSSub) Validate(v *validator) {
	v.ValidateField("url", NotZero(a.URL))
}

// NSQPubsub specific configuration.
type NSQPubsub struct {
	Hosts  string               `json:"hosts,omitempty"`
	Topics map[string]*NSQTopic `json:"topics,omitempty"`
}

func (n *NSQPubsub) Validate(v *validator) {
	v.ValidateField("hosts", NotZero(n.Hosts))
	ValidateChildMap(v, "topics", n.Topics)
}

func (n *NSQPubsub) GetTopics() map[string]PubsubTopic {
	return MapValues(n.Topics, func(k string, v *NSQTopic) PubsubTopic {
		return v
	})
}

func (n *NSQPubsub) DeleteTopic(name string) {
	delete(n.Topics, name)
}

type NSQTopic struct {
	Name          string             `json:"name,omitempty"`
	Subscriptions map[string]*NSQSub `json:"subscriptions,omitempty"`
}

func (n *NSQTopic) Validate(v *validator) {
	v.ValidateField("name", NotZero(n.Name))
	ValidateChildMap(v, "subscriptions", n.Subscriptions)
}

func (n *NSQTopic) GetSubscriptions() map[string]PubsubSubscription {
	return MapValues(n.Subscriptions, func(k string, v *NSQSub) PubsubSubscription {
		return v
	})
}

func (n *NSQTopic) DeleteSubscription(name string) {
	delete(n.Subscriptions, name)
}

type NSQSub struct {
	Name string `json:"name,omitempty"`
}

func (n *NSQSub) Validate(v *validator) {
	v.ValidateField("name", NotZero(n.Name))
}

// MarshalJSON custom marshaller for PubSub.
func (p *PubSub) MarshalJSON() ([]byte, error) {
	// Create a map to hold the JSON structure
	m := make(map[string]interface{})

	// Always add the type
	m["type"] = p.Type

	// Add the specific pubsub configuration based on the type
	switch p.Type {
	case "gcp_pubsub":
		if p.GCP != nil {
			for k, v := range structToMap(p.GCP) {
				m[k] = v
			}
		}
	case "aws_sns_sqs":
		if p.AWS != nil {
			for k, v := range structToMap(p.AWS) {
				m[k] = v
			}
		}
	case "nsq":
		if p.NSQ != nil {
			for k, v := range structToMap(p.NSQ) {
				m[k] = v
			}
		}
	default:
		return nil, errors.New("unsupported pubsub type")
	}

	return json.Marshal(m)
}

// structToMap converts a struct to a map[string]interface{}
// It uses json.Marshal and json.Unmarshal to avoid reflection
func structToMap(v interface{}) map[string]interface{} {
	data, _ := json.Marshal(v)
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	return m
}

// UnmarshalJSON custom unmarshaller for PubSub.
func (p *PubSub) UnmarshalJSON(data []byte) error {
	// Anonymous struct to capture the "type" field first.
	var aux struct {
		Type string `json:"type,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Set the Type field.
	p.Type = aux.Type

	// Unmarshal based on the "type" field.
	switch aux.Type {
	case "gcp_pubsub":
		var g GCPPubsub
		if err := json.Unmarshal(data, &g); err != nil {
			return err
		}
		p.GCP = &g
	case "aws_sns_sqs":
		var a AWSSNS_SQS
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		p.AWS = &a
	case "nsq":
		var n NSQPubsub
		if err := json.Unmarshal(data, &n); err != nil {
			return err
		}
		p.NSQ = &n
	default:
		return errors.New("unsupported pubsub type")
	}

	return nil
}

// Definitions of env_string and env_ref
type EnvString struct {
	Str string  `json:"string,omitempty"`
	Env *EnvRef `json:"env,omitempty"`
}

func (e *EnvString) IsEnvRef() bool {
	return e.Env != nil
}

// Value returns the resolved value of the EnvString.
// If Env is set, it returns the value of the environment variable.
// Otherwise, it returns the Str value.
func (e EnvString) Value() string {
	if e.Env != nil {
		return os.Getenv(e.Env.Env)
	}
	return e.Str
}

type EnvRef struct {
	Env string `json:"$env,omitempty"`
}

func (e *EnvRef) Describe(desc string) EnvDesc {
	return EnvDesc{
		Name:        e.Env,
		Description: desc,
	}
}

// UnmarshalJSON is the custom unmarshalling function for the EnvString type.
func (e *EnvString) UnmarshalJSON(data []byte) error {
	// Try unmarshalling into a simple string first.
	var simpleString string
	if err := json.Unmarshal(data, &simpleString); err == nil {
		e.Str = simpleString
		e.Env = nil
		return nil
	}

	// If it isn't a string, try unmarshalling into an EnvRef object.
	var envRef EnvRef
	if err := json.Unmarshal(data, &envRef); err == nil {
		e.Str = ""
		e.Env = &envRef
		return nil
	}

	// If neither works, return an error.
	return errors.New("invalid EnvString format")
}

// MarshalJSON is the custom marshaller function for the EnvString type (optional if needed).
func (e EnvString) MarshalJSON() ([]byte, error) {
	// If EnvRef is set, marshal as an object.
	if e.Env != nil {
		return json.Marshal(e.Env)
	}
	// Otherwise, marshal as a simple string.
	return json.Marshal(e.Str)
}

func MapValues[T comparable, V any, V2 any](m map[T]V, fn func(T, V) V2) map[T]V2 {
	res := make(map[T]V2)
	for k, v := range m {
		res[k] = fn(k, v)
	}
	return res
}
