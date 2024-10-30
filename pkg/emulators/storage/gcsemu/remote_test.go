package gcsemu

import (
	"context"
	"os"
	"testing"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"gotest.tools/v3/assert"
)

func TestRealStore(t *testing.T) {
	bucket := os.Getenv("BUCKET_ID")
	if bucket == "" {
		t.Skip("BUCKET_ID must be set to run this")
	}

	ctx := context.Background()
	gcsClient, err := storage.NewClient(ctx)
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = gcsClient.Close()
	})

	bh := BucketHandle{
		Name:         bucket,
		BucketHandle: gcsClient.Bucket(bucket),
	}

	// Instead of `initBucket`, just check that it exists.
	_, err = bh.Attrs(ctx)
	assert.NilError(t, err)

	t.Parallel()
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.f(t, bh)
		})
	}

	httpClient, err := google.DefaultClient(context.Background())
	assert.NilError(t, err)
	t.Run("RawHttp", func(t *testing.T) {
		t.Parallel()
		testRawHttp(t, bh, httpClient, "https://storage.googleapis.com")
	})
}
