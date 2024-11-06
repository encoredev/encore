package types

type BucketConfig struct {
	// Whether objects stored in the bucket should be versioned.
	//
	// If true, the bucket will store multiple versions of each object
	// whenever it changes, as opposed to overwriting the old version.
	Versioned bool
}
