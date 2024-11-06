package gcp

import (
	"context"
	"fmt"
	"strconv"

	"cloud.google.com/go/storage"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/types"
)

type Manager struct {
	ctx     context.Context
	runtime *config.Runtime
	client  *storage.Client
}

func NewManager(ctx context.Context, static *config.Static, runtime *config.Runtime) *Manager {
	client, err := storage.NewClient(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to create object storage client: %s", err))
	}
	return &Manager{ctx: ctx, runtime: runtime, client: client}
}

type bucket struct {
	mgr    *Manager
	cfg    *config.Bucket
	handle *storage.BucketHandle
}

func (mgr *Manager) ProviderName() string { return "gcp" }

func (mgr *Manager) Matches(cfg *config.BucketProvider) bool {
	return cfg.GCS != nil
}

func (mgr *Manager) NewBucket(provider *config.BucketProvider, staticCfg types.BucketConfig, runtimeCfg *config.Bucket) types.BucketImpl {
	handle := mgr.client.Bucket(runtimeCfg.CloudName)
	return &bucket{mgr, runtimeCfg, handle}
}

func (b *bucket) NewUpload(data types.UploadData) types.Uploader {
	ctx, cancel := context.WithCancelCause(data.Ctx)
	w := b.handle.Object(data.Object).NewWriter(ctx)
	// TODO set ChunkSize, attributes, etc

	u := &uploader{
		cancel: cancel,
		w:      w,
	}
	return u
}

type uploader struct {
	cancel context.CancelCauseFunc
	w      *storage.Writer
}

func (u *uploader) Write(p []byte) (int, error) {
	return u.w.Write(p)
}

func (u *uploader) Complete() (*types.Attrs, error) {
	if err := u.w.Close(); err != nil {
		return nil, err
	}

	attrs := u.w.Attrs()
	return &types.Attrs{
		Version: strconv.FormatInt(attrs.Generation, 10),
		// TODO fill this in
	}, nil
}

func (u *uploader) Abort(err error) {
	u.cancel(err)
}
