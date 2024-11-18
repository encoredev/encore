package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"encore.dev/storage/objects/internal/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	qt "github.com/frankban/quicktest"
	"github.com/golang/mock/gomock"
)

//go:generate mockgen -source=./uploader.go -destination ./mock_client_test.go -package s3 s3Client

func TestUploader_Sync(t *testing.T) {
	c := qt.New(t)

	ctrl := gomock.NewController(c)
	client := NewMocks3Client(ctrl)

	const (
		bucket      = "bucket"
		object      = "object"
		contentType = "text/plain"
	)
	u := newUploader(client, bucket, types.UploadData{
		Ctx:    context.Background(),
		Object: object,
		Attrs: types.UploadAttrs{
			ContentType: contentType,
		},
		Pre: types.Preconditions{},
	})

	const (
		version = "version"
		etag    = "etag"
	)
	client.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{
		VersionId: ptr(version),
		ETag:      ptr(etag),
	}, nil)

	content := []byte("test")
	n, err := u.Write(content)
	c.Assert(n, qt.Equals, 4)
	c.Assert(err, qt.Equals, nil)

	attrs, err := u.Complete()
	c.Assert(err, qt.Equals, nil)
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
	client := NewMocks3Client(ctrl)

	const (
		bucket      = "bucket"
		object      = "object"
		contentType = "text/plain"
	)
	u := newUploader(client, bucket, types.UploadData{
		Ctx:    context.Background(),
		Object: object,
		Attrs: types.UploadAttrs{
			ContentType: contentType,
		},
		Pre: types.Preconditions{},
	})

	const (
		version = "version"
		etag    = "etag"
	)
	client.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{
		VersionId: ptr(version),
		ETag:      ptr(etag),
	}, nil)

	baseContent := "test"
	totalContent := strings.Repeat(baseContent, 10)
	for i := 0; i < 10; i++ {
		n, err := u.Write([]byte(baseContent))
		c.Assert(n, qt.Equals, len(baseContent))
		c.Assert(err, qt.Equals, nil)
	}

	attrs, err := u.Complete()
	c.Assert(err, qt.Equals, nil)
	c.Assert(attrs, qt.DeepEquals, &types.ObjectAttrs{
		Object:      types.CloudObject(object),
		Version:     version,
		ContentType: contentType,
		ETag:        etag,
		Size:        int64(len(totalContent)),
	})
}

func TestUploader_MultipartUpload(t *testing.T) {
	c := qt.New(t)

	ctrl := gomock.NewController(c)
	client := NewMocks3Client(ctrl)

	const (
		bucket      = "bucket"
		object      = "object"
		contentType = "text/plain"
	)
	u := newUploader(client, bucket, types.UploadData{
		Ctx:    context.Background(),
		Object: object,
		Attrs: types.UploadAttrs{
			ContentType: contentType,
		},
		Pre: types.Preconditions{},
	})

	withBufSize(c, 10)
	const (
		version  = "version"
		etag     = "etag"
		uploadID = "uploadID"
	)
	client.EXPECT().CreateMultipartUpload(gomock.Any(), gomock.Any()).Return(&s3.CreateMultipartUploadOutput{
		UploadId: ptr(uploadID),
	}, nil)
	client.EXPECT().UploadPart(gomock.Any(), &partMatcher{num: 1, data: "abcdefghij"}).Return(&s3.UploadPartOutput{}, nil)
	client.EXPECT().UploadPart(gomock.Any(), &partMatcher{num: 2, data: "klmabcdefg"}).Return(&s3.UploadPartOutput{}, nil)
	client.EXPECT().UploadPart(gomock.Any(), &partMatcher{num: 3, data: "hijklmabcd"}).Return(&s3.UploadPartOutput{}, nil)
	client.EXPECT().UploadPart(gomock.Any(), &partMatcher{num: 4, data: "efghijklm"}).Return(&s3.UploadPartOutput{}, nil)
	client.EXPECT().CompleteMultipartUpload(gomock.Any(), gomock.Any()).Return(&s3.CompleteMultipartUploadOutput{
		VersionId: ptr(version),
		ETag:      ptr(etag),
	}, nil)

	baseContent := "abcdefghijklm"
	totalContent := strings.Repeat(baseContent, 3)
	for i := 0; i < 3; i++ {
		n, err := u.Write([]byte(baseContent))
		c.Assert(n, qt.Equals, len(baseContent))
		c.Assert(err, qt.Equals, nil)
	}

	attrs, err := u.Complete()
	c.Assert(err, qt.Equals, nil)
	c.Assert(attrs, qt.DeepEquals, &types.ObjectAttrs{
		Object:      types.CloudObject(object),
		Version:     version,
		ContentType: contentType,
		ETag:        etag,
		Size:        int64(len(totalContent)),
	})
}

func withBufSize(c *qt.C, n int) {
	orig := bufSize
	bufSize = n
	c.Cleanup(func() { bufSize = orig })
}

type partMatcher struct {
	num  int
	data string
}

func (m *partMatcher) Matches(x interface{}) bool {
	part, ok := x.(*s3.UploadPartInput)
	if !ok {
		return false
	}
	data, err := io.ReadAll(part.Body)
	if err != nil {
		panic(err)
	}
	part.Body = bytes.NewReader(data)
	fmt.Printf("part %d: %q\n", valOrZero(part.PartNumber), data)
	return valOrZero(part.PartNumber) == int32(m.num) && string(data) == m.data
}

func (m *partMatcher) String() string {
	return fmt.Sprintf("is part %d with data %q", m.num, m.data)
}
