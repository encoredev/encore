package objects

import (
	"context"
	"errors"
	"iter"
	"net/url"
	"strings"
	"time"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/exported/trace2"
	"encore.dev/appruntime/shared/reqtrack"
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
	name       string

	// Prefix to prepend to all cloud names.
	baseCloudPrefix string

	// publicBaseURL, if the bucket is public
	publicBaseURL *url.URL
}

// BucketConfig is the configuration for a Bucket.
type BucketConfig struct {
	// Whether objects in the bucket should be publicly accessible, via CDN.
	Public bool

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
			name:       name,
		}
	}

	// Look up the provider config
	provider := mgr.runtime.BucketProviders[bkt.ProviderID]

	tried := make([]string, 0, len(mgr.providers))
	for _, p := range mgr.providers {
		if p.Matches(provider) {
			impl := p.NewBucket(provider, bkt)

			var publicBaseURL *url.URL
			if bkt.PublicBaseURL != "" {
				var err error
				publicBaseURL, err = url.Parse(bkt.PublicBaseURL)
				if err != nil {
					mgr.rootLogger.Fatal().Msgf("invalid public base url for bucket %s: %v", name, err)
				}
			}

			return &Bucket{
				mgr:             mgr,
				runtimeCfg:      bkt,
				impl:            impl,
				name:            name,
				baseCloudPrefix: bkt.KeyPrefix,
				publicBaseURL:   publicBaseURL,
			}
		}

		tried = append(tried, p.ProviderName())
	}

	mgr.rootLogger.Fatal().Msgf("unsupported Object Storage provider for provider[%d], tried: %v",
		bkt.ProviderID, tried)
	panic("unreachable")
}

// Upload uploads a new object to the bucket.
//
// The returned writer must be successfully closed for the upload to complete.
// To abort the upload, call (*Writer).Abort or cancel the provided context.
func (b *Bucket) Upload(ctx context.Context, object string, options ...UploadOption) *Writer {
	var opt uploadOptions
	for _, o := range options {
		o.applyUpload(&opt)
	}

	w := &Writer{
		bkt: b,
		ctx: ctx,
		obj: object,
		opt: opt,
	}

	curr := b.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		w.curr = curr
		w.startEventID = curr.Trace.BucketObjectUploadStart(trace2.BucketObjectUploadStartParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Bucket: b.name,
			Object: object,
			Attrs: trace2.BucketObjectAttributes{
				ContentType: ptrOrNil(opt.attrs.ContentType),
			},
			Stack: stack.Build(1),
		})
	}

	return w
}

// PublicURL returns the public URL for accessing an object in the bucket.
func (b *Bucket) PublicURL(object string, options ...PublicURLOption) *url.URL {
	if b.publicBaseURL == nil {
		// This should never happen, since access to this method is controlled
		// by static analysis.
		panic("internal encore error: missing publicBaseURL for bucket " + b.name)
	}

	// Clone the base url
	u := *b.publicBaseURL

	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	u.Path += escape(object, encodePath)

	return &u
}

// Writer is the writer for an object being uploaded to a bucket.
type Writer struct {
	bkt *Bucket

	ctx context.Context
	obj string

	opt uploadOptions

	// Initialized on first write
	u types.Uploader

	// Set if tracing
	curr         reqtrack.Current
	startEventID trace2.EventID
}

// Write writes data to the object being uploaded.
func (w *Writer) Write(p []byte) (int, error) {
	u := w.initUpload()
	return u.Write(p)
}

// Abort aborts the upload.
func (w *Writer) Abort(err error) {
	if err == nil {
		err = errors.New("upload aborted")
	}
	u := w.initUpload()
	u.Abort(err)
}

// Close closes the upload, completing the upload if no errors occurred.
func (w *Writer) Close() error {
	u := w.initUpload()
	attrs, err := u.Complete()

	if w.curr.Trace != nil {
		params := trace2.BucketObjectUploadEndParams{
			StartID: w.startEventID,
			EventParams: trace2.EventParams{
				TraceID: w.curr.Req.TraceID,
				SpanID:  w.curr.Req.SpanID,
				Goid:    w.curr.Goctr,
			},
			Err: err,
		}

		if attrs != nil {
			params.Size = uint64(attrs.Size)
			params.Version = ptrOrNil(attrs.Version)
		}
		w.curr.Trace.BucketObjectUploadEnd(params)
	}

	return err
}

func (w *Writer) initUpload() types.Uploader {
	if w.u == nil {
		u, err := w.bkt.impl.Upload(types.UploadData{
			Ctx:    w.ctx,
			Object: w.bkt.toCloudObject(w.obj),
			Attrs:  w.opt.attrs,
			Pre: types.Preconditions{
				NotExists: w.opt.pre.NotExists,
			},
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

// Download downloads an object from the bucket.
// Any error is encountered is reported by the methods on *Reader.
// To check if the operation failed, call (*Reader).Err.
//
// If the object does not exist, the error may be checked with errors.Is(err, ErrObjectNotFound).
func (b *Bucket) Download(ctx context.Context, object string, options ...DownloadOption) *Reader {
	var opt downloadOptions
	for _, o := range options {
		o.applyDownload(&opt)
	}

	var startEventID trace2.EventID
	curr := b.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		startEventID = curr.Trace.BucketObjectDownloadStart(trace2.BucketObjectDownloadStartParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Bucket:  b.name,
			Object:  object,
			Version: ptrOrNil(opt.version),
			Stack:   stack.Build(1),
		})
	}

	r, err := b.impl.Download(types.DownloadData{
		Ctx:     ctx,
		Object:  b.toCloudObject(object),
		Version: opt.version,
	})
	return &Reader{r: r, err: err, curr: curr, startEventID: startEventID}
}

// Reader is the reader for an object being downloaded from a bucket.
type Reader struct {
	err       error // any error encountered
	r         types.Downloader
	totalRead uint64

	// Set if traced
	traceCompleted bool
	curr           reqtrack.Current
	startEventID   trace2.EventID
}

// Err returns the error encountered during reading, if any.
func (r *Reader) Err() error {
	return r.err
}

// Read reads data from the object being downloaded.
func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	n, err := r.r.Read(p)
	r.err = err
	r.totalRead += uint64(n)
	return n, err
}

// Close closes the reader.
// It must be called to release resources.
func (r *Reader) Close() error {
	defer r.completeTrace()
	if r.err != nil {
		return r.err
	}

	r.err = r.r.Close()
	return r.err
}

func (r *Reader) completeTrace() {
	if r.traceCompleted {
		return
	}

	r.traceCompleted = true
	if r.curr.Trace != nil && r.startEventID != 0 {
		r.curr.Trace.BucketObjectDownloadEnd(trace2.BucketObjectDownloadEndParams{
			StartID: r.startEventID,
			EventParams: trace2.EventParams{
				TraceID: r.curr.Req.TraceID,
				SpanID:  r.curr.Req.SpanID,
				Goid:    r.curr.Goctr,
			},
			Err:  r.err,
			Size: r.totalRead,
		})
	}
}

// Query describes the set of objects to query for using List.
type Query struct {
	// Prefix indicates to only return objects
	// whose name starts with the given prefix.
	Prefix string

	// Maximum number of objects to return. Zero means no limit.
	Limit int64
}

func (b *Bucket) mapQuery(ctx context.Context, q *Query) types.ListData {
	return types.ListData{
		Ctx:    ctx,
		Prefix: b.baseCloudPrefix + q.Prefix,
		Limit:  ptrOrNil(q.Limit),
	}
}

// ObjectAttrs describes the attributes of an object.
type ObjectAttrs struct {
	// The name of the object.
	Name string

	// The version of the object, if bucket versioning is enabled.
	Version string

	// The content type of the object, if set during upload.
	ContentType string

	// The size of the object, in bytes.
	Size int64

	// The computed ETag of the object.
	ETag string
}

func (b *Bucket) mapAttrs(attrs *types.ObjectAttrs) *ObjectAttrs {
	return &ObjectAttrs{
		Name:        b.fromCloudObject(attrs.Object),
		Version:     attrs.Version,
		ContentType: attrs.ContentType,
		Size:        attrs.Size,
		ETag:        attrs.ETag,
	}
}

// ListEntry describes an objects during listing.
type ListEntry struct {
	// The name of the object.
	Name string
	// The size of the object, in bytes.
	Size int64
	// The computed ETag of the object.
	ETag string
}

func (b *Bucket) mapListEntry(entry *types.ListEntry) *ListEntry {
	return &ListEntry{
		Name: b.fromCloudObject(entry.Object),
		Size: entry.Size,
		ETag: entry.ETag,
	}
}

type SignedUploadURL struct {
	// The signed URL
	URL string
}

// List lists objects in the bucket.
func (b *Bucket) List(ctx context.Context, query *Query, options ...ListOption) iter.Seq2[*ListEntry, error] {
	return func(yield func(*ListEntry, error) bool) {
		// Tracing state
		var (
			listErr  error
			observed uint64
			hasMore  bool
		)

		curr := b.mgr.rt.Current()
		if curr.Req != nil && curr.Trace != nil {
			startEventID := curr.Trace.BucketListObjectsStart(trace2.BucketListObjectsStartParams{
				EventParams: trace2.EventParams{
					TraceID: curr.Req.TraceID,
					SpanID:  curr.Req.SpanID,
					Goid:    curr.Goctr,
				},
				Bucket: b.name,
				Prefix: ptrOrNil(query.Prefix),
				Stack:  stack.Build(1),
			})

			defer curr.Trace.BucketListObjectsEnd(trace2.BucketListObjectsEndParams{
				StartID: startEventID,
				EventParams: trace2.EventParams{
					TraceID: curr.Req.TraceID,
					SpanID:  curr.Req.SpanID,
					Goid:    curr.Goctr,
				},
				Err:      listErr,
				Observed: observed,
				HasMore:  hasMore,
			})
		}

		iter := b.impl.List(b.mapQuery(ctx, query))
		for entry, err := range iter {
			if err != nil {
				listErr = err
				if !yield(nil, err) {
					return
				}
			}

			observed++
			if !yield(b.mapListEntry(entry), nil) {
				// Consumer didn't want any more entries; set hasMore = true
				hasMore = true
				return
			}
		}
	}
}

// Remove removes an object from the bucket.
func (b *Bucket) Remove(ctx context.Context, object string, options ...RemoveOption) error {
	var opts removeOptions
	for _, o := range options {
		o.applyRemove(&opts)
	}

	var removeErr error
	curr := b.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		startEventID := curr.Trace.BucketDeleteObjectsStart(trace2.BucketDeleteObjectsStartParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Bucket: b.name,
			Objects: []trace2.BucketDeleteObjectsEntry{
				{
					Object:  object,
					Version: ptrOrNil(opts.version),
				},
			},
			Stack: stack.Build(1),
		})

		defer curr.Trace.BucketDeleteObjectsEnd(trace2.BucketDeleteObjectsEndParams{
			StartID: startEventID,
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Err: removeErr,
		})
	}

	removeErr = b.impl.Remove(types.RemoveData{
		Ctx:     ctx,
		Object:  b.toCloudObject(object),
		Version: opts.version,
	})

	return removeErr
}

var (
	// ErrObjectNotFound is returned when requested object does not exist in the bucket.
	ErrObjectNotFound = types.ErrObjectNotExist

	// ErrPreconditionFailed is returned when a precondition for an operation is not met,
	// such as when an object already exists and Preconditions.NotExists is true.
	ErrPreconditionFailed = types.ErrPreconditionFailed
)

// Attrs returns the attributes of an object in the bucket.
// If the object does not exist, it returns ErrObjectNotFound.
func (b *Bucket) Attrs(ctx context.Context, object string, options ...AttrsOption) (*ObjectAttrs, error) {
	var opt attrsOptions
	for _, o := range options {
		o.applyAttrs(&opt)
	}

	var (
		attrs    *types.ObjectAttrs
		attrsErr error
	)

	curr := b.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		startEventID := curr.Trace.BucketObjectGetAttrsStart(trace2.BucketObjectGetAttrsStartParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Bucket:  b.name,
			Object:  object,
			Version: ptrOrNil(opt.version),
			Stack:   stack.Build(1),
		})

		defer func() {
			params := trace2.BucketObjectGetAttrsEndParams{
				StartID: startEventID,
				EventParams: trace2.EventParams{
					TraceID: curr.Req.TraceID,
					SpanID:  curr.Req.SpanID,
					Goid:    curr.Goctr,
				},
				Err: attrsErr,
			}
			if attrs != nil {
				size := uint64(attrs.Size)
				params.Attrs = &trace2.BucketObjectAttributes{
					Size:        &size,
					Version:     ptrOrNil(attrs.Version),
					ETag:        ptrOrNil(attrs.ETag),
					ContentType: ptrOrNil(attrs.ContentType),
				}
			}
			curr.Trace.BucketObjectGetAttrsEnd(params)
		}()
	}

	attrs, attrsErr = b.impl.Attrs(types.AttrsData{
		Ctx:     ctx,
		Object:  b.toCloudObject(object),
		Version: opt.version,
	})
	if attrsErr != nil {
		return nil, attrsErr
	}

	return b.mapAttrs(attrs), nil
}

// Generates an external URL to allow uploading an object to the bucket.
//
// Anyone with possession of the URL can write to the given object name
// without any additional auth.
func (b *Bucket) SignedUploadURL(ctx context.Context, object string, options ...UploadURLOption) (*SignedUploadURL, error) {
	var opt uploadURLOptions
	for _, o := range options {
		o.applyUploadURL(&opt)
	}
	if opt.ttl == 0 {
		opt.ttl = time.Hour
	}
	url, urlErr := b.impl.SignedUploadURL(types.UploadURLData{
		Ctx:    ctx,
		Object: b.toCloudObject(object),
		Ttl:    opt.ttl,
	})
	if urlErr != nil {
		return nil, urlErr
	}
	return &SignedUploadURL{URL: url}, nil
}

// Exists reports whether an object exists in the bucket.
func (b *Bucket) Exists(ctx context.Context, object string, options ...ExistsOption) (bool, error) {
	var opt existsOptions
	for _, o := range options {
		o.applyExists(&opt)
	}

	var (
		attrs    *types.ObjectAttrs
		attrsErr error
	)

	curr := b.mgr.rt.Current()
	if curr.Req != nil && curr.Trace != nil {
		startEventID := curr.Trace.BucketObjectGetAttrsStart(trace2.BucketObjectGetAttrsStartParams{
			EventParams: trace2.EventParams{
				TraceID: curr.Req.TraceID,
				SpanID:  curr.Req.SpanID,
				Goid:    curr.Goctr,
			},
			Bucket:  b.name,
			Object:  object,
			Version: ptrOrNil(opt.version),
			Stack:   stack.Build(1),
		})

		defer func() {
			params := trace2.BucketObjectGetAttrsEndParams{
				StartID: startEventID,
				EventParams: trace2.EventParams{
					TraceID: curr.Req.TraceID,
					SpanID:  curr.Req.SpanID,
					Goid:    curr.Goctr,
				},
				Err: attrsErr,
			}
			if attrs != nil {
				size := uint64(attrs.Size)
				params.Attrs = &trace2.BucketObjectAttributes{
					Size:        &size,
					Version:     ptrOrNil(attrs.Version),
					ETag:        ptrOrNil(attrs.ETag),
					ContentType: ptrOrNil(attrs.ContentType),
				}
			}
			curr.Trace.BucketObjectGetAttrsEnd(params)
		}()
	}

	attrs, attrsErr = b.impl.Attrs(types.AttrsData{
		Ctx:     ctx,
		Object:  b.toCloudObject(object),
		Version: opt.version,
	})
	if errors.Is(attrsErr, ErrObjectNotFound) {
		return false, nil
	} else if attrsErr != nil {
		return false, attrsErr
	}
	return true, nil
}

func (b *Bucket) toCloudObject(object string) types.CloudObject {
	return types.CloudObject(b.cloudPrefix() + object)
}

// cloudPrefix computes the cloud prefix to use.
// It adds the current test name as a prefix when running tests, for test isolation.
func (b *Bucket) cloudPrefix() string {
	prefix := b.baseCloudPrefix

	if b.mgr.static.Testing {
		test := b.mgr.ts.CurrentTest()
		if prefix != "" {
			prefix += "/"
		}
		prefix += test.Name() + "/__test__/"
	}

	return prefix
}

func (b *Bucket) fromCloudObject(object types.CloudObject) string {
	return strings.TrimPrefix(string(object), b.cloudPrefix())
}

func ptrOrNil[V comparable](val V) *V {
	var zero V
	if val != zero {
		return &val
	}
	return nil
}
