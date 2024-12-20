package types

import (
	"context"
	"errors"
	"io"
	"iter"
	"time"
)

type BucketImpl interface {
	Upload(data UploadData) (Uploader, error)
	Download(data DownloadData) (Downloader, error)
	List(data ListData) iter.Seq2[*ListEntry, error]
	Remove(data RemoveData) error
	Attrs(data AttrsData) (*ObjectAttrs, error)
	SignedUploadURL(data UploadURLData) (string, error)
}

// CloudObject is the cloud name for an object.
// It can differ from the logical name when using a prefix bucket.
type CloudObject string

func (o CloudObject) String() string { return string(o) }

type UploadData struct {
	Ctx    context.Context
	Object CloudObject

	Attrs UploadAttrs
	Pre   Preconditions
}

type Preconditions struct {
	NotExists bool
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
	Object CloudObject

	// Non-zero to download a specific version
	Version string
}

type Downloader interface {
	io.Reader
	io.Closer
}

type ObjectAttrs struct {
	Object      CloudObject
	Version     string
	ContentType string
	Size        int64
	ETag        string
}

type ListData struct {
	Ctx    context.Context
	Prefix string
	Limit  *int64
}

type ListEntry struct {
	Object CloudObject
	Size   int64
	ETag   string
}

type RemoveData struct {
	Ctx    context.Context
	Object CloudObject

	Version string // non-zero means specific version
}

type AttrsData struct {
	Ctx    context.Context
	Object CloudObject

	Version string // non-zero means specific version
}

type UploadURLData struct {
	Ctx    context.Context
	Object CloudObject

	Ttl time.Duration
}

//publicapigen:keep
var (
	//publicapigen:keep
	ErrObjectNotExist = errors.New("objects: object doesn't exist")
	//publicapigen:keep
	ErrPreconditionFailed = errors.New("objects: precondition failed")
)
