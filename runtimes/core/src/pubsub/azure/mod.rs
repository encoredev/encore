use std::sync::Arc;

use anyhow::Context;
use azservicebus::client::service_bus_client::ServiceBusClientOptions;
use azservicebus::core::BasicRetryPolicy;
use azservicebus::prelude::ServiceBusClient;
use azure_core::credentials::TokenCredential;
use azure_identity::DefaultAzureCredential;

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as pb;
use crate::pubsub;
use crate::pubsub::azure::sub::Subscription;
use crate::pubsub::azure::topic::Topic;

pub(super) mod sub;
pub(super) mod topic;

/// The concrete Azure Service Bus client type using the default retry policy.
pub(super) type AzureClient = ServiceBusClient<BasicRetryPolicy>;

#[derive(Debug)]
pub struct Cluster {
    client: Arc<LazyAzureClient>,
}

impl Cluster {
    pub fn new(cfg: &pb::pub_sub_cluster::AzureServiceBus) -> Self {
        Self {
            client: Arc::new(LazyAzureClient::new(cfg.namespace.clone())),
        }
    }
}

impl pubsub::Cluster for Cluster {
    fn topic(
        &self,
        cfg: &pb::PubSubTopic,
        _publisher_id: xid::Id,
    ) -> Arc<dyn pubsub::Topic + 'static> {
        Arc::new(Topic::new(self.client.clone(), cfg))
    }

    fn subscription(
        &self,
        cfg: &pb::PubSubSubscription,
        meta: &meta::pub_sub_topic::Subscription,
    ) -> Arc<dyn pubsub::Subscription + 'static> {
        Arc::new(Subscription::new(self.client.clone(), cfg, meta))
    }
}

/// Lazily initialises an Azure Service Bus client, wrapped in an Arc<Mutex<...>>
/// so that it can be shared and mutated across async tasks.
#[derive(Debug)]
pub(super) struct LazyAzureClient {
    namespace: String,
    cell: tokio::sync::OnceCell<anyhow::Result<Arc<tokio::sync::Mutex<AzureClient>>>>,
}

impl LazyAzureClient {
    fn new(namespace: String) -> Self {
        Self {
            namespace,
            cell: tokio::sync::OnceCell::new(),
        }
    }

    pub(super) async fn get(
        &self,
    ) -> &anyhow::Result<Arc<tokio::sync::Mutex<AzureClient>>> {
        self.cell
            .get_or_init(|| async {
                // DefaultAzureCredential::new() returns Arc<DefaultAzureCredential> directly.
                //
                // NOTE: azure_identity 0.25 DefaultAzureCredential only tries Azure CLI and
                // Azure Developer CLI credentials.  For production environments using Managed
                // Identity, upgrade to a newer azure_identity release that includes
                // ManagedIdentityCredential, or supply a connection string via
                // ServiceBusClient::new_from_connection_string instead.
                let credential: Arc<dyn TokenCredential> = DefaultAzureCredential::new()
                    .context("failed to create Azure DefaultAzureCredential")?;

                let fqn = format!("{}.servicebus.windows.net", self.namespace);
                let client = AzureClient::new_from_token_credential(
                    fqn,
                    credential,
                    ServiceBusClientOptions::default(),
                )
                .await
                .context("failed to create Azure Service Bus client")?;

                Ok(Arc::new(tokio::sync::Mutex::new(client)))
            })
            .await
    }
}
