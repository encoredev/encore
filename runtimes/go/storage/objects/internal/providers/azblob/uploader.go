//go:build !encore_no_azure

package azblob

import (
"bytes"
"context"
"encoding/base64"
"encoding/binary"
"errors"
"io"
"sync"

"github.com/Azure/azure-sdk-for-go/sdk/azcore"
"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
"golang.org/x/sync/errgroup"

"encore.dev/storage/objects/internal/types"
)

// uploader implements types.Uploader for Azure Block Blobs.
//
// Small uploads (data fits in the first 10 MiB buffer) use a single
// blockblob.Upload call. Larger uploads use staged blocks
// (StageBlock x N -> CommitBlockList), which mirrors the S3 multipart pattern.
type uploader struct {
client blockBlobClient
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
n   int
}

func newUploader(client blockBlobClient, data types.UploadData) *uploader {
return &uploader{
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
p = p[copied:]
select {
case u.out <- uploadEvent{data: curr}:
case <-u.done:
return n, u.err
}
u.curr, curr = nil, nil
} else {
u.curr = curr
return n, nil
}
}
return n, nil
}

func (u *uploader) Complete() (*types.ObjectAttrs, error) {
u.initUpload()
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
<-u.done
return u.attrs, u.err
}

func (u *uploader) Abort(err error) {
u.initUpload()
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
return nil, ev.abort
} else if ev.done {
// All data fits in the first buffer (or there is no data): single-part upload.
var buf []byte
if ev.data != nil {
buf = ev.data.buf[:ev.data.n]
}
return u.singlePartUpload(buf)
}
return u.multiPartUpload(ev.data)
}

// blockBlobClient is the subset of blockblob.Client used by the uploader.
// The interface enables unit testing without a real Azure endpoint.
type blockBlobClient interface {
Upload(ctx context.Context, body io.ReadSeekCloser, options *blockblob.UploadOptions) (blockblob.UploadResponse, error)
StageBlock(ctx context.Context, base64BlockID string, body io.ReadSeekCloser, options *blockblob.StageBlockOptions) (blockblob.StageBlockResponse, error)
CommitBlockList(ctx context.Context, base64BlockIDs []string, options *blockblob.CommitBlockListOptions) (blockblob.CommitBlockListResponse, error)
}

func (u *uploader) singlePartUpload(buf []byte) (*types.ObjectAttrs, error) {
opts := &blockblob.UploadOptions{}
if u.data.Pre.NotExists {
etagAny := azcore.ETagAny
opts.AccessConditions = &blob.AccessConditions{
ModifiedAccessConditions: &blob.ModifiedAccessConditions{
IfNoneMatch: &etagAny,
},
}
}
if ct := u.data.Attrs.ContentType; ct != "" {
opts.HTTPHeaders = &blob.HTTPHeaders{BlobContentType: ptr(ct)}
}

resp, err := u.client.Upload(u.ctx, streaming.NopCloser(bytes.NewReader(buf)), opts)
if err != nil {
return nil, err
}
return &types.ObjectAttrs{
Object:      u.data.Object,
Version:     valOrZero(resp.VersionID),
ContentType: u.data.Attrs.ContentType,
Size:        int64(len(buf)),
ETag:        string(valOrZero(resp.ETag)),
}, nil
}

func (u *uploader) multiPartUpload(initial *buffer) (attrs *types.ObjectAttrs, err error) {
g, groupCtx := errgroup.WithContext(u.ctx)
var (
blockIDs  []string
totalSize int64
part      int32
)

// stageBlock is called sequentially from the event loop below; the errgroup
// goroutines only perform the network upload, so blockIDs slice ordering is safe.
stageBlock := func(buf *buffer) {
if buf == nil {
return
}
totalSize += int64(buf.n)
blockID := blockIDForPart(part)
part++
blockIDs = append(blockIDs, blockID)

g.Go(func() error {
data := buf.buf[:buf.n]
defer putBuf(buf)
_, stageErr := u.client.StageBlock(groupCtx, blockID, streaming.NopCloser(bytes.NewReader(data)), nil)
return stageErr
})
}

stageBlock(initial)
for {
ev := <-u.out
if ev.abort != nil {
// Uncommitted blocks in Azure expire automatically; no explicit abort needed.
_ = g.Wait()
return nil, ev.abort
}
if ev.data != nil {
stageBlock(ev.data)
}
if ev.done {
break
}
}

if err = g.Wait(); err != nil {
return nil, err
}

commitOpts := &blockblob.CommitBlockListOptions{}
if u.data.Pre.NotExists {
etagAny := azcore.ETagAny
commitOpts.AccessConditions = &blob.AccessConditions{
ModifiedAccessConditions: &blob.ModifiedAccessConditions{
IfNoneMatch: &etagAny,
},
}
}
if ct := u.data.Attrs.ContentType; ct != "" {
commitOpts.HTTPHeaders = &blob.HTTPHeaders{BlobContentType: ptr(ct)}
}

commitResp, err := u.client.CommitBlockList(u.ctx, blockIDs, commitOpts)
if err != nil {
return nil, err
}
return &types.ObjectAttrs{
Object:      u.data.Object,
Version:     valOrZero(commitResp.VersionID),
ContentType: u.data.Attrs.ContentType,
Size:        totalSize,
ETag:        string(valOrZero(commitResp.ETag)),
}, nil
}

// blockIDForPart returns a fixed-length base64-encoded block ID for the given
// part index. Azure requires all block IDs for a blob to have the same byte
// length before base64 encoding; we use a 4-byte big-endian representation.
func blockIDForPart(n int32) string {
b := make([]byte, 4)
binary.BigEndian.PutUint32(b, uint32(n))
return base64.StdEncoding.EncodeToString(b)
}

// bufSize is the target buffer size for each upload part.
// Variable for testing. Azure supports blocks up to 100 MiB; 10 MiB matches
// the S3 provider default.
var bufSize = 10 * 1024 * 1024

var bufPool = sync.Pool{
New: func() any {
return &buffer{buf: make([]byte, bufSize)}
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
