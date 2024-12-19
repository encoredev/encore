package noop

import (
	"fmt"
	"iter"

	"encore.dev/storage/objects/internal/types"
)

type BucketImpl struct {
	EncoreName string
}

func (b *BucketImpl) Download(data types.DownloadData) (types.Downloader, error) {
	return nil, fmt.Errorf("cannot download from noop bucket")
}

func (b *BucketImpl) Upload(data types.UploadData) (types.Uploader, error) {
	return nil, fmt.Errorf("cannot upload to noop bucket")
}

func (b *BucketImpl) List(data types.ListData) iter.Seq2[*types.ListEntry, error] {
	return func(yield func(*types.ListEntry, error) bool) {
		yield(nil, fmt.Errorf("cannot list objects from noop bucket"))
	}
}

func (b *BucketImpl) Remove(data types.RemoveData) error {
	return fmt.Errorf("cannot remove from noop bucket")
}

func (b *BucketImpl) Attrs(data types.AttrsData) (*types.ObjectAttrs, error) {
	return nil, fmt.Errorf("cannot get attributes from noop bucket")
}

func (b *BucketImpl) SignedUploadURL(data types.UploadURLData) (string, error) {
	return "", fmt.Errorf("cannot get upload url from noop bucket")
}
