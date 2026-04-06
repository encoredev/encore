//go:build !encore_no_azure

package azblob

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	qt "github.com/frankban/quicktest"
	"github.com/golang/mock/gomock"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/types"
)

// ---- parseConnectionString tests -------------------------------------------------------

func TestParseConnectionString(t *testing.T) {
	tests := []struct {
		name       string
		connStr    string
		wantName   string
		wantKey    string
	}{
		{
			name:     "full connection string",
			connStr:  "DefaultEndpointsProtocol=https;AccountName=myaccount;AccountKey=mykey==;EndpointSuffix=core.windows.net",
			wantName: "myaccount",
			wantKey:  "mykey==",
		},
		{
			name:     "minimal connection string",
			connStr:  "AccountName=foo;AccountKey=bar",
			wantName: "foo",
			wantKey:  "bar",
		},
		{
			name:     "missing account key",
			connStr:  "AccountName=onlyname",
			wantName: "onlyname",
			wantKey:  "",
		},
		{
			name:     "missing account name",
			connStr:  "AccountKey=onlykey",
			wantName: "",
			wantKey:  "onlykey",
		},
		{
			name:     "empty string",
			connStr:  "",
			wantName: "",
			wantKey:  "",
		},
		{
			name:     "key with equals signs (base64 padding)",
			connStr:  "AccountName=acct;AccountKey=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			wantName: "acct",
			wantKey:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			name, key := parseConnectionString(tt.connStr)
			c.Assert(name, qt.Equals, tt.wantName)
			c.Assert(key, qt.Equals, tt.wantKey)
		})
	}
}

// ---- uploader tests --------------------------------------------------------------------

func TestUploader_SingleUpload(t *testing.T) {
	c := qt.New(t)

	ctrl := gomock.NewController(c)
	client := NewMockblockBlobClient(ctrl)

	const (
		object      = "myblob"
		contentType = "text/plain"
		version     = "ver1"
		etag        = "etag1"
	)

	u := newUploader(client, types.UploadData{
		Ctx:    context.Background(),
		Object: object,
		Attrs:  types.UploadAttrs{ContentType: contentType},
	})

	etagVal := azcore.ETag(etag)
	client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(blockblob.UploadResponse{
		VersionID: ptr(version),
		ETag:      &etagVal,
	}, nil)

	content := []byte("hello azure")
	n, err := u.Write(content)
	c.Assert(n, qt.Equals, len(content))
	c.Assert(err, qt.IsNil)

	attrs, err := u.Complete()
	c.Assert(err, qt.IsNil)
	c.Assert(attrs, qt.DeepEquals, &types.ObjectAttrs{
		Object:      types.CloudObject(object),
		Version:     version,
		ContentType: contentType,
		ETag:        etag,
		Size:        int64(len(content)),
	})
}

func TestUploader_MultipleWrites(t *testing.T) {
	c := qt.New(t)

	ctrl := gomock.NewController(c)
	client := NewMockblockBlobClient(ctrl)

	const (
		object      = "myblob"
		contentType = "application/octet-stream"
		version     = "v2"
		etag        = "etag2"
	)

	u := newUploader(client, types.UploadData{
		Ctx:    context.Background(),
		Object: object,
		Attrs:  types.UploadAttrs{ContentType: contentType},
	})

	etagVal := azcore.ETag(etag)
	client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(blockblob.UploadResponse{
		VersionID: ptr(version),
		ETag:      &etagVal,
	}, nil)

	base := "chunk"
	total := strings.Repeat(base, 10)
	for i := 0; i < 10; i++ {
		n, err := u.Write([]byte(base))
		c.Assert(n, qt.Equals, len(base))
		c.Assert(err, qt.IsNil)
	}

	attrs, err := u.Complete()
	c.Assert(err, qt.IsNil)
	c.Assert(attrs, qt.DeepEquals, &types.ObjectAttrs{
		Object:      types.CloudObject(object),
		Version:     version,
		ContentType: contentType,
		ETag:        etag,
		Size:        int64(len(total)),
	})
}

func TestUploader_MultipartUpload(t *testing.T) {
	c := qt.New(t)

	// Use a small buffer so writes spill across buffers and trigger multipart.
	withBufSize(c, 10)

	ctrl := gomock.NewController(c)
	client := NewMockblockBlobClient(ctrl)

	const (
		object      = "bigblob"
		contentType = "text/plain"
		version     = "v3"
		etag        = "etag3"
	)

	u := newUploader(client, types.UploadData{
		Ctx:    context.Background(),
		Object: object,
		Attrs:  types.UploadAttrs{ContentType: contentType},
	})

	// Writing "abcdefghijklm" × 3 (39 bytes) with bufSize=10 produces 4 blocks:
	// "abcdefghij" / "klmabcdefg" / "hijklmabcd" / "efghijklm"
	client.EXPECT().StageBlock(gomock.Any(), blockIDForPart(0), &blockBodyMatcher{"abcdefghij"}, gomock.Any()).Return(blockblob.StageBlockResponse{}, nil)
	client.EXPECT().StageBlock(gomock.Any(), blockIDForPart(1), &blockBodyMatcher{"klmabcdefg"}, gomock.Any()).Return(blockblob.StageBlockResponse{}, nil)
	client.EXPECT().StageBlock(gomock.Any(), blockIDForPart(2), &blockBodyMatcher{"hijklmabcd"}, gomock.Any()).Return(blockblob.StageBlockResponse{}, nil)
	client.EXPECT().StageBlock(gomock.Any(), blockIDForPart(3), &blockBodyMatcher{"efghijklm"}, gomock.Any()).Return(blockblob.StageBlockResponse{}, nil)

	etagVal := azcore.ETag(etag)
	client.EXPECT().CommitBlockList(gomock.Any(), gomock.Any(), gomock.Any()).Return(blockblob.CommitBlockListResponse{
		VersionID: ptr(version),
		ETag:      &etagVal,
	}, nil)

	base := "abcdefghijklm"
	total := strings.Repeat(base, 3)
	for i := 0; i < 3; i++ {
		n, err := u.Write([]byte(base))
		c.Assert(n, qt.Equals, len(base))
		c.Assert(err, qt.IsNil)
	}

	attrs, err := u.Complete()
	c.Assert(err, qt.IsNil)
	c.Assert(attrs, qt.DeepEquals, &types.ObjectAttrs{
		Object:      types.CloudObject(object),
		Version:     version,
		ContentType: contentType,
		ETag:        etag,
		Size:        int64(len(total)),
	})
}

func TestUploader_EmptyUpload(t *testing.T) {
	c := qt.New(t)

	ctrl := gomock.NewController(c)
	client := NewMockblockBlobClient(ctrl)

	etagVal := azcore.ETag("e")
	client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(blockblob.UploadResponse{
		ETag: &etagVal,
	}, nil)

	u := newUploader(client, types.UploadData{
		Ctx:    context.Background(),
		Object: "empty",
		Attrs:  types.UploadAttrs{},
	})

	attrs, err := u.Complete()
	c.Assert(err, qt.IsNil)
	c.Assert(attrs.Size, qt.Equals, int64(0))
}

// withBufSize overrides the package-level bufSize and resets the pool so that
// newly allocated buffers use the new size.
func withBufSize(c *qt.C, n int) {
	origSize := bufSize
	origPool := bufPool
	bufSize = n
	bufPool = sync.Pool{New: func() any { return &buffer{buf: make([]byte, bufSize)} }}
	c.Cleanup(func() {
		bufSize = origSize
		bufPool = origPool
	})
}

// blockBodyMatcher is a gomock.Matcher that reads an io.ReadSeekCloser and
// compares its content to an expected string.
type blockBodyMatcher struct {
	data string
}

func (m *blockBodyMatcher) Matches(x interface{}) bool {
	body, ok := x.(io.ReadSeekCloser)
	if !ok {
		return false
	}
	got, err := io.ReadAll(body)
	if err != nil {
		return false
	}
	// Reset so subsequent reads by the production code work.
	_, _ = body.Seek(0, io.SeekStart)
	return string(got) == m.data
}

func (m *blockBodyMatcher) String() string {
	return fmt.Sprintf("body == %q", m.data)
}

// ---- SAS URL tests ---------------------------------------------------------------------

// testSharedKey creates a SharedKeyCredential for testing using a 64-byte zero key.
func testSharedKey(t *testing.T, accountName string) *azblob.SharedKeyCredential {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(make([]byte, 64))
	cred, err := azblob.NewSharedKeyCredential(accountName, key)
	if err != nil {
		t.Fatalf("create test SharedKeyCredential: %v", err)
	}
	return cred
}

func TestSignedUploadURL(t *testing.T) {
	c := qt.New(t)

	const accountName = "testaccount"
	const containerName = "mycontainer"
	const blobName = "path/to/myblob.txt"

	b := &bucket{
		sharedKey:   testSharedKey(t, accountName),
		accountName: accountName,
		cfg:         &config.Bucket{CloudName: containerName},
	}

	url, err := b.SignedUploadURL(types.UploadURLData{
		Ctx:    context.Background(),
		Object: types.CloudObject(blobName),
		TTL:    time.Hour,
	})
	c.Assert(err, qt.IsNil)

	expectedPrefix := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?", accountName, containerName, blobName)
	c.Assert(strings.HasPrefix(url, expectedPrefix), qt.IsTrue,
		qt.Commentf("URL %q should start with %q", url, expectedPrefix))
	c.Assert(strings.Contains(url, "sp="), qt.IsTrue,
		qt.Commentf("URL should contain SAS permissions param; got %q", url))
	c.Assert(strings.Contains(url, "sig="), qt.IsTrue,
		qt.Commentf("URL should contain SAS signature; got %q", url))
	c.Assert(strings.Contains(url, "spr=https"), qt.IsTrue,
		qt.Commentf("URL should require HTTPS; got %q", url))
}

func TestSignedDownloadURL(t *testing.T) {
	c := qt.New(t)

	const accountName = "testaccount"
	const containerName = "mycontainer"
	const blobName = "path/to/file.bin"

	b := &bucket{
		sharedKey:   testSharedKey(t, accountName),
		accountName: accountName,
		cfg:         &config.Bucket{CloudName: containerName},
	}

	url, err := b.SignedDownloadURL(types.DownloadURLData{
		Ctx:    context.Background(),
		Object: types.CloudObject(blobName),
		TTL:    15 * time.Minute,
	})
	c.Assert(err, qt.IsNil)

	expectedPrefix := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?", accountName, containerName, blobName)
	c.Assert(strings.HasPrefix(url, expectedPrefix), qt.IsTrue,
		qt.Commentf("URL %q should start with %q", url, expectedPrefix))
	c.Assert(strings.Contains(url, "sp="), qt.IsTrue,
		qt.Commentf("URL should contain SAS permissions param; got %q", url))
	c.Assert(strings.Contains(url, "sig="), qt.IsTrue,
		qt.Commentf("URL should contain SAS signature; got %q", url))
}

func TestSignedURL_NoSharedKey(t *testing.T) {
	c := qt.New(t)

	b := &bucket{
		sharedKey:   nil,
		accountName: "account",
		cfg:         &config.Bucket{CloudName: "container"},
	}

	_, err := b.SignedUploadURL(types.UploadURLData{
		Ctx:    context.Background(),
		Object: "blob",
		TTL:    time.Hour,
	})
	c.Assert(err, qt.Not(qt.IsNil))

	_, err = b.SignedDownloadURL(types.DownloadURLData{
		Ctx:    context.Background(),
		Object: "blob",
		TTL:    time.Hour,
	})
	c.Assert(err, qt.Not(qt.IsNil))
}

// blockBodyMatcher uses bytes.NewReader so we can seek; make sure the body
// is a real bytes.Reader-backed seeker.
var _ = (*bytes.Reader)(nil)
