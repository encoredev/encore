package types

import (
	"context"
	"io"
)

type Uploader interface {
	io.Writer
	Abort(err error)
	Complete() (*Attrs, error)
}

type Attrs struct {
	Version string
}

type BucketImpl interface {
	NewUpload(data UploadData) Uploader
}

type UploadData struct {
	Ctx    context.Context
	Object string
}
