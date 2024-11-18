package objects

import "encore.dev/storage/objects/internal/types"

type DownloadOption interface {
	downloadOption(*downloadOptions)
}

func WithVersion(version string) withVersionOption {
	return withVersionOption{version: version}
}

//publicapigen:keep
type withVersionOption struct {
	version string
}

//publicapigen:keep
func (o withVersionOption) downloadOptions(opts *downloadOptions) { opts.version = o.version }
func (o withVersionOption) removeOptions(opts *removeOptions)     { opts.version = o.version }
func (o withVersionOption) attrsOptions(opts *attrsOptions)       { opts.version = o.version }
func (o withVersionOption) existsOptions(opts *existsOptions)     { opts.version = o.version }

//publicapigen:keep
type downloadOptions struct {
	version string
}

type UploadOption interface {
	uploadOption(*uploadOptions)
}

// WithPreconditions is an UploadOption for only uploading an object
// if certain preconditions are met.
func WithPreconditions(pre Preconditions) withPreconditionsOption {
	return withPreconditionsOption{pre: pre}
}

type Preconditions struct {
	NotExists bool
}

//publicapigen:keep
type withPreconditionsOption struct {
	pre Preconditions
}

//publicapigen:keep
func (o withPreconditionsOption) uploadOption(opts *uploadOptions) {
	opts.pre = o.pre
}

type UploadAttrs struct {
	ContentType string
}

func WithUploadAttrs(attrs UploadAttrs) withUploadAttrsOption {
	return withUploadAttrsOption{attrs: attrs}
}

//publicapigen:keep
type withUploadAttrsOption struct {
	attrs UploadAttrs
}

//publicapigen:keep
func (o withUploadAttrsOption) uploadOption(opts *uploadOptions) {
	opts.attrs = types.UploadAttrs{
		ContentType: o.attrs.ContentType,
	}
}

//publicapigen:keep
type uploadOptions struct {
	attrs types.UploadAttrs
	pre   Preconditions
}

type ListOption interface {
	listOption(*listOptions)
}

//publicapigen:keep
type listOptions struct{}

type RemoveOption interface {
	removeOption(*removeOptions)
}

//publicapigen:keep
type removeOptions struct {
	version string
}

type AttrsOption interface {
	attrsOption(*attrsOptions)
}

//publicapigen:keep
type attrsOptions struct {
	version string
}

type ExistsOption interface {
	existsOption(*existsOptions)
}

//publicapigen:keep
type existsOptions struct {
	version string
}
