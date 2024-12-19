package objects

import (
	"time"

	"encore.dev/storage/objects/internal/types"
)

// DownloadOption describes available options for the Download operation.
type DownloadOption interface {
	//publicapigen:keep
	downloadOption()

	applyDownload(*downloadOptions)
}

// WithVersion specifies that the operation should be performed
// against the provided version of the object.
func WithVersion(version string) withVersionOption {
	return withVersionOption{version: version}
}

//publicapigen:keep
type withVersionOption struct {
	version string
}

//publicapigen:keep
func (o withVersionOption) downloadOption() {}

//publicapigen:keep
func (o withVersionOption) removeOption() {}

//publicapigen:keep
func (o withVersionOption) attrsOption() {}

//publicapigen:keep
func (o withVersionOption) existsOption() {}

//publicapigen:keep
func (o withTTLOption) uploadURLOption() {}

func (o withVersionOption) applyDownload(opts *downloadOptions) { opts.version = o.version }
func (o withVersionOption) applyRemove(opts *removeOptions)     { opts.version = o.version }
func (o withVersionOption) applyAttrs(opts *attrsOptions)       { opts.version = o.version }
func (o withVersionOption) applyExists(opts *existsOptions)     { opts.version = o.version }
func (o withTTLOption) applyUploadURL(opts *uploadURLOptions)   { opts.ttl = o.ttl }

// WithTTL is used for signed URLs, to specify the lifetime of the generated
// URL. The max value is seven days. The default lifetime, if this
// option is missing, is one hour.
func WithTTL(ttl time.Duration) withTTLOption {
	return withTTLOption{ttl: ttl}
}

//publicapigen:keep
type withTTLOption struct {
	ttl time.Duration
}

//publicapigen:keep
type downloadOptions struct {
	version string
}

// UploadOption describes available options for the Upload operation.
type UploadOption interface {
	uploadOption()

	applyUpload(*uploadOptions)
}

// WithPreconditions is an UploadOption for only uploading an object
// if certain preconditions are met.
func WithPreconditions(pre Preconditions) withPreconditionsOption {
	return withPreconditionsOption{pre: pre}
}

// Preconditions are the available preconditions for an upload operation.
type Preconditions struct {
	// NotExists specifies that the object must not exist prior to uploading.
	NotExists bool
}

//publicapigen:keep
type withPreconditionsOption struct {
	pre Preconditions
}

//publicapigen:keep
func (o withPreconditionsOption) uploadOption() {}

func (o withPreconditionsOption) applyUpload(opts *uploadOptions) {
	opts.pre = o.pre
}

// UploadAttrs specifies additional object attributes to set during upload.
type UploadAttrs struct {
	// ContentType specifies the content type of the object.
	ContentType string
}

// WithUploadAttrs is an UploadOption for specifying additional object attributes
// to set during upload.
func WithUploadAttrs(attrs UploadAttrs) withUploadAttrsOption {
	return withUploadAttrsOption{attrs: attrs}
}

//publicapigen:keep
type withUploadAttrsOption struct {
	attrs UploadAttrs
}

//publicapigen:keep
func (o withUploadAttrsOption) uploadOption() {}

func (o withUploadAttrsOption) applyUpload(opts *uploadOptions) {
	opts.attrs = types.UploadAttrs{
		ContentType: o.attrs.ContentType,
	}
}

type uploadOptions struct {
	attrs types.UploadAttrs
	pre   Preconditions
}

// ListOption describes available options for the List operation.
type ListOption interface {
	//publicapigen:keep
	listOption()

	applyList(*listOptions)
}

type listOptions struct{}

// RemoveOption describes available options for the Remove operation.
type RemoveOption interface {
	//publicapigen:keep
	removeOption()

	applyRemove(*removeOptions)
}

type removeOptions struct {
	version string
}

// AttrsOption describes available options for the Attrs operation.
type AttrsOption interface {
	//publicapigen:keep
	attrsOption()

	applyAttrs(*attrsOptions)
}

type attrsOptions struct {
	version string
}

// UploadURLOption describes available options for the SignedUploadURL operation.
type UploadURLOption interface {
	//publicapigen:keep
	uploadURLOption()

	applyUploadURL(*uploadURLOptions)
}

type uploadURLOptions struct {
	ttl time.Duration
}

// ExistsOption describes available options for the Exists operation.
type ExistsOption interface {
	//publicapigen:keep
	existsOption()

	applyExists(*existsOptions)
}

type existsOptions struct {
	version string
}

// PublicURLOption describes available options for the PublicURL operation.
type PublicURLOption interface {
	//publicapigen:keep
	publicURLOption()

	applyPublicURL(*publicURLOptions)
}

// No options yet
type publicURLOptions struct{}
