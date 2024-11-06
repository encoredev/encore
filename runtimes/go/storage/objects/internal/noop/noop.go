package noop

import (
	"fmt"

	"encore.dev/storage/objects/internal/types"
)

type BucketImpl struct {
	EncoreName string
}

func (b *BucketImpl) NewUpload(data types.UploadData) types.Uploader {
	return &noopUploader{}
}

type noopUploader struct{}

func (*noopUploader) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("cannot upload to noop bucket")
}

func (*noopUploader) Complete() (*types.Attrs, error) {
	return nil, fmt.Errorf("cannot upload to noop bucket")
}

func (*noopUploader) Abort(err error) {
}
