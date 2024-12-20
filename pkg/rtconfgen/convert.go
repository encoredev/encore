package rtconfgen

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"

	"go.encore.dev/platform-sdk/pkg/auth"

	runtimev1 "encr.dev/proto/encore/runtime/v1"

	encore "encore.dev"
	"encore.dev/appruntime/exported/config"
	"encr.dev/pkg/fns"
)

func ToLegacy(conf *runtimev1.RuntimeConfig, secretEnvs map[string][]byte) (*config.Runtime, error) {
	conv := &legacyConverter{in: conf, secretEnvs: secretEnvs}
	return conv.Convert()
}

type legacyConverter struct {
	in         *runtimev1.RuntimeConfig
	secretEnvs map[string][]byte
	err        error
}

func findRID[T interface{ GetRid() string }](rid string, list []T) (T, bool) {
	return fns.Find(list, func(item T) bool {
		return item.GetRid() == rid
	})
}

func (c *legacyConverter) Convert() (*config.Runtime, error) {
	cfg := &config.Runtime{
		AppID:              c.in.Environment.AppId,
		AppSlug:            c.in.Environment.AppSlug,
		EnvID:              c.in.Environment.EnvId,
		EnvName:            c.in.Environment.EnvName,
		EnvType:            string(c.envType()),
		EnvCloud:           c.envCloud(),
		DeployID:           c.in.Deployment.DeployId,
		DeployedAt:         c.in.Deployment.DeployedAt.AsTime(),
		ServiceDiscovery:   nil,
		ServiceAuth:        nil,
		ShutdownTimeout:    0,
		GracefulShutdown:   nil,
		DynamicExperiments: nil,
		Gateways:           []config.Gateway{},
		PubsubTopics:       make(map[string]*config.PubsubTopic),
		Buckets:            make(map[string]*config.Bucket),
		CORS:               &config.CORS{},
	}

	// Deployment handling.
	{
		deployment := c.in.Deployment
		cfg.HostedServices = fns.Map(deployment.HostedServices, func(s *runtimev1.HostedService) string {
			return s.Name
		})

		cfg.ServiceAuth = fns.Map(deployment.AuthMethods, func(sa *runtimev1.ServiceAuth) config.ServiceAuth {
			switch sa.AuthMethod.(type) {
			case *runtimev1.ServiceAuth_EncoreAuth_:
				return config.ServiceAuth{Method: "encore-auth"}
			}
			return config.ServiceAuth{Method: "noop"}
		})

		cfg.ServiceDiscovery = make(map[string]config.Service)
		for key, value := range deployment.ServiceDiscovery.Services {
			method := config.ServiceAuth{Method: "noop"}
			if len(value.AuthMethods) > 0 {
				if _, ok := value.AuthMethods[0].AuthMethod.(*runtimev1.ServiceAuth_EncoreAuth_); ok {
					method.Method = "encore-auth"
				}
			}
			cfg.ServiceDiscovery[key] = config.Service{
				Name:        key,
				URL:         value.BaseUrl,
				Protocol:    config.Http,
				ServiceAuth: method,
			}
		}

		if deployment.GracefulShutdown != nil {
			cfg.GracefulShutdown = &config.GracefulShutdownTimings{
				Total:         ptr(deployment.GracefulShutdown.Total.AsDuration()),
				ShutdownHooks: ptr(deployment.GracefulShutdown.ShutdownHooks.AsDuration()),
				Handlers:      ptr(deployment.GracefulShutdown.Handlers.AsDuration()),
			}
			cfg.ShutdownTimeout = deployment.GracefulShutdown.Total.AsDuration()
		}
		cfg.DynamicExperiments = deployment.DynamicExperiments

		// Set the API Base URL if we have a gateway.
		if len(c.in.Infra.Resources.Gateways) > 0 {
			cfg.APIBaseURL = c.in.Infra.Resources.Gateways[0].BaseUrl
		}

		for _, gwRID := range deployment.HostedGateways {
			idx := slices.IndexFunc(c.in.Infra.Resources.Gateways, func(gw *runtimev1.Gateway) bool {
				return gw.Rid == gwRID
			})
			if idx >= 0 {
				gw := c.in.Infra.Resources.Gateways[idx]
				if gw.Cors != nil {
					var allowOriginsWithCredentials []string
					switch data := gw.Cors.AllowedOriginsWithCredentials.(type) {
					case *runtimev1.Gateway_CORS_AllowedOrigins:
						allowOriginsWithCredentials = data.AllowedOrigins.AllowedOrigins
					case *runtimev1.Gateway_CORS_UnsafeAllowAllOriginsWithCredentials:
						if data.UnsafeAllowAllOriginsWithCredentials {
							allowOriginsWithCredentials = []string{config.UnsafeAllOriginWithCredentials}
						}
					}
					cfg.CORS = &config.CORS{
						Debug:                          gw.Cors.Debug,
						DisableCredentials:             gw.Cors.DisableCredentials,
						AllowOriginsWithCredentials:    allowOriginsWithCredentials,
						AllowOriginsWithoutCredentials: gw.Cors.AllowedOriginsWithoutCredentials.AllowedOrigins,
						ExtraAllowedHeaders:            gw.Cors.ExtraAllowedHeaders,
						ExtraExposedHeaders:            gw.Cors.ExtraExposedHeaders,
						AllowPrivateNetworkAccess:      gw.Cors.AllowPrivateNetworkAccess,
					}
				}
				cfg.Gateways = append(cfg.Gateways, config.Gateway{
					Name: gw.EncoreName,
					Host: gw.Hostnames[0],
				})
			}
		}

		// Use the most verbose logging requested.
		currLevel := zerolog.PanicLevel
		foundLevel := false
		for _, svc := range deployment.HostedServices {
			if svc.LogConfig != nil {
				if level, err := zerolog.ParseLevel(*svc.LogConfig); err == nil && level < currLevel {
					currLevel = level
					foundLevel = true
				}
			}
		}
		if !foundLevel {
			currLevel = zerolog.TraceLevel
		}
		cfg.LogConfig = currLevel.String()
	}

	// Infrastructure handling.
	{
		res := c.in.Infra.Resources

		getClientCert := func(certKeyRID *string) (clientCertPEM, clientKeyPem string) {
			if certKeyRID == nil {
				return "", ""
			}
			cert, ok := findRID(*certKeyRID, c.in.Infra.Credentials.ClientCerts)
			if !ok {
				return "", ""
			}
			return cert.GetCert(), c.secretString(cert.GetKey())
		}

		// SQL Servers & Databases
		{
			for _, cluster := range res.SqlClusters {
				primary, ok := fns.Find(cluster.Servers, func(s *runtimev1.SQLServer) bool {
					return s.Kind == runtimev1.ServerKind_SERVER_KIND_PRIMARY
				})
				if !ok {
					c.setErrf("unable to find primary server for SQL cluster %q", cluster.Rid)
					continue
				}

				for _, db := range cluster.Databases {
					// Find the read-write connection pool.
					pool, ok := fns.Find(db.ConnPools, func(pool *runtimev1.SQLConnectionPool) bool {
						return !pool.IsReadonly
					})
					if !ok {
						// Use the first pool if none were read-write
						pool = db.ConnPools[0]
					}

					role, ok := findRID(pool.RoleRid, c.in.Infra.Credentials.SqlRoles)
					if !ok {
						c.setErrf("unable to find sql role %q", pool.RoleRid)
						continue
					}

					clientCert, clientKey := getClientCert(role.ClientCertRid)
					candidateServer := &config.SQLServer{
						Host:       primary.Host,
						ClientCert: clientCert,
						ClientKey:  clientKey,
					}
					if primary.TlsConfig != nil {
						candidateServer.ServerCACert = primary.TlsConfig.GetServerCaCert()
					}

					serverIdx := slices.IndexFunc(cfg.SQLServers, func(s *config.SQLServer) bool {
						return s.Host == candidateServer.Host &&
							s.ServerCACert == candidateServer.ServerCACert &&
							s.ClientCert == candidateServer.ClientCert &&
							s.ClientKey == candidateServer.ClientKey
					})
					if serverIdx == -1 {
						serverIdx = len(cfg.SQLServers)
						cfg.SQLServers = append(cfg.SQLServers, candidateServer)
					}

					cfg.SQLDatabases = append(cfg.SQLDatabases, &config.SQLDatabase{
						ServerID:       serverIdx,
						EncoreName:     db.EncoreName,
						DatabaseName:   db.CloudName,
						User:           role.Username,
						Password:       c.secretString(role.Password),
						MinConnections: int(pool.MinConnections),
						MaxConnections: int(pool.MaxConnections),
					})
				}
			}
		}

		// Redis Servers & Databases
		{
			for _, cluster := range res.RedisClusters {
				primary, ok := fns.Find(cluster.Servers, func(s *runtimev1.RedisServer) bool {
					return s.Kind == runtimev1.ServerKind_SERVER_KIND_PRIMARY
				})
				if !ok {
					c.setErrf("unable to find primary server for Redis cluster %q", cluster.Rid)
					continue
				}

				for _, db := range cluster.Databases {
					// Find the read-write connection pool.
					pool, ok := fns.Find(db.ConnPools, func(pool *runtimev1.RedisConnectionPool) bool {
						return !pool.IsReadonly
					})
					if !ok {
						// Use the first pool if none were read-write
						pool = db.ConnPools[0]
					}

					role, ok := findRID(pool.RoleRid, c.in.Infra.Credentials.RedisRoles)
					if !ok {
						c.setErrf("unable to find Redis role %q", pool.RoleRid)
						continue
					}

					user, password := func() (string, string) {
						switch s := role.Auth.(type) {
						case nil:
							return "", "" // no authentication
						case *runtimev1.RedisRole_Acl:
							return s.Acl.Username, c.secretString(s.Acl.Password)
						case *runtimev1.RedisRole_AuthString:
							return "", c.secretString(s.AuthString)
						default:
							c.setErrf("unknown redis auth type %T", s)
							return "", ""
						}
					}()

					clientCert, clientKey := getClientCert(role.ClientCertRid)
					candidateServer := &config.RedisServer{
						Host:       primary.Host,
						ClientCert: clientCert,
						ClientKey:  clientKey,
						User:       user,
						Password:   password,
					}
					if primary.TlsConfig != nil {
						candidateServer.EnableTLS = true
						candidateServer.ServerCACert = primary.TlsConfig.GetServerCaCert()
					}

					serverIdx := slices.IndexFunc(cfg.RedisServers, func(s *config.RedisServer) bool {
						return reflect.DeepEqual(s, candidateServer)
					})
					if serverIdx == -1 {
						serverIdx = len(cfg.RedisServers)
						cfg.RedisServers = append(cfg.RedisServers, candidateServer)
					}

					cfg.RedisDatabases = append(cfg.RedisDatabases, &config.RedisDatabase{
						ServerID:       serverIdx,
						EncoreName:     db.EncoreName,
						Database:       int(db.DatabaseIdx),
						MinConnections: int(pool.MinConnections),
						MaxConnections: int(pool.MaxConnections),
						KeyPrefix:      nilPtrToZero(db.KeyPrefix),
					})
				}
			}

		}

		// Pubsub Providers & Topics
		{
			for _, cluster := range c.in.Infra.Resources.PubsubClusters {
				p := &config.PubsubProvider{}
				switch prov := cluster.Provider.(type) {
				case *runtimev1.PubSubCluster_Encore:
					p.EncoreCloud = &config.EncoreCloudPubsubProvider{}
				case *runtimev1.PubSubCluster_Aws:
					p.AWS = &config.AWSPubsubProvider{}
				case *runtimev1.PubSubCluster_Gcp:
					p.GCP = &config.GCPPubsubProvider{}
				case *runtimev1.PubSubCluster_Nsq:
					p.NSQ = &config.NSQProvider{Host: prov.Nsq.Hosts[0]}
				case *runtimev1.PubSubCluster_Azure:
					p.Azure = &config.AzureServiceBusProvider{Namespace: prov.Azure.Namespace}
				default:
					c.setErrf("unknown pubsub provider type %T", prov)
					continue
				}

				providerID := len(cfg.PubsubProviders)
				cfg.PubsubProviders = append(cfg.PubsubProviders, p)
				for _, top := range cluster.Topics {
					cfg.PubsubTopics[top.EncoreName] = &config.PubsubTopic{
						EncoreName:    top.EncoreName,
						ProviderID:    providerID,
						ProviderName:  top.CloudName,
						Limiter:       nil, // TODO?
						Subscriptions: make(map[string]*config.PubsubSubscription),
						GCP: func() *config.PubsubTopicGCPData {
							switch pc := top.ProviderConfig.(type) {
							case *runtimev1.PubSubTopic_GcpConfig:
								return &config.PubsubTopicGCPData{
									ProjectID: pc.GcpConfig.ProjectId,
								}
							}
							return nil
						}(),
					}
				}

				for _, sub := range cluster.Subscriptions {
					topic := cfg.PubsubTopics[sub.TopicEncoreName]
					if topic == nil {
						// In the new config we could end up with a subscription where the
						// corresponding topic wasn't included, as we only include what's needed.
						// That doesn't work with the legacy metadata, so add it here in that case.
						topic = &config.PubsubTopic{
							EncoreName:    sub.TopicEncoreName,
							ProviderID:    providerID,
							ProviderName:  sub.TopicCloudName,
							Limiter:       nil,
							Subscriptions: make(map[string]*config.PubsubSubscription),
							GCP: func() *config.PubsubTopicGCPData {
								// HACK: this synthesizes a topic config based on the subscription's config.
								// That's not correct in the general case, but we only get here
								// if the service doesn't have access to the topic in the first place,
								// so this should be fine and prevents the runtime from exploding since
								// it doesn't expect to get a nil topic config.
								switch sub.ProviderConfig.(type) {
								case *runtimev1.PubSubSubscription_GcpConfig:
									return &config.PubsubTopicGCPData{
										ProjectID: "",
									}
								}
								return nil
							}(),
						}
						cfg.PubsubTopics[sub.TopicEncoreName] = topic
					}

					topic.Subscriptions[sub.SubscriptionEncoreName] = &config.PubsubSubscription{
						ID:           sub.Rid,
						EncoreName:   sub.SubscriptionEncoreName,
						ProviderName: sub.SubscriptionCloudName,
						PushOnly:     sub.PushOnly,
						GCP: func() *config.PubsubSubscriptionGCPData {
							switch pc := sub.ProviderConfig.(type) {
							case *runtimev1.PubSubSubscription_GcpConfig:
								return &config.PubsubSubscriptionGCPData{
									ProjectID:          pc.GcpConfig.ProjectId,
									PushServiceAccount: pc.GcpConfig.GetPushServiceAccount(),
								}
							}
							return nil
						}(),
					}
				}
			}
		}

		// Cloud Storage
		{
			for _, cluster := range c.in.Infra.Resources.BucketClusters {
				p := &config.BucketProvider{}
				switch prov := cluster.Provider.(type) {
				case *runtimev1.BucketCluster_S3_:
					p.S3 = &config.S3BucketProvider{
						Region:          prov.S3.GetRegion(),
						Endpoint:        prov.S3.Endpoint,
						AccessKeyID:     prov.S3.AccessKeyId,
						SecretAccessKey: ptrOrNil(c.secretString(prov.S3.SecretAccessKey)),
					}
				case *runtimev1.BucketCluster_Gcs:
					p.GCS = &config.GCSBucketProvider{
						Endpoint:  prov.Gcs.GetEndpoint(),
						Anonymous: prov.Gcs.Anonymous,
					}
					if opt := prov.Gcs.LocalSign; opt != nil {
						p.GCS.LocalSign = &config.GCSLocalSignOptions{
							BaseUrl:    opt.BaseUrl,
							AccessId:   opt.AccessId,
							PrivateKey: opt.PrivateKey,
						}
					}
				default:
					c.setErrf("unknown object storage provider type %T", prov)
					continue
				}

				providerID := len(cfg.BucketProviders)
				cfg.BucketProviders = append(cfg.BucketProviders, p)
				for _, bkt := range cluster.Buckets {
					cfg.Buckets[bkt.EncoreName] = &config.Bucket{
						ProviderID:    providerID,
						EncoreName:    bkt.EncoreName,
						CloudName:     bkt.CloudName,
						KeyPrefix:     bkt.GetKeyPrefix(),
						PublicBaseURL: bkt.GetPublicBaseUrl(),
					}
				}

			}
		}
	}

	// Observability.
	{
		obs := c.in.Deployment.Observability
		if len(obs.Metrics) > 0 {
			prov := obs.Metrics[0]
			m := &config.Metrics{
				CollectionInterval: prov.CollectionInterval.AsDuration(),
			}
			switch p := prov.Provider.(type) {
			case *runtimev1.MetricsProvider_Gcp:
				pp := p.Gcp
				m.CloudMonitoring = &config.GCPCloudMonitoringProvider{
					ProjectID:               pp.ProjectId,
					MonitoredResourceType:   pp.MonitoredResourceType,
					MonitoredResourceLabels: pp.MonitoredResourceLabels,
					MetricNames:             pp.MetricNames,
				}
			case *runtimev1.MetricsProvider_EncoreCloud:
				pp := p.EncoreCloud
				m.EncoreCloud = &config.GCPCloudMonitoringProvider{
					ProjectID:               pp.ProjectId,
					MonitoredResourceType:   pp.MonitoredResourceType,
					MonitoredResourceLabels: pp.MonitoredResourceLabels,
					MetricNames:             pp.MetricNames,
				}
			case *runtimev1.MetricsProvider_Aws:
				pp := p.Aws
				m.CloudWatch = &config.AWSCloudWatchMetricsProvider{Namespace: pp.Namespace}
			case *runtimev1.MetricsProvider_Datadog_:
				pp := p.Datadog
				m.Datadog = &config.DatadogProvider{
					APIKey: c.secretString(pp.ApiKey),
					Site:   pp.Site,
				}

			case *runtimev1.MetricsProvider_PromRemoteWrite:
				pp := p.PromRemoteWrite
				m.Prometheus = &config.PrometheusRemoteWriteProvider{
					RemoteWriteURL: c.secretString(pp.RemoteWriteUrl),
				}

			default:
				c.setErrf("unknown metrics provider type %T", p)
			}

			cfg.Metrics = m
		}

		// Add the Encore Tracing endpoint, if any.
		for _, prov := range obs.Tracing {
			if enc := prov.GetEncore(); enc != nil {
				cfg.TraceEndpoint = enc.TraceEndpoint
				cfg.TraceSamplingRate = enc.SamplingRate
				break
			}
		}
	}

	if c.in.EncorePlatform != nil {
		cfg.AuthKeys = c.authKeys(c.in.EncorePlatform.PlatformSigningKeys)
	}

	if ec := c.in.EncorePlatform.GetEncoreCloud(); ec != nil {
		cfg.EncoreCloudAPI = &config.EncoreCloudAPI{
			Server: ec.ServerUrl,
			AuthKeys: fns.Map(c.authKeys(ec.AuthKeys), func(k config.EncoreAuthKey) auth.Key {
				return auth.Key{KeyID: k.KeyID, Data: k.Data}
			}),
		}
	}

	if c.err != nil {
		return nil, c.err
	}
	return cfg, nil
}

func (c *legacyConverter) envType() encore.EnvironmentType {
	switch c.in.Environment.EnvType {
	case runtimev1.Environment_TYPE_DEVELOPMENT:
		return encore.EnvDevelopment
	case runtimev1.Environment_TYPE_PRODUCTION:
		return encore.EnvProduction
	case runtimev1.Environment_TYPE_EPHEMERAL:
		return encore.EnvEphemeral
	case runtimev1.Environment_TYPE_TEST:
		return encore.EnvTest
	default:
		c.setErrf("unknown environment type %+v", c.in.Environment.EnvType)
		return ""
	}
}

func (c *legacyConverter) envCloud() encore.CloudProvider {
	switch c.in.Environment.Cloud {
	case runtimev1.Environment_CLOUD_LOCAL:
		return encore.CloudLocal
	case runtimev1.Environment_CLOUD_AWS:
		return encore.CloudAWS
	case runtimev1.Environment_CLOUD_GCP:
		return encore.CloudGCP
	case runtimev1.Environment_CLOUD_AZURE:
		return encore.CloudAzure
	case runtimev1.Environment_CLOUD_ENCORE:
		return encore.EncoreCloud
	default:
		c.setErrf("unknown cloud %+v", c.in.Environment.Cloud)
		return ""
	}
}

func (c *legacyConverter) authKeys(keys []*runtimev1.EncoreAuthKey) []config.EncoreAuthKey {
	return fns.Map(keys, func(k *runtimev1.EncoreAuthKey) config.EncoreAuthKey {
		return config.EncoreAuthKey{
			KeyID: k.Id,
			Data:  c.secretBytes(k.Data),
		}
	})
}

func (c *legacyConverter) limiter(lim *runtimev1.RateLimiter) *config.Limiter {
	if lim == nil {
		return nil
	}

	switch lim := lim.Kind.(type) {
	case *runtimev1.RateLimiter_TokenBucket_:
		return &config.Limiter{
			TokenBucket: &config.TokenBucketLimiter{
				PerSecondRate: lim.TokenBucket.Rate,
				BucketSize:    int(lim.TokenBucket.Burst),
			},
		}
	default:
		c.setErrf("unknown rate limiter type %T", lim)
		return nil
	}
}

func (c *legacyConverter) secretString(s *runtimev1.SecretData) string {
	return string(c.secretBytes(s))
}

func (c *legacyConverter) secretBytes(s *runtimev1.SecretData) []byte {
	if s == nil {
		return nil
	}

	// First resolve the secret data
	var secretData []byte
	switch data := s.Source.(type) {
	case *runtimev1.SecretData_Embedded:
		secretData = data.Embedded
	case *runtimev1.SecretData_Env:
		val, ok := c.secretEnvs[data.Env]
		if !ok {
			c.setErrf("missing secret env var %q", data.Env)
		}
		secretData = val
	default:
		c.setErrf("unknown secret data type %T", data)
		return nil
	}

	// Resolve a sub-path, if any.
	switch sub := s.SubPath.(type) {
	case nil:
		// No sub-path defined.
		return secretData

	case *runtimev1.SecretData_JsonKey:
		jsonObj := map[string]any{}
		if err := json.Unmarshal(secretData, &jsonObj); err != nil {
			c.setErrf("secret data is not valid json: %v", err)
			return nil
		}
		val, ok := jsonObj[sub.JsonKey]
		if !ok {
			c.setErrf("missing json key %q", sub.JsonKey)
		}
		switch val := val.(type) {
		case string:
			return []byte(val)
		case map[string]any:
			baseVal, ok := val["bytes"]
			if !ok {
				panic("missing bytes key")
			}
			b64Str, ok := baseVal.(string)
			if !ok {
				panic("bytes key is not a string")
			}
			bytes, err := base64.StdEncoding.DecodeString(b64Str)
			if err != nil {
				panic(err)
			}
			return bytes
		default:
			panic("unexpected value type")
		}
	default:
		c.setErrf("unknown secret sub-path type %T", s)
		return nil
	}
}

func (c *legacyConverter) setErr(err error) {
	if c.err == nil {
		c.err = err
	}
}

func (c *legacyConverter) setErrf(format string, args ...any) {
	c.setErr(errors.Newf(format, args...))
}

func nilPtrToZero[T comparable](val *T) T {
	if val == nil {
		var zero T
		return zero
	}
	return *val
}

func ptr[T comparable](val T) *T {
	return &val
}

func ptrOrNil[T comparable](val T) *T {
	var zero T
	if val == zero {
		return nil
	}
	return &val
}

func randomMapValue[K comparable, V any](m map[K]V) (V, bool) {
	for _, v := range m {
		return v, true
	}
	var zero V
	return zero, false
}
