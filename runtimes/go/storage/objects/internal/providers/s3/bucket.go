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
	awsCreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/types"
)

type Manager struct {
	ctx     context.Context
	runtime *config.Runtime
	clients map[*config.BucketProvider]*clientSet

	cfgOnce          sync.Once
	awsDefaultConfig aws.Config
}

func NewManager(ctx context.Context, runtime *config.Runtime) *Manager {
	return &Manager{ctx: ctx, runtime: runtime, clients: make(map[*config.BucketProvider]*clientSet)}
}

type bucket struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	cfg           *config.Bucket
}

type clientSet struct {
	client        *s3.Client
	presignClient *s3.PresignClient
}

func (mgr *Manager) ProviderName() string { return "s3" }

func (mgr *Manager) Matches(cfg *config.BucketProvider) bool {
	return cfg.S3 != nil
}

func (mgr *Manager) NewBucket(provider *config.BucketProvider, runtimeCfg *config.Bucket) types.BucketImpl {
	clients := mgr.clientForProvider(provider)
	return &bucket{
		client:        clients.client,
		presignClient: clients.presignClient,
		cfg:           runtimeCfg,
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

func (b *bucket) SignedUploadURL(data types.UploadURLData) (string, error) {
	object := string(data.Object)
	params := s3.PutObjectInput{
		Bucket: &b.cfg.CloudName,
		Key:    &object,
	}
	sign_opts := func(opts *s3.PresignOptions) {
		opts.Expires = data.Ttl
	}
	req, err := b.presignClient.PresignPutObject(data.Ctx, &params, sign_opts)

	url := ""
	if req != nil {
		url = req.URL
		// TODO: add check/warn against unexpected method and headers
		// (we expect PUT and host:<> but nothing else.)
	}
	return url, mapErr(err)
}

func (mgr *Manager) clientForProvider(prov *config.BucketProvider) *clientSet {
	if cs, ok := mgr.clients[prov]; ok {
		return cs
	}

	// If we have a custom access key and secret, use them instead of the default config.
	var cfg aws.Config
	if prov.S3.AccessKeyID != nil && prov.S3.SecretAccessKey != nil {
		var err error
		cfg, err = awsConfig.LoadDefaultConfig(context.Background(),
			awsConfig.WithCredentialsProvider(awsCreds.NewStaticCredentialsProvider(*prov.S3.AccessKeyID, *prov.S3.SecretAccessKey, "")),
		)
		if err != nil {
			panic(fmt.Sprintf("unable to load AWS config: %v", err))
		}
	} else {
		cfg = mgr.defaultConfig()
	}

	client := s3.New(s3.Options{
		Region:       prov.S3.Region,
		BaseEndpoint: prov.S3.Endpoint,
		Credentials:  cfg.Credentials,
	})

	clients := clientSet{
		client:        client,
		presignClient: s3.NewPresignClient(client),
	}

	mgr.clients[prov] = &clients
	return &clients
}

// defaultConfig loads the required AWS config to connect to AWS
func (mgr *Manager) defaultConfig() aws.Config {
	mgr.cfgOnce.Do(func() {
		cfg, err := awsConfig.LoadDefaultConfig(context.Background())
		if err != nil {
			panic(fmt.Sprintf("unable to load AWS config: %v", err))
		}
		mgr.awsDefaultConfig = cfg
	})
	return mgr.awsDefaultConfig
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
