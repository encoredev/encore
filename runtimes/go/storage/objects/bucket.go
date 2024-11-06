package objects

import (
	"context"
	"errors"
	"iter"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/providers/noop"
	"encore.dev/storage/objects/internal/types"
)

// Bucket represents an object storage bucket, containing a set of files.
//
// See NewBucket for more information on how to declare a Bucket.
type Bucket struct {
	mgr        *Manager
	runtimeCfg *config.Bucket // The config for this running instance of the application
	impl       types.BucketImpl
}

type BucketConfig struct {
	// Whether objects stored in the bucket should be versioned.
	//
	// If true, the bucket will store multiple versions of each object
	// whenever it changes, as opposed to overwriting the old version.
	Versioned bool
}

func newBucket(mgr *Manager, name string) *Bucket {
	// Look up the bkt configuration
	bkt, ok := mgr.runtime.Buckets[name]
	if !ok {
		// No runtime config; return the noop implementation.
		return &Bucket{
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
			impl := p.NewBucket(provider, bkt)
			return &Bucket{
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

func (b *Bucket) Upload(ctx context.Context, object string, options ...UploadOption) *Writer {
	var opt uploadOptions
	for _, o := range options {
		o.uploadOption(&opt)
	}

	return &Writer{
		bkt: b,
		ctx: ctx,
		obj: object,
		opt: opt,
	}
}

type Writer struct {
	bkt *Bucket

	ctx context.Context
	obj string

	opt uploadOptions

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
		u, err := w.bkt.impl.Upload(types.UploadData{
			Ctx:    w.ctx,
			Object: w.obj,
			Attrs:  w.opt.attrs,
		})
		if err != nil {
			w.u = &errUploader{err: err}
		} else {
			w.u = u
		}
	}

	return w.u
}

type errUploader struct {
	err error
}

func (e *errUploader) Write(p []byte) (int, error) {
	return 0, e.err
}
func (e *errUploader) Abort(err error) {}
func (e *errUploader) Complete() (*types.ObjectAttrs, error) {
	return nil, e.err
}

var _ types.Uploader = &errUploader{}

func (b *Bucket) Download(ctx context.Context, object string, options ...DownloadOption) *Reader {
	var opt downloadOptions
	for _, o := range options {
		o.downloadOption(&opt)
	}

	r, err := b.impl.Download(types.DownloadData{
		Ctx:     ctx,
		Object:  object,
		Version: opt.version,
	})
	return &Reader{r: r, err: err}
}

type Reader struct {
	err error // any error encountered
	r   types.Downloader
}

func (r *Reader) Err() error {
	return r.err
}

func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	n, err := r.r.Read(p)
	r.err = err
	return n, err
}

func (r *Reader) Close() error {
	if r.err != nil {
		return r.err
	}

	r.err = r.r.Close()
	return r.err
}

type Query struct {
	// Prefix indicates to only return objects
	// whose name starts with the given prefix.
	Prefix string

	// Maximum number of objects to return. Zero means no limit.
	Limit int64
}

func mapQuery(ctx context.Context, q *Query) types.ListData {
	return types.ListData{
		Ctx:    ctx,
		Prefix: q.Prefix,
		Limit:  q.Limit,
	}
}

type ObjectAttrs struct {
	Version string
}

func mapAttrs(attrs *types.ObjectAttrs) *ObjectAttrs {
	return &ObjectAttrs{
		Version: attrs.Version,
	}
}

type ListEntry struct {
	Name string
	Size int64
	ETag string
}

func mapListEntry(entry *types.ListEntry) *ListEntry {
	return &ListEntry{
		Name: entry.Name,
		Size: entry.Size,
		ETag: entry.ETag,
	}
}

func (b *Bucket) List(ctx context.Context, query *Query, options ...ListOption) iter.Seq2[*ListEntry, error] {
	return func(yield func(*ListEntry, error) bool) {
		iter := b.impl.List(mapQuery(ctx, query))
		for entry, err := range iter {
			if err != nil {
				if !yield(nil, err) {
					return
				}
			}
			if !yield(mapListEntry(entry), nil) {
				return
			}
		}
	}
}

// Remove removes an object from the bucket.
func (b *Bucket) Remove(ctx context.Context, object string, options ...RemoveOption) error {
	return b.impl.Remove(types.RemoveData{
		Ctx:    ctx,
		Object: object,
	})
}

var ErrObjectNotFound = types.ErrObjectNotExist

// Attrs returns the attributes of an object in the bucket.
// If the object does not exist, it returns ErrObjectNotFound.
func (b *Bucket) Attrs(ctx context.Context, object string, options ...AttrsOption) (*ObjectAttrs, error) {
	var opt attrsOptions
	for _, o := range options {
		o.attrsOption(&opt)
	}

	attrs, err := b.impl.Attrs(types.AttrsData{
		Ctx:     ctx,
		Object:  object,
		Version: opt.version,
	})
	if err != nil {
		return nil, err
	}

	return mapAttrs(attrs), nil
}

// Exists reports whether an object exists in the bucket.
func (b *Bucket) Exists(ctx context.Context, object string, options ...ExistsOption) (bool, error) {
	var opt existsOptions
	for _, o := range options {
		o.existsOption(&opt)
	}

	_, err := b.impl.Attrs(types.AttrsData{
		Ctx:     ctx,
		Object:  object,
		Version: opt.version,
	})
	if errors.Is(err, ErrObjectNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
