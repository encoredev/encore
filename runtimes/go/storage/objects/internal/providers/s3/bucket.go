package s3

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/types"
)

type Manager struct {
	ctx     context.Context
	runtime *config.Runtime
	clients map[*config.BucketProvider]*s3.Client

	cfgOnce sync.Once
	awsCfg  aws.Config
}

func NewManager(ctx context.Context, runtime *config.Runtime) *Manager {
	return &Manager{ctx: ctx, runtime: runtime, clients: make(map[*config.BucketProvider]*s3.Client)}
}

type bucket struct {
	client *s3.Client
	cfg    *config.Bucket
}

func (mgr *Manager) ProviderName() string { return "s3" }

func (mgr *Manager) Matches(cfg *config.BucketProvider) bool {
	return cfg.S3 != nil
}

func (mgr *Manager) NewBucket(provider *config.BucketProvider, runtimeCfg *config.Bucket) types.BucketImpl {
	client := mgr.clientForProvider(provider)
	return &bucket{
		client: client,
		cfg:    runtimeCfg,
	}
}

func (b *bucket) Download(data types.DownloadData) (types.Downloader, error) {
	object := string(data.Object)
	resp, err := b.client.GetObject(data.Ctx, &s3.GetObjectInput{
		Bucket:    &b.cfg.CloudName,
		Key:       &object,
		VersionId: ptrOrNil(data.Version),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Body, nil
}

func (b *bucket) Upload(data types.UploadData) (types.Uploader, error) {
	return newUploader(b.client, b.cfg.CloudName, data), nil
}

func mapListEntry(attrs *storage.ObjectAttrs) *types.ListEntry {
	return &types.ListEntry{
		Object: types.CloudObject(attrs.Name),
		Size:   attrs.Size,
		ETag:   attrs.Etag,
	}
}

func (b *bucket) List(data types.ListData) iter.Seq2[*types.ListEntry, error] {
	return func(yield func(*types.ListEntry, error) bool) {
		var n int64
		var continuationToken string
		for data.Limit == nil || n < *data.Limit {
			// Abort early if the context is canceled.
			if err := data.Ctx.Err(); err != nil {
				yield(nil, err)
				return
			}

			maxKeys := int32(1000)
			if data.Limit != nil {
				maxKeys = min(int32(*data.Limit-n), 1000)
			}
			resp, err := b.client.ListObjectsV2(data.Ctx, &s3.ListObjectsV2Input{
				Bucket:            &b.cfg.CloudName,
				MaxKeys:           &maxKeys,
				ContinuationToken: ptrOrNil(continuationToken),
				Prefix:            ptrOrNil(data.Prefix),
			})
			if err != nil {
				yield(nil, mapErr(err))
				return
			}

			for _, obj := range resp.Contents {
				if !yield(&types.ListEntry{
					Object: types.CloudObject(*obj.Key),
					Size:   *obj.Size,
					ETag:   *obj.ETag,
				}, nil) {
					return
				}
				n++
			}

			// Are we done?
			if !valOrZero(resp.IsTruncated) {
				return
			}
			continuationToken = valOrZero(resp.NextContinuationToken)
		}
	}
}

func (b *bucket) Remove(data types.RemoveData) error {
	object := string(data.Object)
	_, err := b.client.DeleteObject(data.Ctx, &s3.DeleteObjectInput{
		Bucket:    &b.cfg.CloudName,
		Key:       &object,
		VersionId: ptrOrNil(data.Version),
	})
	return mapErr(err)
}

func (b *bucket) Attrs(data types.AttrsData) (*types.ObjectAttrs, error) {
	object := string(data.Object)
	resp, err := b.client.HeadObject(data.Ctx, &s3.HeadObjectInput{
		Bucket:    &b.cfg.CloudName,
		Key:       &object,
		VersionId: ptrOrNil(data.Version),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return &types.ObjectAttrs{
		Object:      data.Object,
		Version:     valOrZero(resp.VersionId),
		ContentType: valOrZero(resp.ContentType),
		Size:        valOrZero(resp.ContentLength),
		ETag:        valOrZero(resp.ETag),
	}, nil
}

func (mgr *Manager) clientForProvider(prov *config.BucketProvider) *s3.Client {
	if client, ok := mgr.clients[prov]; ok {
		return client
	}

	cfg := mgr.getConfig()
	client := s3.New(s3.Options{
		Region:       prov.S3.Region,
		BaseEndpoint: prov.S3.Endpoint,
		Credentials:  cfg.Credentials,
	})

	mgr.clients[prov] = client
	return client
}

// getConfig loads the required AWS config to connect to AWS
func (mgr *Manager) getConfig() aws.Config {
	mgr.cfgOnce.Do(func() {
		cfg, err := awsConfig.LoadDefaultConfig(context.Background())
		if err != nil {
			panic(fmt.Sprintf("unable to load AWS config: %v", err))
		}
		mgr.awsCfg = cfg

	})
	return mgr.awsCfg
}

func mapErr(err error) error {
	var (
		noSuchKey *s3types.NoSuchKey
		generic   smithy.APIError
	)
	switch {
	case err == nil:
		return nil
	case errors.As(err, &noSuchKey):
		return types.ErrObjectNotExist
	case errors.As(err, &generic):
		if generic.ErrorCode() == "PreconditionFailed" {
			return types.ErrPreconditionFailed
		}
		return err
	default:
		return err

	}
}

func ptrOrNil[T comparable](val T) *T {
	var zero T
	if val != zero {
		return &val
	}
	return nil
}

func valOrZero[T comparable](val *T) T {
	if val != nil {
		return *val
	}
	var zero T
	return zero
}

func ptr[T any](val T) *T {
	return &val
}
