use crate::encore::runtime::v1::infrastructure::{Credentials, Resources};
use crate::encore::runtime::v1::{
    self as pbruntime, environment, gateway, metrics_provider, pub_sub_cluster,
    pub_sub_subscription, pub_sub_topic, redis_role, secret_data, service_auth, service_discovery,
    AppSecret, Deployment, Environment, Infrastructure, MetricsProvider, Observability,
    PubSubCluster, PubSubSubscription, PubSubTopic, RedisCluster, RedisConnectionPool,
    RedisDatabase, RedisRole, RedisServer, RuntimeConfig, SqlCluster, SqlConnectionPool,
    SqlDatabase, SqlRole, SqlServer, TlsConfig,
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Serialize, Deserialize)]
pub struct InfraConfig {
    pub metadata: Option<Metadata>,
    pub graceful_shutdown: Option<GracefulShutdown>,
    pub auth: Option<Vec<Auth>>,
    pub service_discovery: Option<HashMap<String, ServiceDiscovery>>,
    pub metrics: Option<Metrics>,
    pub sql_servers: Option<Vec<SQLServer>>,
    pub redis: Option<HashMap<String, Redis>>,
    pub pubsub: Option<Vec<PubSub>>,
    pub secrets: Option<Secrets>,
    pub hosted_services: Option<Vec<String>>,
    pub hosted_gateways: Option<Vec<String>>,
    pub cors: Option<CORS>,
    pub object_storage: Option<Vec<ObjectStorage>>,
    pub worker_threads: Option<i32>,
    pub log_config: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum ObjectStorage {
    #[serde(rename = "gcs")]
    GCS(GCS),
    #[serde(rename = "s3")]
    S3(S3),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GCS {
    pub endpoint: Option<String>,
    pub buckets: HashMap<String, Bucket>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct S3 {
    pub region: String,
    pub endpoint: Option<String>,
    pub access_key_id: Option<String>,
    pub secret_access_key: Option<EnvString>,
    pub buckets: HashMap<String, Bucket>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Bucket {
    pub name: String,
    pub key_prefix: Option<String>,
    pub public_base_url: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Default)]
pub struct Metadata {
    pub app_id: Option<String>,
    pub env_name: Option<String>,
    pub env_type: Option<String>,
    pub cloud: Option<String>,
    pub base_url: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CORS {
    pub debug: Option<bool>,
    pub allow_headers: Option<Vec<String>>,
    pub expose_headers: Option<Vec<String>>,
    pub allow_origins_without_credentials: Option<Vec<String>>,
    pub allow_origins_with_credentials: Option<Vec<String>>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GracefulShutdown {
    pub total: Option<i32>,

    pub shutdown_hooks: Option<i32>,

    pub handlers: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum Auth {
    #[serde(rename = "key")]
    Key(KeyAuth),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct KeyAuth {
    pub id: i32,
    pub key: EnvString,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ServiceDiscovery {
    pub base_url: String,

    pub auth: Option<Vec<Auth>>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum Metrics {
    #[serde(rename = "prometheus")]
    Prometheus(PrometheusMetrics),
    #[serde(rename = "datadog")]
    Datadog(DatadogMetrics),
    #[serde(rename = "gcp_cloud_monitoring")]
    GCPCloudMonitoring(GCPCloudMonitoringMetrics),
    #[serde(rename = "aws_cloudwatch")]
    AWSCloudWatch(AWSCloudWatchMetrics),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PrometheusMetrics {
    pub collection_interval: Option<i32>,
    pub remote_write_url: EnvString,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct DatadogMetrics {
    pub collection_interval: Option<i32>,
    pub site: String,
    pub api_key: EnvString,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GCPCloudMonitoringMetrics {
    pub collection_interval: Option<i32>,
    pub project_id: String,
    pub monitored_resource_type: String,
    pub monitored_resource_labels: Option<HashMap<String, String>>,
    pub metric_names: Option<HashMap<String, String>>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AWSCloudWatchMetrics {
    pub collection_interval: Option<i32>,
    pub namespace: String,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(untagged)]
pub enum Secrets {
    Map(HashMap<String, EnvString>),
    EnvRef(EnvRef),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct EnvRef {
    #[serde(rename = "$env")]
    pub env: String,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(untagged)]
pub enum EnvString {
    String(String),
    EnvRef(EnvRef),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SQLServer {
    pub host: String,
    pub tls_config: Option<TLSConfig>,
    pub databases: HashMap<String, SQLDatabase>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct TLSConfig {
    #[serde(default)]
    pub disabled: bool,
    pub ca: Option<String>,
    pub client_cert: Option<ClientCert>,
    #[serde(default)]
    pub disable_tls_hostname_verification: bool,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SQLDatabase {
    pub max_connections: Option<i32>,
    pub min_connections: Option<i32>,
    pub username: String,
    pub password: EnvString,
    pub client_cert: Option<ClientCert>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Redis {
    pub host: String,
    pub database_index: i32,

    pub auth: Option<RedisAuth>,

    pub key_prefix: Option<String>,

    pub tls_config: Option<TLSConfig>,

    pub max_connections: Option<i32>,

    pub min_connections: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct RedisAuth {
    pub r#type: String,

    pub username: Option<String>,

    pub password: Option<EnvString>,

    pub auth_string: Option<EnvString>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ClientCert {
    pub cert: String,
    pub key: EnvString,
}

// PubSub-related structures

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum PubSub {
    #[serde(rename = "gcp_pubsub")]
    GCPPubsub(GCPPubsub),
    #[serde(rename = "aws_sns_sqs")]
    AWSSnsSqs(AWSSnsSqs),
    #[serde(rename = "nsq")]
    NSQ(NSQPubsub),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GCPPubsub {
    pub project_id: String,
    pub topics: HashMap<String, GCPTopic>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GCPTopic {
    pub name: String,

    pub project_id: Option<String>,
    pub subscriptions: HashMap<String, GCPSub>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GCPSub {
    pub name: String,

    pub project_id: Option<String>,

    pub push_config: Option<PushConfig>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PushConfig {
    pub service_account: String,
    pub jwt_audience: String,
    pub id: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AWSSnsSqs {
    pub topics: HashMap<String, AWSTopic>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AWSTopic {
    pub arn: String,
    pub subscriptions: HashMap<String, AWSSub>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AWSSub {
    pub arn: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NSQPubsub {
    pub hosts: String,
    pub topics: HashMap<String, NSQTopic>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NSQTopic {
    pub name: String,
    #[serde(skip_serializing_if = "HashMap::is_empty")]
    pub subscriptions: HashMap<String, NSQSub>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NSQSub {
    pub name: String,
}

pub fn map_infra_to_runtime(infra: InfraConfig) -> RuntimeConfig {
    let mut next_rid = 0;
    let mut get_next_rid = || {
        let rid = next_rid;
        next_rid += 1;
        rid.to_string()
    };

    let metadata = infra.metadata.unwrap_or_default();
    // Map the Environment
    let environment = Some(Environment {
        app_id: "".to_string(),
        app_slug: metadata.app_id.unwrap_or_default(),
        env_id: "".to_string(),
        env_name: metadata.env_name.unwrap_or_default(),
        env_type: metadata
            .env_type
            .as_ref()
            .map(|t| match t.as_str() {
                "development" => environment::Type::Development as i32,
                "production" => environment::Type::Production as i32,
                "ephemeral" => environment::Type::Ephemeral as i32,
                "test" => environment::Type::Test as i32,
                _ => environment::Type::Unspecified as i32,
            })
            .unwrap_or(environment::Type::Unspecified as i32),
        cloud: metadata
            .cloud
            .as_ref()
            .map(|c| match c.as_str() {
                "local" => environment::Cloud::Local as i32,
                "encore" => environment::Cloud::Encore as i32,
                "aws" => environment::Cloud::Aws as i32,
                "gcp" => environment::Cloud::Gcp as i32,
                "azure" => environment::Cloud::Azure as i32,
                _ => environment::Cloud::Unspecified as i32,
            })
            .unwrap_or(environment::Cloud::Unspecified as i32),
    });

    // Map GracefulShutdown
    let graceful_shutdown =
        infra
            .graceful_shutdown
            .as_ref()
            .map(|gs| pbruntime::GracefulShutdown {
                total: gs.total.map(|t| prost_types::Duration {
                    seconds: t as i64,
                    nanos: 0,
                }),
                shutdown_hooks: gs.shutdown_hooks.map(|t| prost_types::Duration {
                    seconds: t as i64,
                    nanos: 0,
                }),
                handlers: gs.handlers.map(|t| prost_types::Duration {
                    seconds: t as i64,
                    nanos: 0,
                }),
            });

    // Map Auth methods
    let auth_methods = infra
        .auth
        .as_ref()
        .map(|auths| {
            auths
                .iter()
                .map(|auth| {
                    let auth_method = match auth {
                        Auth::Key(k) => {
                            service_auth::AuthMethod::EncoreAuth(service_auth::EncoreAuth {
                                auth_keys: vec![pbruntime::EncoreAuthKey {
                                    id: k.id as u32,
                                    data: Some(map_env_string_to_secret_data(&k.key)),
                                }],
                            })
                        }
                    };
                    pbruntime::ServiceAuth {
                        auth_method: Some(auth_method),
                    }
                })
                .collect()
        })
        .unwrap_or_else(|| {
            vec![pbruntime::ServiceAuth {
                auth_method: Some(service_auth::AuthMethod::Noop(service_auth::NoopAuth {})),
            }]
        });

    // Map ServiceDiscovery
    let service_discovery = infra.service_discovery.map(|services| {
        let services_mapped = services
            .into_iter()
            .map(|(name, sd)| {
                let svc_auth_methods = sd
                    .auth
                    .map(|auths| {
                        auths
                            .into_iter()
                            .map(|auth| match auth {
                                Auth::Key(k) => pbruntime::ServiceAuth {
                                    auth_method: Some(service_auth::AuthMethod::EncoreAuth(
                                        service_auth::EncoreAuth {
                                            auth_keys: vec![pbruntime::EncoreAuthKey {
                                                id: k.id as u32,
                                                data: Some(map_env_string_to_secret_data(&k.key)),
                                            }],
                                        },
                                    )),
                                },
                            })
                            .collect()
                    })
                    .unwrap_or(auth_methods.clone());
                (
                    name,
                    service_discovery::Location {
                        base_url: sd.base_url,
                        auth_methods: svc_auth_methods,
                    },
                )
            })
            .collect();

        pbruntime::ServiceDiscovery {
            services: services_mapped,
        }
    });

    // Map Buckets
    let buckets = infra.object_storage.map(|object_storages| {
        object_storages
            .into_iter()
            .map(|os| match os {
                ObjectStorage::GCS(gcs) => pbruntime::BucketCluster {
                    rid: get_next_rid(),
                    provider: Some(pbruntime::bucket_cluster::Provider::Gcs(
                        pbruntime::bucket_cluster::Gcs {
                            endpoint: gcs.endpoint,
                            anonymous: false,
                            local_sign: None,
                        },
                    )),
                    buckets: gcs
                        .buckets
                        .into_iter()
                        .map(|(name, bucket)| pbruntime::Bucket {
                            encore_name: name,
                            cloud_name: bucket.name,
                            key_prefix: bucket.key_prefix,
                            public_base_url: bucket.public_base_url,
                            rid: get_next_rid(),
                        })
                        .collect(),
                },
                ObjectStorage::S3(s3) => pbruntime::BucketCluster {
                    rid: get_next_rid(),
                    provider: Some(pbruntime::bucket_cluster::Provider::S3(
                        pbruntime::bucket_cluster::S3 {
                            region: s3.region,
                            endpoint: s3.endpoint,
                            access_key_id: s3.access_key_id,
                            secret_access_key: s3
                                .secret_access_key
                                .as_ref()
                                .map(map_env_string_to_secret_data),
                        },
                    )),
                    buckets: s3
                        .buckets
                        .into_iter()
                        .map(|(name, bucket)| pbruntime::Bucket {
                            encore_name: name,
                            cloud_name: bucket.name,
                            key_prefix: bucket.key_prefix,
                            public_base_url: bucket.public_base_url,
                            rid: get_next_rid(),
                        })
                        .collect(),
                },
            })
            .collect()
    });

    // Map Metrics
    let metrics = infra.metrics.map(|metrics| {
        let (provider, interval) = match metrics {
            Metrics::Prometheus(pm) => (
                metrics_provider::Provider::PromRemoteWrite(
                    metrics_provider::PrometheusRemoteWrite {
                        remote_write_url: Some(map_env_string_to_secret_data(&pm.remote_write_url)),
                    },
                ),
                pm.collection_interval,
            ),
            Metrics::Datadog(dd) => (
                metrics_provider::Provider::Datadog(metrics_provider::Datadog {
                    site: dd.site,
                    api_key: Some(map_env_string_to_secret_data(&dd.api_key)),
                }),
                dd.collection_interval,
            ),
            Metrics::GCPCloudMonitoring(gcp) => (
                metrics_provider::Provider::Gcp(metrics_provider::GcpCloudMonitoring {
                    project_id: gcp.project_id,
                    monitored_resource_type: gcp.monitored_resource_type,
                    monitored_resource_labels: gcp.monitored_resource_labels.unwrap_or_default(),
                    metric_names: gcp.metric_names.unwrap_or_default(),
                }),
                gcp.collection_interval,
            ),
            Metrics::AWSCloudWatch(aws) => (
                metrics_provider::Provider::Aws(metrics_provider::AwsCloudWatch {
                    namespace: aws.namespace,
                }),
                aws.collection_interval,
            ),
        };

        vec![MetricsProvider {
            rid: get_next_rid(),
            collection_interval: interval.map(|i| prost_types::Duration {
                seconds: i as i64,
                nanos: 0,
            }),
            provider: Some(provider),
        }]
    });

    // Map Observability
    let observability = Some(Observability {
        metrics: metrics.unwrap_or_default(),
        tracing: Vec::new(),
        logs: Vec::new(),
    });

    let cors = infra.cors.map(|cors| gateway::Cors {
        debug: cors.debug.unwrap_or(false),
        disable_credentials: false,
        allowed_origins_without_credentials: cors
            .allow_origins_without_credentials
            .map(|f| gateway::CorsAllowedOrigins { allowed_origins: f }),
        allowed_origins_with_credentials: cors.allow_origins_with_credentials.map(|f| {
            gateway::cors::AllowedOriginsWithCredentials::AllowedOrigins(
                gateway::CorsAllowedOrigins { allowed_origins: f },
            )
        }),
        extra_allowed_headers: cors.allow_headers.unwrap_or_default(),
        extra_exposed_headers: cors.expose_headers.unwrap_or_default(),
        allow_private_network_access: true,
    });

    let gateways = infra
        .hosted_gateways
        .map(|gateways| {
            gateways
                .into_iter()
                .map(|gateway| pbruntime::Gateway {
                    rid: get_next_rid(),
                    encore_name: gateway,
                    base_url: metadata.base_url.clone().unwrap_or_default(),
                    hostnames: vec![],
                    cors: cors.clone(),
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();

    // Map Deployment
    let deployment = Some(Deployment {
        deploy_id: String::new(),
        deployed_at: None,
        dynamic_experiments: Vec::new(),
        hosted_gateways: gateways.iter().map(|g| g.rid.clone()).collect(),
        hosted_services: infra
            .hosted_services
            .map(|services| {
                services
                    .iter()
                    .map(|service| pbruntime::HostedService {
                        name: service.clone(),
                        worker_threads: infra.worker_threads,
                        log_config: infra.log_config.clone(),
                    })
                    .collect()
            })
            .unwrap_or_default(),
        auth_methods,
        observability,
        service_discovery,
        graceful_shutdown,
    });

    let mut credentials = Credentials {
        client_certs: Vec::new(),
        sql_roles: Vec::new(),
        redis_roles: Vec::new(),
    };

    // Map SQL Servers
    let sql_clusters = infra.sql_servers.map(|servers| {
        servers
            .into_iter()
            .map(|server| {
                let default_client_cert = server
                    .tls_config
                    .as_ref()
                    .and_then(|tls| tls.client_cert.as_ref())
                    .map(|f| {
                        let rid = get_next_rid();
                        let client_cert = pbruntime::ClientCert {
                            rid: rid.clone(),
                            cert: f.cert.clone(),
                            key: Some(map_env_string_to_secret_data(&f.key)),
                        };
                        credentials.client_certs.push(client_cert);
                        rid
                    });

                let databases = server
                    .databases
                    .into_iter()
                    .map(|(name, db)| {
                        let client_cert = db
                            .client_cert
                            .map(|f| {
                                let rid = get_next_rid();
                                let client_cert = pbruntime::ClientCert {
                                    rid: rid.clone(),
                                    cert: f.cert,
                                    key: Some(map_env_string_to_secret_data(&f.key)),
                                };
                                credentials.client_certs.push(client_cert);
                                rid
                            })
                            .or_else(|| default_client_cert.clone());
                        let role_rid = get_next_rid();
                        let role = SqlRole {
                            rid: role_rid.clone(),
                            client_cert_rid: client_cert,
                            username: db.username,
                            password: Some(map_env_string_to_secret_data(&db.password)),
                        };
                        credentials.sql_roles.push(role);
                        SqlDatabase {
                            rid: get_next_rid(),
                            encore_name: name.clone(),
                            cloud_name: name,
                            conn_pools: vec![SqlConnectionPool {
                                is_readonly: false,
                                role_rid,
                                min_connections: db.min_connections.unwrap_or(0),
                                max_connections: db.max_connections.unwrap_or(100),
                            }],
                        }
                    })
                    .collect();

                SqlCluster {
                    rid: get_next_rid(),
                    servers: vec![SqlServer {
                        rid: get_next_rid(),
                        host: server.host,
                        kind: pbruntime::ServerKind::Primary as i32,
                        tls_config: server.tls_config.map_or_else(
                            || Some(TlsConfig::default()),
                            |tls| match tls.disabled {
                                true => None,
                                false => Some(TlsConfig {
                                    server_ca_cert: tls.ca,
                                    disable_tls_hostname_verification: tls
                                        .disable_tls_hostname_verification,
                                }),
                            },
                        ),
                    }],
                    databases,
                }
            })
            .collect()
    });

    // Map Redis
    let redis_clusters = infra.redis.map(|redis_map| {
        redis_map
            .into_iter()
            .map(|(name, redis)| {
                let client_cert = redis
                    .tls_config
                    .as_ref()
                    .and_then(|tls| tls.client_cert.as_ref())
                    .map(|f| {
                        let rid = get_next_rid();
                        let client_cert = pbruntime::ClientCert {
                            rid: rid.clone(),
                            cert: f.cert.clone(),
                            key: Some(map_env_string_to_secret_data(&f.key)),
                        };
                        credentials.client_certs.push(client_cert);
                        rid
                    });
                let auth = redis.auth.map(|ra| match ra.r#type.as_str() {
                    "auth_string" => redis_role::Auth::AuthString(map_env_string_to_secret_data(
                        ra.auth_string.as_ref().unwrap(),
                    )),
                    "acl" => redis_role::Auth::Acl(redis_role::AuthAcl {
                        username: ra.username.unwrap(),
                        password: Some(map_env_string_to_secret_data(
                            ra.password.as_ref().unwrap(),
                        )),
                    }),
                    _ => redis_role::Auth::AuthString(map_env_string_to_secret_data(
                        ra.auth_string.as_ref().unwrap(),
                    )),
                });

                let role_rid = get_next_rid();
                let role = RedisRole {
                    rid: role_rid.clone(),
                    client_cert_rid: client_cert,
                    auth,
                };
                credentials.redis_roles.push(role);
                let database = RedisDatabase {
                    rid: get_next_rid(),
                    encore_name: name, // Use the key as the name
                    database_idx: redis.database_index,
                    key_prefix: redis.key_prefix,
                    conn_pools: vec![RedisConnectionPool {
                        is_readonly: false,
                        role_rid,
                        min_connections: redis.min_connections.unwrap_or(0),
                        max_connections: redis.max_connections.unwrap_or(100),
                    }],
                };

                RedisCluster {
                    rid: String::new(), // Assign a unique RID
                    servers: vec![RedisServer {
                        rid: String::new(), // Assign a unique RID
                        host: redis.host,
                        kind: pbruntime::ServerKind::Primary as i32,
                        tls_config: redis.tls_config.map_or_else(
                            || Some(TlsConfig::default()),
                            |tls| match tls.disabled {
                                true => None,
                                false => Some(TlsConfig {
                                    server_ca_cert: tls.ca,
                                    disable_tls_hostname_verification: tls
                                        .disable_tls_hostname_verification,
                                }),
                            },
                        ),
                    }],
                    databases: vec![database],
                }
            })
            .collect()
    });

    // Map PubSub
    let pubsub_clusters = infra.pubsub.map(|pubsubs| {
        pubsubs
            .into_iter()
            .map(|pubsub| {
                // Handle different PubSub types
                let (provider, topics, subscriptions) = match pubsub {
                    PubSub::GCPPubsub(gcp) => {
                        let topics = gcp
                            .topics
                            .iter()
                            .map(|(name, topic)| PubSubTopic {
                                rid: String::new(),
                                encore_name: name.clone(),
                                cloud_name: topic.name.clone(),
                                delivery_guarantee: pub_sub_topic::DeliveryGuarantee::AtLeastOnce
                                    as i32,
                                ordering_attr: None,
                                provider_config: Some(pub_sub_topic::ProviderConfig::GcpConfig(
                                    pub_sub_topic::GcpConfig {
                                        project_id: topic
                                            .project_id
                                            .clone()
                                            .unwrap_or_else(|| gcp.project_id.clone()),
                                    },
                                )),
                            })
                            .collect();

                        let subscriptions = gcp
                            .topics
                            .iter()
                            .flat_map(|(topic_name, topic)| {
                                topic.subscriptions.iter().map(|(sub_name, sub)| {
                                    PubSubSubscription {
                                        rid: String::new(),
                                        topic_encore_name: topic_name.clone(),
                                        subscription_encore_name: sub_name.clone(),
                                        topic_cloud_name: topic.name.clone(),
                                        subscription_cloud_name: sub.name.clone(),
                                        push_only: sub.push_config.is_some(),
                                        provider_config: sub.push_config.as_ref().map(|pc| {
                                            pub_sub_subscription::ProviderConfig::GcpConfig(
                                                pub_sub_subscription::GcpConfig {
                                                    project_id: sub
                                                        .project_id
                                                        .clone()
                                                        .unwrap_or_else(|| gcp.project_id.clone()),
                                                    push_service_account: Some(
                                                        pc.service_account.clone(),
                                                    ),
                                                    push_jwt_audience: Some(
                                                        pc.jwt_audience.clone(),
                                                    ),
                                                },
                                            )
                                        }),
                                    }
                                })
                            })
                            .collect();

                        let provider =
                            pub_sub_cluster::Provider::Gcp(pub_sub_cluster::GcpPubSub {});
                        (Some(provider), topics, subscriptions)
                    }
                    PubSub::AWSSnsSqs(aws) => {
                        let topics = aws
                            .topics
                            .iter()
                            .map(|(name, topic)| PubSubTopic {
                                rid: String::new(),
                                encore_name: name.clone(),
                                cloud_name: topic.arn.clone(),
                                delivery_guarantee: pub_sub_topic::DeliveryGuarantee::AtLeastOnce
                                    as i32, // AWS typically provides at-least-once delivery
                                ordering_attr: None, // Add ordering if necessary
                                provider_config: None, // AWS doesn't need additional provider config here
                            })
                            .collect();

                        let subscriptions = aws
                            .topics
                            .iter()
                            .flat_map(|(topic_name, topic)| {
                                topic.subscriptions.iter().map(|(sub_name, sub)| {
                                    PubSubSubscription {
                                        rid: String::new(),
                                        topic_encore_name: topic_name.clone(),
                                        subscription_encore_name: sub_name.clone(),
                                        topic_cloud_name: topic.arn.clone(),
                                        subscription_cloud_name: sub.arn.clone(),
                                        push_only: false, // AWS SQS doesn't typically use push config
                                        provider_config: None, // AWS doesn't need additional provider config
                                    }
                                })
                            })
                            .collect();

                        let provider =
                            pub_sub_cluster::Provider::Aws(pub_sub_cluster::AwsSqsSns {});

                        (Some(provider), topics, subscriptions)
                    }
                    PubSub::NSQ(nsq) => {
                        let topics = nsq
                            .topics
                            .iter()
                            .map(|(name, topic)| PubSubTopic {
                                rid: String::new(),
                                encore_name: name.clone(),
                                cloud_name: topic.name.clone(), // NSQ doesn't have cloud-specific names, using the topic name
                                delivery_guarantee: pub_sub_topic::DeliveryGuarantee::AtLeastOnce
                                    as i32, // NSQ typically guarantees at-least-once delivery
                                ordering_attr: None, // NSQ doesn't handle message ordering natively
                                provider_config: None, // No additional provider config for NSQ
                            })
                            .collect();

                        let subscriptions = nsq
                            .topics
                            .iter()
                            .flat_map(|(topic_name, topic)| {
                                topic.subscriptions.iter().map(|(sub_name, sub)| {
                                    PubSubSubscription {
                                        rid: String::new(),
                                        topic_encore_name: topic_name.clone(),
                                        subscription_encore_name: sub_name.clone(),
                                        topic_cloud_name: topic.name.clone(), // Using topic name for simplicity
                                        subscription_cloud_name: sub.name.clone(),
                                        push_only: false, // NSQ is pull-based, no push config
                                        provider_config: None, // No additional provider config for NSQ
                                    }
                                })
                            })
                            .collect();

                        let provider = pub_sub_cluster::Provider::Nsq(pub_sub_cluster::Nsq {
                            hosts: vec![nsq.hosts.clone()], // Mapping NSQ hosts
                        });

                        (Some(provider), topics, subscriptions)
                    }
                };

                PubSubCluster {
                    rid: get_next_rid(),
                    topics,
                    subscriptions,
                    provider,
                }
            })
            .collect()
    });

    // Map Secrets
    let app_secrets: Vec<AppSecret> = match infra.secrets {
        Some(Secrets::Map(secrets_map)) => secrets_map
            .into_iter()
            .map(|(name, value)| AppSecret {
                rid: get_next_rid(),
                encore_name: name.clone(),
                data: Some(map_env_string_to_secret_data(&value)),
            })
            .collect(),
        Some(Secrets::EnvRef(env_ref)) => {
            // Fetch the environment variable
            match std::env::var(env_ref.env) {
                Ok(secrets_json) => {
                    // Parse the JSON string into a HashMap
                    match serde_json::from_str::<HashMap<String, String>>(&secrets_json) {
                        Ok(secrets_map) => secrets_map
                            .into_iter()
                            .map(|(name, value)| AppSecret {
                                rid: get_next_rid(),
                                encore_name: name,
                                data: Some(pbruntime::SecretData {
                                    encoding: secret_data::Encoding::None as i32,
                                    source: Some(secret_data::Source::Embedded(value.into_bytes())),
                                    sub_path: None,
                                }),
                            })
                            .collect(),
                        Err(_) => {
                            ::log::error!(
                                "Failed to parse secrets JSON from secret environment variable"
                            );
                            Vec::new()
                        }
                    }
                }
                Err(_) => {
                    ::log::error!("Failed to read secrets from environment variable");
                    Vec::new()
                }
            }
        }
        None => Vec::new(),
    };

    // Map Infrastructure Resources
    let resources = Some(Resources {
        gateways,
        sql_clusters: sql_clusters.unwrap_or_default(),
        pubsub_clusters: pubsub_clusters.unwrap_or_default(),
        redis_clusters: redis_clusters.unwrap_or_default(),
        app_secrets,
        bucket_clusters: buckets.unwrap_or_default(),
    });

    let infra_struct = Some(Infrastructure {
        resources,
        credentials: Some(credentials),
    });

    // Construct the final RuntimeConfig
    RuntimeConfig {
        environment,
        infra: infra_struct,
        deployment,
        encore_platform: None,
    }
}

// Helper function to map EnvString to SecretData
fn map_env_string_to_secret_data(env_string: &EnvString) -> pbruntime::SecretData {
    match env_string {
        EnvString::String(s) => pbruntime::SecretData {
            encoding: secret_data::Encoding::None as i32,
            source: Some(secret_data::Source::Embedded(s.clone().into_bytes())),
            sub_path: None,
        },
        EnvString::EnvRef(env_ref) => pbruntime::SecretData {
            encoding: secret_data::Encoding::None as i32,
            source: Some(secret_data::Source::Env(env_ref.env.clone())),
            sub_path: None,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use prost::Message;
    use serde_json;
    use std::fs;

    #[test]
    fn test_map_infra_to_runtime() {
        // Load and parse the infra.config.json fixture
        let infra_json = fs::read_to_string(format!(
            "{}/resources/test/infra.config.json",
            env!("CARGO_MANIFEST_DIR")
        ))
        .expect("Failed to read infra.config.json");
        let infra_config: InfraConfig =
            serde_json::from_str(&infra_json).expect("Failed to parse infra.config.json");

        // Convert InfraConfig to Runtime
        let runtime: RuntimeConfig = map_infra_to_runtime(infra_config);

        // Load and parse the runtime.json fixture
        let runtime_data = fs::read(format!(
            "{}/resources/test/runtime.pb",
            env!("CARGO_MANIFEST_DIR")
        ))
        .expect("Failed to read runtime.json");
        let expected_runtime =
            RuntimeConfig::decode(runtime_data.as_slice()).expect("Failed to parse runtime.json");

        // Compare the converted runtime with the expected runtime
        assert_eq!(
            runtime, expected_runtime,
            "Converted runtime does not match expected runtime"
        );
    }
}
