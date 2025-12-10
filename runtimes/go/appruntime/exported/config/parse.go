package config

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"encore.dev/appruntime/exported/config/infra"
)

func gunzip(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(gz)
}

type ProcessConfig struct {
	HostedServices    []string       `json:"hosted_services"`
	HostedGateways    []string       `json:"hosted_gateways"`
	LocalServicePorts map[string]int `json:"local_service_ports"`
}

func parseRuntimeConfigEnv(config string) *Runtime {
	var cfg Runtime
	// We used to support RawURLEncoding, but now we use StdEncoding.
	// Try both if StdEncoding fails.
	var (
		bytes []byte
		err   error
	)
	config, isGzipped := strings.CutPrefix(config, "gzip:")
	// nosemgrep
	if bytes, err = base64.StdEncoding.DecodeString(config); err != nil {
		bytes, err = base64.RawURLEncoding.DecodeString(config)
	}
	if err != nil {
		log.Fatalln("encore runtime: fatal error: could not decode encore runtime config:", err)
	}
	if isGzipped {
		if bytes, err = gunzip(bytes); err != nil {
			log.Fatalln("encore runtime: fatal error: could not gunzip encore runtime config:", err)
		}
	}
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse encore runtime config:", err)
	}

	if _, err := url.Parse(cfg.APIBaseURL); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse api base url from encore runtime config:", err)
	}
	return &cfg
}

func parseProcessConfigEnv(processCfg string, cfg *Runtime) {
	if processCfg == "" {
		return
	}
	bytes, err := base64.StdEncoding.DecodeString(processCfg)
	if err != nil {
		log.Fatalln("encore runtime: fatal error: could not decode encore process config:", err)
	}
	var procCfg ProcessConfig
	if err := json.Unmarshal(bytes, &procCfg); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse encore process config:", err)
	}
	cfg.HostedServices = procCfg.HostedServices
	var hostedGateways []Gateway
	for _, name := range procCfg.HostedGateways {
		i := slices.IndexFunc(cfg.Gateways, func(gw Gateway) bool { return gw.Name == name })
		if i == -1 {
			log.Fatalf("encore runtime: fatal error: gateway %q not found in runtime config", name)
		}
		hostedGateways = append(hostedGateways, cfg.Gateways[i])
	}
	cfg.Gateways = hostedGateways

	// Use noop service auth method if not specified
	svcAuth := ServiceAuth{"noop"}
	if len(cfg.ServiceAuth) > 0 {
		// Use the first service auth method from the runtime config
		svcAuth = cfg.ServiceAuth[0]
	}

	for name, port := range procCfg.LocalServicePorts {
		if cfg.ServiceDiscovery == nil {
			cfg.ServiceDiscovery = make(map[string]Service)
		}
		cfg.ServiceDiscovery[name] = Service{
			Name:        name,
			URL:         fmt.Sprintf("http://localhost:%d", port),
			Protocol:    Http,
			ServiceAuth: svcAuth,
		}
	}
}

func toPtr[T any](t T) *T {
	return &t
}

func LoadInfraConfig(infraCfgPath string) (*infra.InfraConfig, error) {
	var envCfg infra.InfraConfig
	file, err := os.Open(infraCfgPath)
	if err != nil {
		return nil, fmt.Errorf("could not open infra config: %w", err)
	}
	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&envCfg)
	if err != nil {
		return nil, fmt.Errorf("could not decode infra config: %w", err)
	}
	return &envCfg, nil
}

func LoadRuntimeConfig(runtimeCfgPath string) (*Runtime, error) {
	var cfg Runtime
	file, err := os.Open(runtimeCfgPath)
	if err != nil {
		return nil, fmt.Errorf("could not open runtime config: %w", err)
	}
	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("could not decode runtime config: %w", err)
	}
	return &cfg, nil
}

func parseInfraConfigEnv(infraCfgPath string) *Runtime {
	var cfg Runtime
	infraCfg, err := LoadInfraConfig(infraCfgPath)
	if err != nil {
		log.Fatalf("encore runtime: fatal error: %v", err)
	}

	cfg.AppSlug = infraCfg.Metadata.AppID
	cfg.EnvName = infraCfg.Metadata.EnvName
	cfg.EnvType = infraCfg.Metadata.EnvType
	cfg.EnvCloud = infraCfg.Metadata.Cloud
	cfg.APIBaseURL = infraCfg.Metadata.BaseURL
	cfg.LogConfig = infraCfg.LogConfig

	// Map graceful shutdown configuration
	if infraCfg.GracefulShutdown != nil {
		cfg.GracefulShutdown = &GracefulShutdownTimings{}
		if infraCfg.GracefulShutdown.Total != nil {
			cfg.GracefulShutdown.Total = toPtr(time.Duration(*infraCfg.GracefulShutdown.Total) * time.Second)
		}
		if infraCfg.GracefulShutdown.ShutdownHooks != nil {
			cfg.GracefulShutdown.ShutdownHooks = toPtr(time.Duration(*infraCfg.GracefulShutdown.ShutdownHooks) * time.Second)
		}
		if infraCfg.GracefulShutdown.Handlers != nil {
			cfg.GracefulShutdown.Handlers = toPtr(time.Duration(*infraCfg.GracefulShutdown.Handlers) * time.Second)
		}
	}

	// Map authentication configuration
	cfg.ServiceAuth = make([]ServiceAuth, len(infraCfg.Auth))
	if len(infraCfg.Auth) == 0 {
		cfg.ServiceAuth = []ServiceAuth{{Method: "noop"}}
	}
	for i, auth := range infraCfg.Auth {
		switch auth.Type {
		case "key":
			cfg.ServiceAuth[i] = ServiceAuth{
				Method: "encore-auth",
			}
			cfg.AuthKeys = append(cfg.AuthKeys, EncoreAuthKey{
				KeyID: uint32(auth.ID),
				Data:  []byte(auth.Key.Value()),
			})
		default:
			log.Fatalf("encore runtime: fatal error: unsupported auth type %q", auth.Type)
		}
	}

	// Map metrics configuration
	if infraCfg.Metrics != nil {
		cfg.Metrics = &Metrics{
			CollectionInterval: time.Duration(infraCfg.Metrics.CollectionInterval) * time.Second,
		}
		switch infraCfg.Metrics.Type {
		case "prometheus":
			if infraCfg.Metrics.Prometheus != nil {
				cfg.Metrics.Prometheus = &PrometheusRemoteWriteProvider{
					infraCfg.Metrics.Prometheus.RemoteWriteURL.Value(),
				}
			}
		case "datadog":
			if infraCfg.Metrics.Datadog != nil {
				cfg.Metrics.Datadog = &DatadogProvider{
					infraCfg.Metrics.Datadog.Site,
					infraCfg.Metrics.Datadog.APIKey.Value(),
				}
			}
		case "gcp_cloud_monitoring":
			if infraCfg.Metrics.GCPCloudMonitoring != nil {
				cfg.Metrics.CloudMonitoring = &GCPCloudMonitoringProvider{
					infraCfg.Metrics.GCPCloudMonitoring.ProjectID,
					infraCfg.Metrics.GCPCloudMonitoring.MonitoredResourceType,
					infraCfg.Metrics.GCPCloudMonitoring.MonitoredResourceLabels,
					infraCfg.Metrics.GCPCloudMonitoring.MetricNames,
				}
			}
		case "aws_cloudwatch":
			if infraCfg.Metrics.AWSCloudWatch != nil {
				cfg.Metrics.CloudWatch = &AWSCloudWatchMetricsProvider{
					infraCfg.Metrics.AWSCloudWatch.Namespace,
				}
			}
		}
	}

	// Map SQL servers configuration
	cfg.SQLServers = make([]*SQLServer, len(infraCfg.SQLServers))
	for i, sqlServer := range infraCfg.SQLServers {
		cfg.SQLServers[i] = &SQLServer{
			Host: sqlServer.Host,
		}
		if sqlServer.TLSConfig != nil {
			cfg.SQLServers[i].ServerCACert = sqlServer.TLSConfig.CA
			if sqlServer.TLSConfig.ClientCert != nil {
				cfg.SQLServers[i].ClientCert = sqlServer.TLSConfig.ClientCert.Cert
				cfg.SQLServers[i].ClientKey = sqlServer.TLSConfig.ClientCert.Key.Value()
			}
		}

		for dbName, db := range sqlServer.Databases {
			cfg.SQLDatabases = append(cfg.SQLDatabases, &SQLDatabase{
				ServerID:       i,
				EncoreName:     orDefault(db.Name, dbName),
				DatabaseName:   dbName,
				User:           db.Username.Value(),
				Password:       db.Password.Value(),
				MinConnections: db.MinConnections,
				MaxConnections: db.MaxConnections,
			})
		}
	}

	// Map Redis configuration
	cfg.RedisServers = make([]*RedisServer, len(infraCfg.Redis))
	var i int
	for name, redis := range infraCfg.Redis {
		cfg.RedisServers[i] = &RedisServer{
			Host: redis.Host,
		}
		if redis.TLSConfig != nil {
			cfg.RedisServers[i].EnableTLS = true
			cfg.RedisServers[i].ServerCACert = redis.TLSConfig.CA
			if redis.TLSConfig.ClientCert != nil {
				cfg.RedisServers[i].ClientCert = redis.TLSConfig.ClientCert.Cert
				cfg.RedisServers[i].ClientKey = redis.TLSConfig.ClientCert.Key.Value()
			}
		}
		if redis.Auth != nil {
			switch redis.Auth.Type {
			case "acl":
				cfg.RedisServers[i].User = redis.Auth.Username.Value()
				cfg.RedisServers[i].Password = redis.Auth.Password.Value()
			case "auth":
				cfg.RedisServers[i].Password = redis.Auth.AuthString.Value()
			default:
				log.Fatalf("encore runtime: fatal error: unsupported redis auth type %q", redis.Auth.Type)
			}
		}
		cfg.RedisDatabases = append(cfg.RedisDatabases, &RedisDatabase{
			ServerID:       i,
			EncoreName:     name,
			Database:       redis.DatabaseIndex,
			MinConnections: orDefaultPtr(redis.MinConnections, 0),
			MaxConnections: orDefaultPtr(redis.MaxConnections, 0),
			KeyPrefix:      orDefaultPtr(redis.KeyPrefix, ""),
		})
		i++
	}

	// Map PubSub configuration
	cfg.PubsubProviders = make([]*PubsubProvider, len(infraCfg.PubSub))
	for i, pubsub := range infraCfg.PubSub {
		switch pubsub.Type {
		case "gcp_pubsub":
			cfg.PubsubProviders[i] = &PubsubProvider{
				GCP: &GCPPubsubProvider{},
			}
		case "aws_sns_sqs":
			cfg.PubsubProviders[i] = &PubsubProvider{
				AWS: &AWSPubsubProvider{},
			}
		case "nsq":
			cfg.PubsubProviders[i] = &PubsubProvider{
				NSQ: &NSQProvider{
					Host: pubsub.NSQ.Hosts,
				},
			}
		}
		cfg.PubsubTopics = map[string]*PubsubTopic{}
		for topicName, topic := range pubsub.GetTopics() {
			switch topic := topic.(type) {
			case *infra.GCPTopic:
				cfg.PubsubTopics[topicName] = &PubsubTopic{
					EncoreName:    topicName,
					ProviderID:    i,
					ProviderName:  topic.Name,
					Subscriptions: map[string]*PubsubSubscription{},
					GCP: &PubsubTopicGCPData{
						ProjectID: orDefault(topic.ProjectID, pubsub.GCP.ProjectID),
					},
				}
			case *infra.AWSTopic:
				cfg.PubsubTopics[topicName] = &PubsubTopic{
					EncoreName:    topicName,
					ProviderID:    i,
					ProviderName:  topic.ARN,
					Subscriptions: map[string]*PubsubSubscription{},
				}
			case *infra.NSQTopic:
				cfg.PubsubTopics[topicName] = &PubsubTopic{
					EncoreName:    topicName,
					ProviderID:    i,
					ProviderName:  topic.Name,
					Subscriptions: map[string]*PubsubSubscription{},
				}
			}

			for subName, subscription := range topic.GetSubscriptions() {
				switch subscription := subscription.(type) {
				case *infra.GCPSub:
					sub := &PubsubSubscription{
						EncoreName:   subName,
						ProviderName: subscription.Name,
						PushOnly:     subscription.PushConfig != nil,
						GCP:          &PubsubSubscriptionGCPData{ProjectID: orDefault(subscription.ProjectID, pubsub.GCP.ProjectID)},
					}
					if subscription.PushConfig != nil {
						sub.ID = subscription.PushConfig.ID
						sub.GCP.PushServiceAccount = subscription.PushConfig.ServiceAccount
					}
					cfg.PubsubTopics[topicName].Subscriptions[subName] = sub
				case *infra.AWSSub:
					cfg.PubsubTopics[topicName].Subscriptions[subName] = &PubsubSubscription{
						EncoreName:   subName,
						ProviderName: subscription.URL,
						PushOnly:     false,
					}
				case *infra.NSQSub:
					cfg.PubsubTopics[topicName].Subscriptions[subName] = &PubsubSubscription{
						EncoreName:   subName,
						ProviderName: subscription.Name,
						PushOnly:     false,
					}
				}
			}
		}
	}

	// Map Service Discovery configuration
	cfg.ServiceDiscovery = make(map[string]Service)
	for name, service := range infraCfg.ServiceDiscovery {
		cfg.ServiceDiscovery[name] = Service{
			Name:        name,
			URL:         service.BaseURL,
			Protocol:    Http,
			ServiceAuth: cfg.ServiceAuth[0],
		}
	}

	// Map Buckets
	cfg.BucketProviders = make([]*BucketProvider, len(infraCfg.ObjectStorage))
	for i, storage := range infraCfg.ObjectStorage {
		switch storage.Type {
		case "gcs":
			cfg.BucketProviders[i] = &BucketProvider{
				GCS: &GCSBucketProvider{
					Endpoint: storage.GCS.Endpoint,
				},
			}
		case "s3":
			cfg.BucketProviders[i] = &BucketProvider{
				S3: &S3BucketProvider{
					Region:          storage.S3.Region,
					Endpoint:        nilOr(storage.S3.Endpoint),
					AccessKeyID:     nilOr(storage.S3.AccessKeyID),
					SecretAccessKey: nilOr(storage.S3.SecretAccessKey.Value()),
				},
			}
		}
		cfg.Buckets = map[string]*Bucket{}
		for bucketName, bucket := range storage.GetBuckets() {
			cfg.Buckets[bucketName] = &Bucket{
				ProviderID:    i,
				EncoreName:    bucketName,
				CloudName:     bucket.Name,
				KeyPrefix:     bucket.KeyPrefix,
				PublicBaseURL: bucket.PublicBaseURL,
			}
		}
	}

	if infraCfg.CORS != nil {
		cfg.CORS = &CORS{
			Debug:                          infraCfg.CORS.Debug,
			DisableCredentials:             false,
			AllowOriginsWithCredentials:    infraCfg.CORS.AllowOriginsWithCredentials,
			AllowOriginsWithoutCredentials: infraCfg.CORS.AllowOriginsWithoutCredentials,
			ExtraAllowedHeaders:            infraCfg.CORS.AllowHeaders,
			ExtraExposedHeaders:            infraCfg.CORS.ExposeHeaders,
			AllowPrivateNetworkAccess:      true,
		}
	}
	// Map hosted services
	cfg.HostedServices = infraCfg.HostedServices
	cfg.Gateways = make([]Gateway, len(infraCfg.HostedGateways))
	for i, gw := range infraCfg.HostedGateways {
		cfg.Gateways[i] = Gateway{
			Name: gw,
		}
	}
	return &cfg
}

func nilOr[T comparable](val T) *T {
	var zero T
	if val == zero {
		return nil
	}
	return &val

}

func orDefaultPtr[T any](val *T, def T) T {
	if val == nil {
		return def
	}
	return *val
}

func orDefault[T comparable](val T, def T) T {
	var zero T
	if val == zero {
		return def
	}
	return val
}

// ParseRuntime parses the Encore runtime config.
func ParseRuntime(runtimeConfig, runtimeConfigPath, processCfg, infraCfgPath, deployID string) *Runtime {
	var cfg *Runtime
	if infraCfgPath != "" {
		cfg = parseInfraConfigEnv(infraCfgPath)
	} else if runtimeConfig != "" {
		cfg = parseRuntimeConfigEnv(runtimeConfig)
	} else if runtimeConfigPath != "" {
		var err error
		cfg, err = LoadRuntimeConfig(runtimeConfigPath)
		if err != nil {
			log.Fatalln("encore runtime: fatal error: could not load encore runtime config:", err)
		}
	} else {
		log.Fatalln("encore runtime: fatal error: no encore runtime or infra config provided")
	}
	parseProcessConfigEnv(processCfg, cfg)

	// If the environment deploy ID is set, use that instead of the one
	// embedded in the runtime config
	if deployID != "" {
		cfg.DeployID = deployID
	}

	return cfg
}

// ParseStatic parses the Encore static config.
func ParseStatic(config string) *Static {
	if config == "" {
		log.Fatalln("encore runtime: fatal error: no encore static config provided")
	}
	bytes, err := base64.StdEncoding.DecodeString(config)
	if err != nil {
		log.Fatalln("encore runtime: fatal error: could not decode encore static config:", err)
	}
	var cfg Static
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse encore static config:", err)
	}
	return &cfg
}
