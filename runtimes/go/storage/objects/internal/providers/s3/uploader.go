package s3

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"encore.dev/storage/objects/internal/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/sync/errgroup"
)

type uploader struct {
	client s3Client
	bucket string
	data   types.UploadData
	ctx    context.Context
	out    chan uploadEvent

	init  sync.Once
	done  chan struct{}
	attrs *types.ObjectAttrs
	err   error

	curr *buffer
}

type uploadEvent struct {
	data  *buffer
	abort error
	done  bool
}

type buffer struct {
	buf []byte
	n   int // number of bytes in buf
}

func newUploader(client s3Client, bucket string, data types.UploadData) *uploader {
	return &uploader{
		bucket: bucket,
		client: client,
		ctx:    data.Ctx,
		data:   data,
		out:    make(chan uploadEvent, 10),
		done:   make(chan struct{}),
	}
}

func (u *uploader) Write(p []byte) (n int, err error) {
	u.initUpload()
	for len(p) > 0 {
		curr := u.curr
		if curr == nil {
			curr = getBuf()
		}

		copied := copy(curr.buf[curr.n:], p)
		n += copied
		curr.n += copied

		if copied < len(p) {
			// Buffer is full, send it.
			p = p[copied:]
			select {
			case u.out <- uploadEvent{data: curr}:
			case <-u.done:
				return n, u.err
			}

			u.curr, curr = nil, nil
		} else {
			// We've written all the data. Keep track
			// of the buffer for the next call.
			u.curr = curr
			return n, nil
		}
	}

	return n, nil
}

func (u *uploader) Complete() (*types.ObjectAttrs, error) {
	u.initUpload()
	// If we have a current buffer, send it.
	if curr := u.curr; curr != nil && curr.n > 0 {
		select {
		case u.out <- uploadEvent{data: curr, done: true}:
		case <-u.done:
		}
		u.curr = nil
	} else {
		select {
		case u.out <- uploadEvent{done: true}:
		case <-u.done:
		}
	}

	// Wait for the upload to finish.
	<-u.done
	return u.attrs, u.err
}

func (u *uploader) Abort(err error) {
	u.initUpload()
	// Ensure err is non-nil
	if err == nil {
		err = errors.New("upload aborted")
	}

	select {
	case u.out <- uploadEvent{abort: err}:
	case <-u.done:
	}
}

func (u *uploader) initUpload() {
	u.init.Do(func() {
		go func() {
			defer close(u.done)
			attrs, err := u.doUpload()
			u.attrs, u.err = attrs, mapErr(err)
		}()
	})
}

func (u *uploader) doUpload() (*types.ObjectAttrs, error) {
	ev := <-u.out
	if ev.abort != nil {
		// Nothing to do.
		return nil, ev.abort
	} else if ev.done {
		// First buffer is the final one; we can do a single-part upload.
		var buf []byte
		if ev.data != nil {
			buf = ev.data.buf[:ev.data.n]
		}
		return u.singlePartUpload(buf)
	}

	return u.multiPartUpload(ev.data)
}

type s3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

func (u *uploader) singlePartUpload(buf []byte) (*types.ObjectAttrs, error) {
	key := ptr(u.data.Object.String())
	md5sum := md5.Sum(buf)
	contentMD5 := base64.StdEncoding.EncodeToString(md5sum[:])

	var ifNoneMatch *string
	if u.data.Pre.NotExists {
		ifNoneMatch = ptr("*")
	}

	resp, err := u.client.PutObject(u.ctx, &s3.PutObjectInput{
		Bucket:        &u.bucket,
		Key:           key,
		Body:          bytes.NewReader(buf),
		ContentType:   ptrOrNil(u.data.Attrs.ContentType),
		ContentMD5:    &contentMD5,
		ContentLength: ptr(int64(len(buf))),
		IfNoneMatch:   ifNoneMatch,
	})
	if err != nil {
		return nil, err
	}

	return &types.ObjectAttrs{
		Object:      u.data.Object,
		Version:     valOrZero(resp.VersionId),
		ContentType: u.data.Attrs.ContentType,
		Size:        int64(len(buf)),
		ETag:        valOrZero(resp.ETag),
	}, nil
}

func (u *uploader) multiPartUpload(initial *buffer) (attrs *types.ObjectAttrs, err error) {
	key := ptr(u.data.Object.String())
	resp, err := u.client.CreateMultipartUpload(u.ctx, &s3.CreateMultipartUploadInput{
		Bucket:      &u.bucket,
		Key:         key,
		ContentType: ptrOrNil(u.data.Attrs.ContentType),
	})
	if err != nil {
		return nil, err
	}
	uploadID := valOrZero(resp.UploadId)

	defer func() {
		if err != nil {
			// The upload failed. Abort the multipart upload.
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_, _ = u.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
					Bucket:   &u.bucket,
					Key:      key,
					UploadId: &uploadID,
				})
			}()
		}
	}()

	g, groupCtx := errgroup.WithContext(u.ctx)
	partNumber := int32(1)
	var totalSize int64
	uploadPart := func(buf *buffer) {
		if buf == nil {
			// No data to upload.
			return
		}

		totalSize += int64(buf.n)
		part := partNumber
		partNumber++
		g.Go(func() error {
			data := buf.buf[:buf.n]
			defer putBuf(buf)

			md5sum := md5.Sum(data)
			contentMD5 := base64.StdEncoding.EncodeToString(md5sum[:])
			_, err := u.client.UploadPart(groupCtx, &s3.UploadPartInput{
				Bucket:        &u.bucket,
				UploadId:      &uploadID,
				PartNumber:    &part,
				Body:          bytes.NewReader(data),
				ContentLength: ptr(int64(len(data))),
				ContentMD5:    ptr(contentMD5),
			})
			return err
		})
	}

	// Upload the first part, if given.
	uploadPart(initial)
	for {
		ev := <-u.out
		if ev.abort != nil {
			return nil, ev.abort
		}

		if ev.data != nil {
			uploadPart(ev.data)
		}

		if ev.done {
			break
		}
	}

	// Wait for the uploads to complete.
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Complete the multipart upload.
	var ifNoneMatch *string
	if u.data.Pre.NotExists {
		ifNoneMatch = ptr("*")
	}

	var completeResp *s3.CompleteMultipartUploadOutput
	completeResp, err = u.client.CompleteMultipartUpload(u.ctx, &s3.CompleteMultipartUploadInput{
		Bucket:      &u.bucket,
		Key:         key,
		UploadId:    &uploadID,
		IfNoneMatch: ifNoneMatch,
	})
	if err != nil {
		return nil, err
	}
	return &types.ObjectAttrs{
		Object:      u.data.Object,
		Version:     valOrZero(completeResp.VersionId),
		ContentType: u.data.Attrs.ContentType,
		Size:        totalSize,
		ETag:        valOrZero(completeResp.ETag),
	}, nil
}

// bufSize is the size of buffers allocated by bufPool.
// It's a variable for testing purposes.
var bufSize = 10 * 1024 * 1024

var bufPool = sync.Pool{
	New: func() any {
		return &buffer{
			buf: make([]byte, bufSize),
		}
	},
}

func getBuf() *buffer {
	buf := bufPool.Get().(*buffer)
	buf.n = 0
	return buf
}

func putBuf(buf *buffer) {
	bufPool.Put(buf)
}
