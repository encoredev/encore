package objects

import (
	"context"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/noop"
	"encore.dev/storage/objects/internal/types"
)

// Bucket represents an object storage bucket, containing a set of files.
//
// See NewBucket for more information on how to declare a Bucket.
type Bucket struct {
	mgr        *Manager
	staticCfg  BucketConfig   // The config as defined in the applications source code
	runtimeCfg *config.Bucket // The config for this running instance of the application
	impl       types.BucketImpl
}

func newBucket(mgr *Manager, name string, cfg BucketConfig) *Bucket {
	// if mgr.static.Testing {
	// 	return &Bucket{
	// 		staticCfg:  cfg,
	// 		mgr:        mgr,
	// 		runtimeCfg: &config.Bucket{EncoreName: name},
	// 		impl:       test.NewBucket(mgr.ts, name),
	// 	}
	// }

	// Look up the bkt configuration
	bkt, ok := mgr.runtime.Buckets[name]
	if !ok {
		// If we don't have a topic configuration for this topic, it means that the topic was not registered for this instance
		// thus we should default to the noop implementation.
		return &Bucket{
			staticCfg:  cfg,
			mgr:        mgr,
			runtimeCfg: &config.Bucket{EncoreName: name},
			impl:       &noop.BucketImpl{},
		}
	}

	// Look up the provider config
	provider := mgr.runtime.BucketProviders[bkt.ProviderID]

	tried := make([]string, 0, len(mgr.providers))
	for _, p := range mgr.providers {
		if p.Matches(provider) {
			impl := p.NewBucket(provider, cfg, bkt)
			return &Bucket{
				staticCfg:  cfg,
				mgr:        mgr,
				runtimeCfg: bkt,
				impl:       impl,
			}
		}
		tried = append(tried, p.ProviderName())
	}

	mgr.rootLogger.Fatal().Msgf("unsupported Object Storage provider for provider[%d], tried: %v",
		bkt.ProviderID, tried)
	panic("unreachable")
}

// BucketMeta contains metadata about a bucket.
// The fields should not be modified by the caller.
// Additional fields may be added in the future.
type BucketMeta struct {
	// Name is the name of the bucket, as provided in the constructor to NewTopic.
	Name string
	// Config is the bucket's configuration.
	Config BucketConfig
}

// Meta returns metadata about the topic.
func (b *Bucket) Meta() BucketMeta {
	return BucketMeta{
		Name:   b.runtimeCfg.EncoreName,
		Config: b.staticCfg,
	}
}

func (b *Bucket) Upload(ctx context.Context, object string) *Writer {
	return &Writer{
		bkt: b,

		ctx: ctx,
		obj: object,
	}
}

type Writer struct {
	bkt *Bucket

	ctx context.Context
	obj string

	// Initialized on first write
	u types.Uploader
}

func (w *Writer) Write(p []byte) (int, error) {
	u := w.initUpload()
	return u.Write(p)
}

func (w *Writer) Close() error {
	u := w.initUpload()
	_, err := u.Complete()
	return err
}

func (w *Writer) initUpload() types.Uploader {
	if w.u == nil {
		w.u = w.bkt.impl.NewUpload(types.UploadData{
			Ctx:    w.ctx,
			Object: w.obj,
		})
	}
	return w.u
}
