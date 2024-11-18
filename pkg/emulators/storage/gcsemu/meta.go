package gcsemu

import (
	"fmt"
	"mime"
	"strings"

	"google.golang.org/api/storage/v1"
)

// BucketMeta returns a default bucket metadata for the given name and base url.
func BucketMeta(baseUrl HttpBaseUrl, bucket string) *storage.Bucket {
	return &storage.Bucket{
		Kind:         "storage#bucket",
		Name:         bucket,
		SelfLink:     BucketUrl(baseUrl, bucket),
		StorageClass: "STANDARD",
	}
}

// InitScrubbedMeta "bakes" metadata with intrinsic values and removes fields that are intrinsic / computed.
func InitScrubbedMeta(meta *storage.Object, filename string) {
	parts := strings.Split(filename, ".")
	ext := parts[len(parts)-1]

	if meta.ContentType == "" {
		meta.ContentType = mime.TypeByExtension(ext)
	}
	meta.Name = filename
	ScrubMeta(meta)
}

// InitMetaWithUrls "bakes" metadata with intrinsic values, including computed links.
func InitMetaWithUrls(baseUrl HttpBaseUrl, meta *storage.Object, bucket string, filename string, size uint64) {
	parts := strings.Split(filename, ".")
	ext := parts[len(parts)-1]

	meta.Bucket = bucket
	if meta.ContentType == "" {
		meta.ContentType = mime.TypeByExtension(ext)
	}
	meta.Kind = "storage#object"
	meta.MediaLink = ObjectUrl(baseUrl, bucket, filename) + "?alt=media"
	meta.Name = filename
	meta.SelfLink = ObjectUrl(baseUrl, bucket, filename)
	meta.Size = size
	meta.StorageClass = "STANDARD"
}

// ScrubMeta removes fields that are intrinsic / computed for minimal storage.
func ScrubMeta(meta *storage.Object) {
	meta.Bucket = ""
	meta.Kind = ""
	meta.MediaLink = ""
	meta.SelfLink = ""
	meta.Size = 0
	meta.StorageClass = ""
}

// BucketUrl returns the URL for a bucket.
func BucketUrl(baseUrl HttpBaseUrl, bucket string) string {
	return fmt.Sprintf("%sstorage/v1/b/%s", normalizeBaseUrl(baseUrl), bucket)
}

// ObjectUrl returns the URL for a file.
func ObjectUrl(baseUrl HttpBaseUrl, bucket string, filepath string) string {
	return fmt.Sprintf("%sstorage/v1/b/%s/o/%s", normalizeBaseUrl(baseUrl), bucket, filepath)
}

// HttpBaseUrl represents the emulator base URL, including trailing slash; e.g. https://www.googleapis.com/
type HttpBaseUrl string

// when the caller doesn't really care about the object meta URLs
const dontNeedUrls = HttpBaseUrl("")

func normalizeBaseUrl(baseUrl HttpBaseUrl) HttpBaseUrl {
	if baseUrl == dontNeedUrls || baseUrl == "https://storage.googleapis.com/" {
		return "https://www.googleapis.com/"
	} else if baseUrl == "http://storage.googleapis.com/" {
		return "http://www.googleapis.com/"
	} else {
		return baseUrl
	}
}
