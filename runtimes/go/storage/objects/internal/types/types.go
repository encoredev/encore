package types

import (
	"context"
	"errors"
	"io"
	"iter"
)

type BucketImpl interface {
	Upload(data UploadData) (Uploader, error)
	Download(data DownloadData) (Downloader, error)
	List(data ListData) iter.Seq2[*ListEntry, error]
	Remove(data RemoveData) error
	Attrs(data AttrsData) (*ObjectAttrs, error)
}

type UploadData struct {
	Ctx    context.Context
	Object string

	Attrs UploadAttrs
}

type UploadAttrs struct {
	ContentType string
}

type Uploader interface {
	io.Writer
	Abort(err error)
	Complete() (*ObjectAttrs, error)
}

type DownloadData struct {
	Ctx    context.Context
	Object string

	// Non-zero to download a specific version
	Version string
}

type Downloader interface {
	io.Reader
	io.Closer
}

type ObjectAttrs struct {
	Version     string
	ContentType string
	Size        int64
	ETag        string
}

type ListData struct {
	Ctx    context.Context
	Prefix string
	Limit  int64
}

type ListEntry struct {
	Name string
	Size int64
	ETag string
}

type RemoveData struct {
	Ctx    context.Context
	Object string

	Version string // non-zero means specific version
}

type AttrsData struct {
	Ctx    context.Context
	Object string

	Version string // non-zero means specific version
}

var ErrObjectNotExist = errors.New("objects: object doesn't exist")
