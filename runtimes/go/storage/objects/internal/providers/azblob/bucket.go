//go:build !encore_no_azure

package azblob

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/types"
)

// Manager manages Azure Blob Storage bucket clients.
//
// NOTE: Azure Blob Storage proto/config support (AzureBlobBucketProvider) was
// added to config.go but does not yet exist in infra.proto. When the proto
// definition is added, the config parsing layer will need to be updated to
// populate AzureBlobBucketProvider from the proto message.
type Manager struct {
	ctx     context.Context
	runtime *config.Runtime
	clients map[*config.BucketProvider]*clientState
}

type clientState struct {
	serviceClient *azblob.Client
	sharedKey     *azblob.SharedKeyCredential // nil when using managed identity
	accountName   string
}

func NewManager(ctx context.Context, runtime *config.Runtime) *Manager {
	return &Manager{
		ctx:     ctx,
		runtime: runtime,
		clients: make(map[*config.BucketProvider]*clientState),
	}
}

type bucket struct {
	containerClient *container.Client
	sharedKey       *azblob.SharedKeyCredential // nil when using managed identity
	accountName     string
	cfg             *config.Bucket
}

func (mgr *Manager) ProviderName() string { return "azure-blob" }

func (mgr *Manager) Matches(cfg *config.BucketProvider) bool {
	return cfg.AzureBlob != nil
}

func (mgr *Manager) NewBucket(provider *config.BucketProvider, runtimeCfg *config.Bucket) types.BucketImpl {
	state := mgr.clientForProvider(provider)
	containerClient := state.serviceClient.ServiceClient().NewContainerClient(runtimeCfg.CloudName)
	return &bucket{
		containerClient: containerClient,
		sharedKey:       state.sharedKey,
		accountName:     state.accountName,
		cfg:             runtimeCfg,
	}
}

func (b *bucket) Download(data types.DownloadData) (types.Downloader, error) {
	blobClient := b.containerClient.NewBlockBlobClient(data.Object.String())
	if data.Version != "" {
		var err error
		blobClient, err = blobClient.WithVersionID(data.Version)
		if err != nil {
			return nil, err
		}
	}
	resp, err := blobClient.DownloadStream(data.Ctx, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Body, nil
}

func (b *bucket) Upload(data types.UploadData) (types.Uploader, error) {
	blobClient := b.containerClient.NewBlockBlobClient(data.Object.String())
	return newUploader(blobClient, data), nil
}

func (b *bucket) List(data types.ListData) iter.Seq2[*types.ListEntry, error] {
	return func(yield func(*types.ListEntry, error) bool) {
		var n int64
		pager := b.containerClient.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
			Prefix: ptrOrNil(data.Prefix),
		})
		for pager.More() {
			if err := data.Ctx.Err(); err != nil {
				yield(nil, err)
				return
			}
			resp, err := pager.NextPage(data.Ctx)
			if err != nil {
				yield(nil, mapErr(err))
				return
			}
			for _, item := range resp.Segment.BlobItems {
				if data.Limit != nil && n >= *data.Limit {
					return
				}
				n++
				entry := &types.ListEntry{
					Object: types.CloudObject(valOrZero(item.Name)),
					Size:   valOrZero(item.Properties.ContentLength),
					ETag:   string(valOrZero(item.Properties.ETag)),
				}
				if !yield(entry, nil) {
					return
				}
			}
		}
	}
}

func (b *bucket) Remove(data types.RemoveData) error {
	blobClient := b.containerClient.NewBlockBlobClient(data.Object.String())
	if data.Version != "" {
		var err error
		blobClient, err = blobClient.WithVersionID(data.Version)
		if err != nil {
			return err
		}
	}
	_, err := blobClient.Delete(data.Ctx, nil)
	return mapErr(err)
}

func (b *bucket) Attrs(data types.AttrsData) (*types.ObjectAttrs, error) {
	blobClient := b.containerClient.NewBlockBlobClient(data.Object.String())
	if data.Version != "" {
		var err error
		blobClient, err = blobClient.WithVersionID(data.Version)
		if err != nil {
			return nil, err
		}
	}
	resp, err := blobClient.GetProperties(data.Ctx, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return &types.ObjectAttrs{
		Object:      data.Object,
		Version:     valOrZero(resp.VersionID),
		ContentType: valOrZero(resp.ContentType),
		Size:        valOrZero(resp.ContentLength),
		ETag:        string(valOrZero(resp.ETag)),
	}, nil
}

func (b *bucket) SignedUploadURL(data types.UploadURLData) (string, error) {
	if b.sharedKey == nil {
		return "", fmt.Errorf("azure blob: signed URLs require SharedKey credentials; provide a storage_key or connection_string")
	}
	blobName := data.Object.String()
	perms := sas.BlobPermissions{Write: true, Create: true}
	sasParams, err := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     time.Now().UTC().Add(-10 * time.Second), // small buffer for clock skew
		ExpiryTime:    time.Now().UTC().Add(data.TTL),
		Permissions:   perms.String(),
		ContainerName: b.cfg.CloudName,
		BlobName:      blobName,
	}.SignWithSharedKey(b.sharedKey)
	if err != nil {
		return "", mapErr(err)
	}
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?%s",
		b.accountName, b.cfg.CloudName, blobName, sasParams.Encode()), nil
}

func (b *bucket) SignedDownloadURL(data types.DownloadURLData) (string, error) {
	if b.sharedKey == nil {
		return "", fmt.Errorf("azure blob: signed URLs require SharedKey credentials; provide a storage_key or connection_string")
	}
	blobName := data.Object.String()
	perms := sas.BlobPermissions{Read: true}
	sasParams, err := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     time.Now().UTC().Add(-10 * time.Second), // small buffer for clock skew
		ExpiryTime:    time.Now().UTC().Add(data.TTL),
		Permissions:   perms.String(),
		ContainerName: b.cfg.CloudName,
		BlobName:      blobName,
	}.SignWithSharedKey(b.sharedKey)
	if err != nil {
		return "", mapErr(err)
	}
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?%s",
		b.accountName, b.cfg.CloudName, blobName, sasParams.Encode()), nil
}

func (mgr *Manager) clientForProvider(prov *config.BucketProvider) *clientState {
	if state, ok := mgr.clients[prov]; ok {
		return state
	}

	cfg := prov.AzureBlob
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.StorageAccount)

	var (
		client    *azblob.Client
		sharedKey *azblob.SharedKeyCredential
		err       error
	)

	switch {
	case cfg.ConnectionString != nil:
		// Connection string auth: create the service client from the connection string.
		client, err = azblob.NewClientFromConnectionString(*cfg.ConnectionString, nil)
		if err != nil {
			panic(fmt.Sprintf("azure blob: failed to create client from connection string: %v", err))
		}
		// Try to extract AccountName + AccountKey from the connection string so we
		// can generate SAS URLs. Connection strings look like:
		// DefaultEndpointsProtocol=https;AccountName=xxx;AccountKey=yyy==;EndpointSuffix=...
		if accountName, accountKey := parseConnectionString(*cfg.ConnectionString); accountName != "" && accountKey != "" {
			sharedKey, err = azblob.NewSharedKeyCredential(accountName, accountKey)
			if err != nil {
				panic(fmt.Sprintf("azure blob: failed to create shared key credential from connection string: %v", err))
			}
			cfg.StorageAccount = accountName // ensure accountName is set for SAS URL generation
		}

	case cfg.StorageKey != nil:
		// Explicit SharedKey authentication.
		sharedKey, err = azblob.NewSharedKeyCredential(cfg.StorageAccount, *cfg.StorageKey)
		if err != nil {
			panic(fmt.Sprintf("azure blob: failed to create shared key credential: %v", err))
		}
		client, err = azblob.NewClientWithSharedKeyCredential(serviceURL, sharedKey, nil)
		if err != nil {
			panic(fmt.Sprintf("azure blob: failed to create Azure Blob client with shared key: %v", err))
		}

	default:
		// No explicit credentials: use DefaultAzureCredential (managed identity, env vars, etc.).
		cred, credErr := azidentity.NewDefaultAzureCredential(nil)
		if credErr != nil {
			panic(fmt.Sprintf("azure blob: failed to create default Azure credential: %v", credErr))
		}
		client, err = azblob.NewClient(serviceURL, cred, nil)
		if err != nil {
			panic(fmt.Sprintf("azure blob: failed to create Azure Blob client: %v", err))
		}
	}

	state := &clientState{
		serviceClient: client,
		sharedKey:     sharedKey,
		accountName:   cfg.StorageAccount,
	}
	mgr.clients[prov] = state
	return state
}

// parseConnectionString extracts the AccountName and AccountKey from an Azure
// Blob Storage connection string of the form:
//
//	DefaultEndpointsProtocol=https;AccountName=<name>;AccountKey=<key>;...
func parseConnectionString(connStr string) (accountName, accountKey string) {
	for _, segment := range strings.Split(connStr, ";") {
		kv := strings.SplitN(segment, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "AccountName":
			accountName = kv[1]
		case "AccountKey":
			accountKey = kv[1]
		}
	}
	return
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if bloberror.HasCode(err, bloberror.BlobNotFound, bloberror.ContainerNotFound) {
		return types.ErrObjectNotExist
	}
	if bloberror.HasCode(err, bloberror.ConditionNotMet) {
		return types.ErrPreconditionFailed
	}
	return err
}

func ptrOrNil[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

func valOrZero[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

func ptr[T any](v T) *T { return &v }
