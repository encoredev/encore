use std::sync::Arc;

use anyhow::Context;
use azure_storage::StorageCredentials;
use azure_storage_blobs::prelude::BlobServiceClient;

use crate::encore::runtime::v1 as pb;
use crate::objects;
use crate::objects::azblob::bucket::Bucket;
use crate::secrets::Secret;

pub(super) mod bucket;

#[derive(Debug)]
pub struct Cluster {
    client: Arc<LazyAzBlobClient>,
}

impl Cluster {
    pub fn new(cfg: pb::bucket_cluster::AzBlob, storage_key: Option<Secret>) -> Self {
        let client = Arc::new(LazyAzBlobClient::new(cfg, storage_key));

        // Begin initializing the client in the background.
        tokio::spawn(client.clone().begin_initialize());

        Self { client }
    }
}

impl objects::ClusterImpl for Cluster {
    fn bucket(self: Arc<Self>, cfg: &pb::Bucket) -> Arc<dyn objects::BucketImpl + 'static> {
        Arc::new(Bucket::new(self.client.clone(), cfg))
    }
}

pub(super) struct ClientState {
    pub service_client: BlobServiceClient,
    /// Raw storage account key (not base64-decoded), used for SAS URL signing.
    /// None when using managed identity (token credential).
    pub storage_key: Option<String>,
    pub account_name: String,
}

impl std::fmt::Debug for ClientState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("ClientState")
            .field("account_name", &self.account_name)
            .finish()
    }
}

pub(super) struct LazyAzBlobClient {
    cfg: pb::bucket_cluster::AzBlob,
    storage_key: Option<Secret>,
    cell: tokio::sync::OnceCell<anyhow::Result<ClientState>>,
}

impl std::fmt::Debug for LazyAzBlobClient {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("LazyAzBlobClient")
            .field("account", &self.cfg.storage_account)
            .finish()
    }
}

impl LazyAzBlobClient {
    fn new(cfg: pb::bucket_cluster::AzBlob, storage_key: Option<Secret>) -> Self {
        Self {
            cfg,
            storage_key,
            cell: tokio::sync::OnceCell::new(),
        }
    }

    pub async fn get(&self) -> &anyhow::Result<ClientState> {
        self.cell
            .get_or_init(|| initialize(&self.cfg, self.storage_key.as_ref()))
            .await
    }

    async fn begin_initialize(self: Arc<Self>) {
        self.get().await;
    }
}

async fn initialize(
    cfg: &pb::bucket_cluster::AzBlob,
    storage_key: Option<&Secret>,
) -> anyhow::Result<ClientState> {
    if let Some(conn_str) = &cfg.connection_string {
        // Parse the connection string using the azure_storage SDK.
        let parsed = azure_storage::ConnectionString::new(conn_str)
            .context("failed to parse Azure storage connection string")?;

        let account_name = parsed
            .account_name
            .map(|s| s.to_string())
            .unwrap_or_else(|| cfg.storage_account.clone());

        let account_key = parsed.account_key.map(|k| k.to_string());

        let credentials = parsed
            .storage_credentials()
            .context("failed to extract credentials from Azure connection string")?;

        let service_client = BlobServiceClient::new(&account_name, credentials);

        return Ok(ClientState {
            service_client,
            storage_key: account_key,
            account_name,
        });
    }

    if let Some(secret) = storage_key {
        let key_bytes = secret
            .get()
            .context("failed to resolve Azure storage key secret")?;
        let key_str = std::str::from_utf8(key_bytes)
            .context("Azure storage key is not valid UTF-8")?
            .to_string();

        let credentials = StorageCredentials::access_key(
            cfg.storage_account.clone(),
            key_str.clone(),
        );
        let service_client = BlobServiceClient::new(&cfg.storage_account, credentials);

        return Ok(ClientState {
            service_client,
            storage_key: Some(key_str),
            account_name: cfg.storage_account.clone(),
        });
    }

    // No explicit credentials: managed identity auth is not directly available
    // because azure_storage_blobs 0.21 uses azure_core 0.21 while azure_identity
    // 0.25 uses azure_core 0.25 — they carry incompatible TokenCredential traits.
    //
    // Workaround: provide a storage_key or connection_string.
    // Alternatively, set the AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_KEY environment
    // variables; the connection-string path above will pick them up if the caller
    // passes a connection string built from those variables.
    //
    // TODO: once azure_storage_blobs is updated to use azure_core 0.25, replace
    // this error with DefaultAzureCredential::new() and StorageCredentials::token_credential.
    Err(anyhow::anyhow!(
        "azure blob: managed identity authentication requires either a 'storage_key' secret or \
         a 'connection_string' to be configured.  Direct DefaultAzureCredential support is not \
         yet available due to an azure_storage_blobs/azure_identity SDK version mismatch."
    ))
}
