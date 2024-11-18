package gcsemu

import (
	"context"
	"os"

	"google.golang.org/api/storage/v1"
)

// Store is an interface to either on-disk or in-mem storage
type Store interface {
	// CreateBucket creates a bucket; no error if the bucket already exists.
	CreateBucket(bucket string) error

	// Get returns a bucket's metadata.
	GetBucketMeta(baseUrl HttpBaseUrl, bucket string) (*storage.Bucket, error)

	// Get returns a file's contents and metadata.
	Get(url HttpBaseUrl, bucket string, filename string) (*storage.Object, []byte, error)

	// GetMeta returns a file's metadata.
	GetMeta(url HttpBaseUrl, bucket string, filename string) (*storage.Object, error)

	// Add creates the specified file.
	Add(bucket string, filename string, contents []byte, meta *storage.Object) error

	// UpdateMeta updates the given file's metadata.
	UpdateMeta(bucket string, filename string, meta *storage.Object, metagen int64) error

	// Copy copies the file
	Copy(srcBucket string, srcFile string, dstBucket string, dstFile string) (bool, error)

	// Delete deletes the file.
	Delete(bucket string, filename string) error

	// ReadMeta reads the GCS metadata for a file, when you already have file info.
	ReadMeta(url HttpBaseUrl, bucket string, filename string, fInfo os.FileInfo) (*storage.Object, error)

	// Walks the given bucket.
	Walk(ctx context.Context, bucket string, cb func(ctx context.Context, filename string, fInfo os.FileInfo) error) error
}
