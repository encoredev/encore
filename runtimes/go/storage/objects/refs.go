package objects

import "context"

// BucketPerms is the type constraint for all permission-declaring
// interfaces that can be used with BucketRef.
type BucketPerms interface {
	Meta() BucketMeta
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
	Upload(ctx context.Context, object string) *Writer

	// Meta returns metadata about the bucket.
	Meta() BucketMeta
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
//	var ref = objects.BucketRef[objects.Uploader](MyBucket)
//	// ref.Publish(...) can now be used to publish messages to MyTopic.
func BucketRef[P BucketPerms](bucket *Bucket) P {
	return any(bucketRef{Bucket: bucket}).(P)
}

type bucketRef struct {
	*Bucket
}
