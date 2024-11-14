//go:build encore_app

package objects

// NewBucket declares a new object storage bucket.
//
// See https://encore.dev/docs/primitives/object-storage for more information.
func NewBucket(name string, cfg BucketConfig) *Bucket {
	return newBucket(Singleton, name)
}

// constStr is a string that can only be provided as a constant.
//
//publicapigen:keep
type constStr string

// Named returns a database object connected to the database with the given name.
//
// The name must be a string literal constant, to facilitate static analysis.
func Named(name constStr) *Bucket {
	return newBucket(Singleton, string(name))
}
