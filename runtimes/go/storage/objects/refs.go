package objects

import (
	"context"
	"iter"
	"net/url"
)

// BucketPerms is the type constraint for all permission-declaring
// interfaces that can be used with BucketRef.
type BucketPerms interface {
	perms()
}

// ReadWriter is a utility permission interface that provides
// all the read-write permissions.
type ReadWriter interface {
	Uploader
	Downloader
	Remover
	Lister
	Attrser
}

// Uploader is the interface for uploading objects to a bucket.
// It can be used in conjunction with [BucketRef] to declare
// a reference that can upload objects to the bucket.
//
// For example:
//
//	var MyBucket = objects.NewBucket(...)
//	var ref = objects.BucketRef[objects.Uploader](MyBucket)
//
// The ref object can then be used to upload objects and can be
// passed around freely within the service, without being subject
// to Encore's static analysis restrictions that apply to MyBucket.
type Uploader interface {
	// Upload begins uploading an object to the bucket.
	Upload(ctx context.Context, object string, options ...UploadOption) *Writer

	// SignedUploadURL returns a signed URL that can be used to upload directly to
	// storage, without any other authentication.
	SignedUploadURL(ctx context.Context, object string, options ...UploadURLOption) (string, error)

	perms()
}

// Downloader is the interface for downloading objects from a bucket.
// It can be used in conjunction with [BucketRef] to declare
// a reference that can download objects from the bucket.
//
// For example:
//
//	var MyBucket = objects.NewBucket(...)
//	var ref = objects.BucketRef[objects.Downloader](MyBucket)
//
// The ref object can then be used to download objects and can be
// passed around freely within the service, without being subject
// to Encore's static analysis restrictions that apply to MyBucket.
type Downloader interface {
	// Download downloads an object from the bucket.
	Download(ctx context.Context, object string, options ...DownloadOption) *Reader

	perms()
}

// Lister is the interface for listing objects in a bucket.
// It can be used in conjunction with [BucketRef] to declare
// a reference that can list objects in the bucket.
//
// For example:
//
//	var MyBucket = objects.NewBucket(...)
//	var ref = objects.BucketRef[objects.Lister](MyBucket)
//
// The ref object can then be used to list objects and can be
// passed around freely within the service, without being subject
// to Encore's static analysis restrictions that apply to MyBucket.
type Lister interface {
	// List lists objects in the bucket.
	List(ctx context.Context, query *Query, options ...ListOption) iter.Seq2[*ListEntry, error]

	perms()
}

// Remove is the interface for removing objects from a bucket.
// It can be used in conjunction with [BucketRef] to declare
// a reference that can remove objects from the bucket.
//
// For example:
//
//	var MyBucket = objects.NewBucket(...)
//	var ref = objects.BucketRef[objects.Remover](MyBucket)
//
// The ref object can then be used to remove objects and can be
// passed around freely within the service, without being subject
// to Encore's static analysis restrictions that apply to MyBucket.
type Remover interface {
	// Remove removes an object from the bucket.
	Remove(ctx context.Context, object string, options ...RemoveOption) error

	perms()
}

// Attrser is the interface for resolving objects' attributes in a bucket.
// It can be used in conjunction with [BucketRef] to declare
// a reference that can check object attributes in a bucket.
//
// For example:
//
//	var MyBucket = objects.NewBucket(...)
//	var ref = objects.BucketRef[objects.Attrser](MyBucket)
//
// The ref object can then be used to remove objects and can be
// passed around freely within the service, without being subject
// to Encore's static analysis restrictions that apply to MyBucket.
type Attrser interface {
	// Attrs resolves the attributes of an object.
	Attrs(ctx context.Context, object string, options ...AttrsOption) (*ObjectAttrs, error)

	// Exists checks whether an object exists in the bucket.
	Exists(ctx context.Context, object string, options ...ExistsOption) (bool, error)

	perms()
}

// PublicURLer is the interface for resolving the public URL for an object.
// It can be used in conjunction with [BucketRef] to declare
// a reference that can resolve an object's public URL.
//
// For example:
//
//	var MyBucket = objects.NewBucket(...)
//	var ref = objects.BucketRef[objects.PublicURLer](MyBucket)
//
// The ref object can then be used to remove objects and can be
// passed around freely within the service, without being subject
// to Encore's static analysis restrictions that apply to MyBucket.
type PublicURLer interface {
	// PublicURL resolves the public URL for retrieving an object.
	PublicURL(object string, options ...PublicURLOption) *url.URL

	perms()
}

// BucketRef returns an interface reference to a bucket,
// that can be freely passed around within a service
// without being subject to Encore's typical static analysis
// restrictions that normally apply to *Bucket objects.
//
// This works because using BucketRef effectively declares
// which operations you want to be able to perform since the
// type argument P must be a permission-declaring interface (implementing BucketPerms).
//
// The returned reference is scoped down to those permissions.
//
// For example:
//
//	var MyBucket = objects.NewBucket(...)
//	var ref = objects.BucketRef[objects.ReadWriter](MyBucket)
//	// ref.Upload(...) can now be used to upload objects to MyBucket.
//
// Multiple permissions can be combined by defining a custom interface
// that embeds multiple permission interfaces:
//
//	var ref = objects.BucketRef[interface { objects.Uploader; objects.Downloader }](MyBucket)
func BucketRef[P BucketPerms](bucket *Bucket) P {
	return any(bucketRef{Bucket: bucket}).(P)
}

type bucketRef struct {
	*Bucket
}

func (r bucketRef) perms() {}
