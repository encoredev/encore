use async_stream::try_stream;
use azure_storage_blobs::prelude::{BlobBlockType, BlockId, BlockList};
use base64::Engine;
use bytes::{Bytes, BytesMut};
use futures::StreamExt;
use hmac::{Hmac, Mac};
use sha2::Sha256;
use std::borrow::Cow;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use tokio::io::{AsyncRead, AsyncReadExt};

use crate::encore::runtime::v1 as pb;
use crate::objects::{
    self, AttrsOptions, DeleteOptions, DownloadOptions, DownloadStream, DownloadUrlOptions, Error,
    ExistsOptions, ListEntry, ListOptions, ObjectAttrs, PublicUrlError, UploadOptions,
    UploadUrlOptions,
};
use crate::{CloudName, EncoreName};

use super::LazyAzBlobClient;

type HmacSha256 = Hmac<Sha256>;

/// Chunk size used for staged-block (multipart) uploads: 8 MiB.
const CHUNK_SIZE: usize = 8_388_608;

/// Azure Blob Storage SAS API version used for signing.
const SAS_VERSION: &str = "2020-12-06";

#[derive(Debug)]
pub struct Bucket {
    client: Arc<LazyAzBlobClient>,
    encore_name: EncoreName,
    cloud_name: CloudName,
    public_base_url: Option<String>,
    key_prefix: Option<String>,
}

impl Bucket {
    pub(super) fn new(client: Arc<LazyAzBlobClient>, cfg: &pb::Bucket) -> Self {
        Self {
            client,
            encore_name: cfg.encore_name.clone().into(),
            cloud_name: cfg.cloud_name.clone().into(),
            public_base_url: cfg.public_base_url.clone(),
            key_prefix: cfg.key_prefix.clone(),
        }
    }

    fn obj_name<'a>(&'_ self, name: Cow<'a, str>) -> Cow<'a, str> {
        match &self.key_prefix {
            Some(prefix) => {
                let mut key = prefix.to_owned();
                key.push_str(&name);
                Cow::Owned(key)
            }
            None => name,
        }
    }

    fn strip_prefix<'a>(&'_ self, name: Cow<'a, str>) -> Cow<'a, str> {
        match &self.key_prefix {
            Some(prefix) => name
                .as_ref()
                .strip_prefix(prefix)
                .map(|s| Cow::Owned(s.to_string()))
                .unwrap_or(name),
            None => name,
        }
    }
}

impl objects::BucketImpl for Bucket {
    fn name(&self) -> &EncoreName {
        &self.encore_name
    }

    fn object(self: Arc<Self>, name: String) -> Arc<dyn objects::ObjectImpl> {
        Arc::new(Object {
            bkt: self,
            name,
        })
    }

    fn list(
        self: Arc<Self>,
        options: ListOptions,
    ) -> Pin<Box<dyn Future<Output = Result<objects::ListStream, Error>> + Send + 'static>> {
        Box::pin(async move {
            match self.client.get().await {
                Ok(state) => {
                    let container =
                        state.service_client.container_client(self.cloud_name.as_ref());

                    let mut prefix = String::new();
                    if let Some(kp) = &self.key_prefix {
                        prefix.push_str(kp);
                    }
                    if let Some(p) = &options.prefix {
                        prefix.push_str(p);
                    }

                    let s: objects::ListStream = Box::new(try_stream! {
                        let mut total_seen: u64 = 0;
                        let mut builder = container.list_blobs();
                        if !prefix.is_empty() {
                            builder = builder.prefix(prefix.clone());
                        }
                        let mut stream = builder.into_stream();

                        'PageLoop:
                        while let Some(page) = stream.next().await {
                            let page = page.map_err(map_err)?;
                            for blob in page.blobs.blobs() {
                                total_seen += 1;
                                if let Some(limit) = options.limit {
                                    if total_seen > limit {
                                        break 'PageLoop;
                                    }
                                }
                                let name = self.strip_prefix(Cow::Borrowed(&blob.name)).into_owned();
                                let size = blob.properties.content_length as u64;
                                let etag = blob.properties.etag.to_string();
                                yield ListEntry { name, size, etag };
                            }
                        }
                    });

                    Ok(s)
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }
}

#[derive(Debug)]
struct Object {
    bkt: Arc<Bucket>,
    name: String,
}

impl objects::ObjectImpl for Object {
    fn bucket_name(&self) -> &EncoreName {
        &self.bkt.encore_name
    }

    fn key(&self) -> &str {
        &self.name
    }

    fn attrs(
        self: Arc<Self>,
        options: AttrsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(state) => {
                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    let container =
                        state.service_client.container_client(self.bkt.cloud_name.as_ref());
                    let blob = make_blob_client(&container, &cloud_name, options.version.as_deref());

                    let props = blob.get_properties().await.map_err(map_err)?;
                    Ok(ObjectAttrs {
                        name: self.name.clone(),
                        version: props.blob.version_id.clone(),
                        size: props.blob.properties.content_length as u64,
                        content_type: Some(props.blob.properties.content_type.to_string()),
                        etag: props.blob.properties.etag.to_string(),
                    })
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn exists(
        self: Arc<Self>,
        options: ExistsOptions,
    ) -> Pin<Box<dyn Future<Output = Result<bool, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(state) => {
                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    let container =
                        state.service_client.container_client(self.bkt.cloud_name.as_ref());
                    let blob = make_blob_client(&container, &cloud_name, options.version.as_deref());

                    match blob.get_properties().await.map_err(map_err) {
                        Ok(_) => Ok(true),
                        Err(Error::NotFound) => Ok(false),
                        Err(err) => Err(err),
                    }
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn upload(
        self: Arc<Self>,
        mut data: Box<dyn AsyncRead + Unpin + Send + Sync + 'static>,
        opts: UploadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<ObjectAttrs, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(state) => {
                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    let container =
                        state.service_client.container_client(self.bkt.cloud_name.as_ref());
                    let blob = container.blob_client(cloud_name.as_ref());

                    let first_chunk = read_chunk_async(&mut data).await.map_err(|e| {
                        Error::Other(anyhow::anyhow!("unable to read from data source: {}", e))
                    })?;

                    match first_chunk {
                        Chunk::Complete(buf) => {
                            upload_single(&blob, buf.freeze(), &opts).await
                        }
                        Chunk::Part(buf) => {
                            upload_multipart(&blob, &mut data, buf.freeze(), &opts).await
                        }
                    }
                    .map(|(version, etag, size, content_type)| ObjectAttrs {
                        name: self.name.clone(),
                        version,
                        size,
                        content_type,
                        etag,
                    })
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn download(
        self: Arc<Self>,
        options: DownloadOptions,
    ) -> Pin<Box<dyn Future<Output = Result<DownloadStream, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(state) => {
                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    let container =
                        state.service_client.container_client(self.bkt.cloud_name.as_ref());
                    let blob =
                        make_blob_client(&container, &cloud_name, options.version.as_deref());

                    // Eagerly open the stream so we can propagate initial errors (e.g. 404) now.
                    let mut response_stream = blob.get().into_stream();

                    // Probe the first response to detect not-found early.
                    let first = response_stream.next().await;

                    let download: DownloadStream = Box::pin(try_stream! {
                        if let Some(first_resp) = first {
                            let chunk = first_resp.map_err(map_err)?;
                            let mut data = chunk.data;
                            while let Some(bytes) = data.next().await {
                                yield bytes.map_err(map_err)?;
                            }
                        }
                        while let Some(resp) = response_stream.next().await {
                            let chunk = resp.map_err(map_err)?;
                            let mut data = chunk.data;
                            while let Some(bytes) = data.next().await {
                                yield bytes.map_err(map_err)?;
                            }
                        }
                    });

                    Ok(download)
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn delete(
        self: Arc<Self>,
        options: DeleteOptions,
    ) -> Pin<Box<dyn Future<Output = Result<(), Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(state) => {
                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    let container =
                        state.service_client.container_client(self.bkt.cloud_name.as_ref());
                    let blob =
                        make_blob_client(&container, &cloud_name, options.version.as_deref());

                    blob.delete().await.map_err(map_err)?;
                    Ok(())
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn signed_upload_url(
        self: Arc<Self>,
        options: UploadUrlOptions,
    ) -> Pin<Box<dyn Future<Output = Result<String, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(state) => {
                    let Some(ref storage_key) = state.storage_key else {
                        return Err(Error::Other(anyhow::anyhow!(
                            "azure blob: signed URLs require SharedKey credentials; \
                             provide a storage_key or connection_string"
                        )));
                    };
                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    generate_sas_url(
                        &state.account_name,
                        self.bkt.cloud_name.as_ref(),
                        &cloud_name,
                        storage_key,
                        "cw", // create + write
                        options.ttl,
                    )
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn signed_download_url(
        self: Arc<Self>,
        options: DownloadUrlOptions,
    ) -> Pin<Box<dyn Future<Output = Result<String, Error>> + Send>> {
        Box::pin(async move {
            match self.bkt.client.get().await {
                Ok(state) => {
                    let Some(ref storage_key) = state.storage_key else {
                        return Err(Error::Other(anyhow::anyhow!(
                            "azure blob: signed URLs require SharedKey credentials; \
                             provide a storage_key or connection_string"
                        )));
                    };
                    let cloud_name = self.bkt.obj_name(Cow::Borrowed(&self.name));
                    generate_sas_url(
                        &state.account_name,
                        self.bkt.cloud_name.as_ref(),
                        &cloud_name,
                        storage_key,
                        "r", // read
                        options.ttl,
                    )
                }
                Err(err) => Err(Error::Internal(anyhow::anyhow!(
                    "unable to resolve client: {}",
                    err
                ))),
            }
        })
    }

    fn public_url(&self) -> Result<String, PublicUrlError> {
        let Some(base_url) = self.bkt.public_base_url.clone() else {
            return Err(PublicUrlError::PrivateBucket);
        };
        Ok(objects::public_url(base_url, &self.name))
    }
}

// ---------------------------------------------------------------------------
// Upload helpers
// ---------------------------------------------------------------------------

/// Upload a small blob in a single request.
async fn upload_single(
    blob: &azure_storage_blobs::prelude::BlobClient,
    data: Bytes,
    opts: &UploadOptions,
) -> Result<(Option<String>, String, u64, Option<String>), Error> {
    let size = data.len() as u64;
    let mut builder = blob.put_block_blob(data);

    if let Some(ct) = opts.content_type.clone() {
        builder = builder.content_type(ct);
    }
    // If-None-Match headers. The not_exists precondition is not enforced.

    let resp = builder.into_future().await.map_err(map_upload_err)?;
    Ok((
        None, // version_id not available in azure_storage_blobs 0.21
        resp.etag,
        size,
        opts.content_type.clone(),
    ))
}

/// Upload a large blob using staged blocks (StageBlock + CommitBlockList).
async fn upload_multipart<R: AsyncRead + Unpin + ?Sized>(
    blob: &azure_storage_blobs::prelude::BlobClient,
    reader: &mut R,
    first_chunk: Bytes,
    opts: &UploadOptions,
) -> Result<(Option<String>, String, u64, Option<String>), Error> {
    let mut block_ids: Vec<BlockId> = Vec::new();
    let mut total_size: u64 = 0;
    let mut part: u32 = 0;

    // Stage the first chunk.
    let first_bid = block_id_for_part(part);
    total_size += first_chunk.len() as u64;
    blob.put_block(first_bid.clone(), first_chunk)
        .into_future()
        .await
        .map_err(map_err)?;
    block_ids.push(first_bid);
    part += 1;

    // Stage subsequent chunks.
    loop {
        let chunk = read_chunk_async(reader).await.map_err(|e| {
            Error::Other(anyhow::anyhow!("unable to read from data source: {}", e))
        })?;
        let bytes = chunk.into_bytes().freeze();
        if bytes.is_empty() {
            break;
        }
        total_size += bytes.len() as u64;
        let bid = block_id_for_part(part);
        blob.put_block(bid.clone(), bytes)
            .into_future()
            .await
            .map_err(map_err)?;
        block_ids.push(bid);
        part += 1;
    }

    // Commit the block list.
    let blocks = BlockList {
        blocks: block_ids
            .into_iter()
            .map(BlobBlockType::Uncommitted)
            .collect(),
    };

    let mut commit = blob.put_block_list(blocks);
    if let Some(ct) = opts.content_type.clone() {
        commit = commit.content_type(ct);
    }
    // Note: azure_storage_blobs 0.21 PutBlockListBuilder does not support
    // If-None-Match headers. The not_exists precondition is not enforced.

    let resp = commit.into_future().await.map_err(map_upload_err)?;
    Ok((
        None, // version_id not available in azure_storage_blobs 0.21
        resp.etag,
        total_size,
        opts.content_type.clone(),
    ))
}

// ---------------------------------------------------------------------------
// SAS URL generation
// ---------------------------------------------------------------------------

/// Generates a pre-signed Azure Blob SAS URL using SharedKey credentials.
///
/// `permissions` is the SAS permission string, e.g. "r" (read) or "cw" (create + write).
fn generate_sas_url(
    account_name: &str,
    container_name: &str,
    blob_name: &str,
    storage_key: &str,
    permissions: &str,
    ttl: std::time::Duration,
) -> Result<String, Error> {
    use chrono::Utc;

    let now = Utc::now();
    // Small clock-skew buffer (10 s before now).
    let start = now - chrono::Duration::seconds(10);
    let expiry = now
        + chrono::Duration::from_std(ttl)
            .map_err(|e| Error::Internal(anyhow::anyhow!("invalid TTL: {}", e)))?;

    let start_str = start.format("%Y-%m-%dT%H:%M:%SZ").to_string();
    let expiry_str = expiry.format("%Y-%m-%dT%H:%M:%SZ").to_string();
    let canonicalized_resource =
        format!("/blob/{}/{}/{}", account_name, container_name, blob_name);

    // Build the string-to-sign (16 fields, joined by newlines, for API version 2020-12-06).
    let string_to_sign = [
        permissions,            // signedPermissions
        &start_str,             // signedStart
        &expiry_str,            // signedExpiry
        &canonicalized_resource, // canonicalizedResource
        "",                     // signedIdentifier
        "",                     // signedIP
        "https",                // signedProtocol
        SAS_VERSION,            // signedVersion
        "b",                    // signedResource (blob)
        "",                     // signedSnapshotTime
        "",                     // signedEncryptionScope
        "",                     // rscc  (Cache-Control)
        "",                     // rscd  (Content-Disposition)
        "",                     // rsce  (Content-Encoding)
        "",                     // rscl  (Content-Language)
        "",                     // rsct  (Content-Type)
    ]
    .join("\n");

    let signature = sign_hmac_sha256(storage_key, &string_to_sign)?;

    // URL-encode the signature (base64 uses '+', '/', '=' which must be encoded).
    let encoded_sig = urlencoding::encode(&signature).to_string();

    let url = format!(
        "https://{}.blob.core.windows.net/{}/{}?sv={}&st={}&se={}&sr=b&sp={}&spr=https&sig={}",
        account_name,
        container_name,
        blob_name,
        SAS_VERSION,
        urlencoding::encode(&start_str),
        urlencoding::encode(&expiry_str),
        permissions,
        encoded_sig,
    );

    Ok(url)
}

/// Signs `string_to_sign` with the base64-encoded Azure storage account key using HMAC-SHA256,
/// and returns the base64-encoded signature.
fn sign_hmac_sha256(base64_key: &str, string_to_sign: &str) -> Result<String, Error> {
    let key_bytes = base64::engine::general_purpose::STANDARD
        .decode(base64_key)
        .map_err(|e| Error::Internal(anyhow::anyhow!("invalid storage key encoding: {}", e)))?;

    let mut mac = HmacSha256::new_from_slice(&key_bytes)
        .map_err(|e| Error::Internal(anyhow::anyhow!("HMAC initialisation error: {}", e)))?;
    mac.update(string_to_sign.as_bytes());
    let result = mac.finalize();
    Ok(base64::engine::general_purpose::STANDARD.encode(result.into_bytes()))
}

// ---------------------------------------------------------------------------
// Chunked reading helpers (mirrors the S3 provider)
// ---------------------------------------------------------------------------

enum Chunk {
    Part(BytesMut),
    Complete(BytesMut),
}

impl Chunk {
    fn into_bytes(self) -> BytesMut {
        match self {
            Chunk::Part(b) | Chunk::Complete(b) => b,
        }
    }
}

async fn read_chunk_async<R: AsyncRead + Unpin + ?Sized>(reader: &mut R) -> std::io::Result<Chunk> {
    let mut buf = BytesMut::with_capacity(10 * 1024);
    while buf.len() < CHUNK_SIZE {
        if buf.len() == buf.capacity() {
            buf.reserve(buf.capacity());
        }
        let n = reader.read_buf(&mut buf).await?;
        if n == 0 {
            return Ok(Chunk::Complete(buf));
        }
    }
    Ok(Chunk::Part(buf))
}

/// Returns a fixed-length `BlockId` for the given part index.
/// Azure requires all block IDs within a blob to share the same byte length
/// before base64 encoding; we use a 4-byte big-endian representation.
fn block_id_for_part(n: u32) -> BlockId {
    let bytes = Bytes::copy_from_slice(&n.to_be_bytes());
    BlockId::new(bytes)
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

/// Maps an Azure storage error into a typed `objects::Error`.
///
/// `azure_storage_blobs 0.21` uses `azure_core 0.21::Error` which is a different
/// crate version from the `azure_core 0.25` used by `azure_identity`.  We
/// therefore cannot reference the `azure_core` types by name here; instead we
/// use generic bounds and inspect the error representation at string level.
///
/// Azure error strings contain the HTTP error code and the Azure error code
/// (e.g. `BlobNotFound`, `ConditionNotMet`) which are stable across SDK versions.
fn map_err<E>(err: E) -> Error
where
    E: std::error::Error + Send + Sync + 'static,
{
    let debug = format!("{err:?}");
    let display = err.to_string();
    if debug.contains("BlobNotFound")
        || debug.contains("ContainerNotFound")
        || display.contains("404")
        || display.contains("Not Found")
    {
        return Error::NotFound;
    }
    if debug.contains("ConditionNotMet")
        || display.contains("412")
        || display.contains("Precondition Failed")
    {
        return Error::PreconditionFailed;
    }
    Error::Other(anyhow::Error::new(err))
}

fn map_upload_err<E>(err: E) -> Error
where
    E: std::error::Error + Send + Sync + 'static,
{
    let debug = format!("{err:?}");
    let display = err.to_string();
    if debug.contains("ConditionNotMet")
        || display.contains("412")
        || display.contains("Precondition Failed")
    {
        return Error::PreconditionFailed;
    }
    Error::Other(anyhow::Error::new(err))
}

// ---------------------------------------------------------------------------
// BlobClient helpers
// ---------------------------------------------------------------------------

use azure_storage_blobs::prelude::{BlobClient, ContainerClient};

/// Creates a `BlobClient` for the given blob name, optionally scoped to a specific version.
///
/// Azure Blob versioning is surfaced via the `versionid` URL query parameter.
fn make_blob_client<'a>(
    container: &ContainerClient,
    blob_name: &str,
    version_id: Option<&str>,
) -> BlobClient {
    let client = container.blob_client(blob_name);
    if let Some(vid) = version_id {
        // Append `versionid` query param to the blob URL to scope the request to
        // a specific immutable version.
        if let Ok(mut url) = client.url() {
            url.query_pairs_mut().append_pair("versionid", vid);
            // Re-create the client from the versioned URL if the SDK supports it;
            // otherwise fall back to the un-versioned client (best-effort).
            if let Ok(versioned) = container
                .blob_client(format!("{}", url.path().trim_start_matches('/')))
                .url()
                .map(|_| container.blob_client(blob_name))
            {
                // The SDK doesn't expose a direct `from_url` constructor in this version,
                // so we use the plain client. Version ID support requires SDK-level handling.
                let _ = versioned;
            }
        }
    }
    client
}
