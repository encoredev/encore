package objects

import (
	"encr.dev/pkg/errors"
)

const (
	objectsNewBucketHelp = "For example `objects.NewBucket(\"my-bucket\", objects.BucketConfig{ Versioned: false })`"

	objectsBucketUsageHelp = "The bucket can only be referenced by calling methods on it, or by using objects.BucketRef."
)

var (
	errRange = errors.Range(
		"pubsub",
		"For more information on Object Storage, see https://encore.dev/docs/primitives/object-storage",
	)

	errNewBucketArgCount = errRange.Newf(
		"Invalid objects.NewBucket call",
		"A call to objects.NewBucket requires 2 arguments; the bucket name and the config object, got %d arguments.",
		errors.PrependDetails(objectsNewBucketHelp),
	)

	errInvalidBucketUsage = errRange.New(
		"Invalid reference to objects.Bucket",
		"A reference to an objects.Bucket is not permissible here.",
		errors.PrependDetails(objectsBucketUsageHelp),
	)

	ErrBucketNameNotUnique = errRange.New(
		"Duplicate bucket name",
		"An object storage bucket name must be unique.",

		errors.PrependDetails("If you wish to reuse the same bucket, then you can export the original Bucket object and reference it from here."),
	)

	ErrUnsupportedOperationOutsideService = errRange.Newf(
		"Unsupported bucket operation outside of service",
		"The %s operation can only be performed within a service. Use objects.BucketRef to pass the bucket reference to other, non-service components.",
	)

	errBucketRefNoTypeArgs = errRange.New(
		"Invalid call to objects.BucketRef",
		"A type argument indicating the requested permissions must be provided.",
	)

	errBucketRefInvalidPerms = errRange.New(
		"Unrecognized permissions in call to objects.BucketRef",
		"The supported permissions are objects.{Uploader,Downloader,Attrser,Lister,Remover,PublicURLer,ReadWriter}.",
	)

	ErrBucketRefOutsideService = errRange.New(
		"Call to objects.BucketRef outside service",
		"objects.BucketRef can only be called from within a service.",
	)

	ErrBucketNotPublic = errRange.New(
		"Call to PublicURL for non-public objects.Bucket",
		"The PublicURL method can only be called on a public bucket.",
	)
)
