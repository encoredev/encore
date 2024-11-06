//go:build encore_app

package objects

// NewBucket declares a new object storage bucket.
//
// See https://encore.dev/docs/develop/object-storage for more information.
func NewBucket(name string, cfg BucketConfig) *Bucket {
	return newBucket(Singleton, name, cfg)
}
