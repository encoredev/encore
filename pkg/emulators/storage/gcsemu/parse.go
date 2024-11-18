package gcsemu

import (
	"net/url"
	"regexp"
)

const (
	// example: "/storage/v1/b/my-bucket/o/2013-tax-returns.pdf" (for a file) or "/storage/v1/b/my-bucket/o" (for a bucket)
	gcsObjectPathPattern = "/storage/v1/b/([^\\/]+)/o(?:/(.+))?"
	// example: "//b/my-bucket/o/2013-tax-returns.pdf" (for a file) or "/b/my-bucket/o" (for a bucket)
	gcsObjectPathPattern2 = "/b/([^\\/]+)/o(?:/(.+))?"
	// example: "/storage/v1/b/my-bucket
	gcsBucketPathPattern = "/storage/v1/b(?:/([^\\/]+))?"
	// example: "/my-bucket/2013-tax-returns.pdf" (for a file)
	gcsStoragePathPattern = "/([^\\/]+)/(.+)"
)

var (
	gcsObjectPathRegex  = regexp.MustCompile(gcsObjectPathPattern)
	gcsObjectPathRegex2 = regexp.MustCompile(gcsObjectPathPattern2)
	gcsBucketPathRegex  = regexp.MustCompile(gcsBucketPathPattern)
	gcsStoragePathRegex = regexp.MustCompile(gcsStoragePathPattern)
)

// GcsParams represent a parsed GCS url.
type GcsParams struct {
	Bucket   string
	Object   string
	IsPublic bool
}

// ParseGcsUrl parses a GCS url.
func ParseGcsUrl(u *url.URL) (*GcsParams, bool) {
	if g, ok := parseGcsUrl(gcsObjectPathRegex, u); ok {
		return g, true
	}
	if g, ok := parseGcsUrl(gcsBucketPathRegex, u); ok {
		return g, true
	}
	if g, ok := parseGcsUrl(gcsObjectPathRegex2, u); ok {
		return g, true
	}
	if g, ok := parseGcsUrl(gcsStoragePathRegex, u); ok {
		g.IsPublic = true
		return g, true
	}
	return nil, false
}

func parseGcsUrl(re *regexp.Regexp, u *url.URL) (*GcsParams, bool) {
	submatches := re.FindStringSubmatch(u.Path)
	if submatches == nil {
		return nil, false
	}

	g := &GcsParams{}
	if len(submatches) > 1 {
		g.Bucket = submatches[1]
	}
	if len(submatches) > 2 {
		g.Object = submatches[2]
	}
	return g, true
}
