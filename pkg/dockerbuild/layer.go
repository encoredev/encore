package dockerbuild

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/errors"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"encr.dev/pkg/fns"
)

// Image owns the temporary compressed blobs for its newly built layers.
// Call Close after all exports, uploads and layer readers have finished.
// Base image layers retain their original storage and are not owned by Image.
type Image struct {
	v1.Image
	dir string

	mu      sync.Mutex
	closed  bool
	readers map[*os.File]struct{}
}

func (i *Image) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.closed = true
	var errs []error
	// Some image exporters do not close Compressed readers on early failure.
	// The image owns those handles too, including on platforms where an open
	// file would prevent removal of the temporary directory.
	for f := range i.readers {
		errs = append(errs, f.Close())
		delete(i.readers, f)
	}
	errs = append(errs, os.RemoveAll(i.dir))
	return errors.Join(errs...)
}

func (i *Image) openBlob(path string) (io.ReadCloser, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closed {
		return nil, fs.ErrClosed
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if i.readers == nil {
		i.readers = make(map[*os.File]struct{})
	}
	i.readers[f] = struct{}{}
	return &blobReader{file: f, owner: i}, nil
}

type blobReader struct {
	file  *os.File
	owner *Image
	done  atomic.Bool
}

func (r *blobReader) Read(p []byte) (int, error) {
	if r.done.Load() {
		return 0, io.EOF
	}
	n, err := r.file.Read(p)
	if err == io.EOF {
		fns.CloseIgnore(r)
	}
	return n, err
}

func (r *blobReader) Close() error {
	if r.done.Swap(true) {
		return nil
	}
	r.owner.mu.Lock()
	defer r.owner.mu.Unlock()
	if _, open := r.owner.readers[r.file]; !open {
		return nil
	}
	delete(r.owner.readers, r.file)
	return r.file.Close()
}

// fileLayer records both hashes while reading the source tar exactly once.
// Only compressed bytes are retained, on disk rather than in a large buffer.
type fileLayer struct {
	owner          *Image
	path           string
	digest, diffID v1.Hash
	size           int64
}

var _ v1.Layer = (*fileLayer)(nil)

func (l *fileLayer) Digest() (v1.Hash, error)            { return l.digest, nil }
func (l *fileLayer) DiffID() (v1.Hash, error)            { return l.diffID, nil }
func (l *fileLayer) Size() (int64, error)                { return l.size, nil }
func (l *fileLayer) MediaType() (types.MediaType, error) { return types.DockerLayer, nil }
func (l *fileLayer) Compressed() (io.ReadCloser, error)  { return l.owner.openBlob(l.path) }

func (l *fileLayer) Uncompressed() (io.ReadCloser, error) {
	f, err := l.Compressed()
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(f)
	if err != nil {
		fns.CloseIgnore(f)
		return nil, err
	}
	return &gzipFileReader{Reader: z, file: f}, nil
}

type gzipFileReader struct {
	*gzip.Reader
	file io.ReadCloser
}

func (r *gzipFileReader) Close() error {
	return errors.Join(r.Reader.Close(), r.file.Close())
}

// The image owns the directory and removes it on failure as well as after use.
func compressLayer(ctx context.Context, owner *Image, opener tarball.Opener, level int) (*fileLayer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	src, err := opener()
	if err != nil {
		return nil, errors.Wrap(err, "open layer tar")
	}
	defer fns.CloseIgnore(src)
	f, err := os.CreateTemp(owner.dir, "layer-*.gz")
	if err != nil {
		return nil, errors.Wrap(err, "create layer blob")
	}
	defer fns.CloseIgnore(f)

	compressedHash, tarHash := sha256.New(), sha256.New()
	// Coalesce gzip's small writes without buffering the whole layer.
	out := bufio.NewWriterSize(io.MultiWriter(f, compressedHash), 128<<10)
	z, err := gzip.NewWriterLevel(out, level)
	if err != nil {
		return nil, err
	}
	defer fns.CloseIgnore(z)
	if _, err := io.Copy(z, io.TeeReader(layerContextReader{ctx, src}, tarHash)); err != nil {
		return nil, errors.Wrap(err, "compress layer")
	}
	if err := z.Close(); err != nil {
		return nil, errors.Wrap(err, "finish layer gzip")
	}
	if err := out.Flush(); err != nil {
		return nil, errors.Wrap(err, "flush layer blob")
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "stat layer blob")
	}
	if err := f.Close(); err != nil {
		return nil, errors.Wrap(err, "close layer blob")
	}
	if err := src.Close(); err != nil {
		return nil, errors.Wrap(err, "close layer tar")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &fileLayer{
		owner: owner,
		path:  f.Name(), size: stat.Size(),
		digest: v1.Hash{Algorithm: "sha256", Hex: hex.EncodeToString(compressedHash.Sum(nil))},
		diffID: v1.Hash{Algorithm: "sha256", Hex: hex.EncodeToString(tarHash.Sum(nil))},
	}, nil
}

type layerContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r layerContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
