package dockerbuild

import (
	"context"
	"io"
	"sync"

	"github.com/cockroachdb/errors"

	"encr.dev/pkg/tarstream"
)

// fileReadAhead prefetches distinct source-file entries before tar assembly.
// A fixed pool reads files in order within each slot. Each slot owns
// its buffers, so a later file cannot exhaust the memory needed by an earlier
// file and deadlock the consumer. Headers and padding are never prefetched.
type fileReadAhead struct {
	ctx          context.Context
	cancel       context.CancelFunc
	tar          *tarstream.TarVec
	done         chan struct{}
	workerErrors []error    // each worker owns one element, read after done
	readMu       sync.Mutex // TarVec is sequential; also excludes Close from Read
	closeOnce    sync.Once
	closeErr     error
}

type fileReadSlot struct {
	free  chan []byte
	ready chan fileReadChunk
}

type fileReadChunk struct {
	buf []byte
	n   int
	err error
}

// fileIndices identifies the source-file vectors, in tar order. The other
// vectors are immutable in-memory headers, contents and padding. Each opener
// gets its own pool; a source file is never split across workers.
func newFileReadAhead(ctx context.Context, vecs []tarstream.Datavec, fileIndices []int, budget, workers int) io.ReadCloser {
	if budget <= 0 || len(fileIndices) == 0 {
		return tarstream.NewTarVec(vecs)
	}
	workers = min(workers, len(fileIndices), budget)
	ctx, cancel := context.WithCancel(ctx)
	r := &fileReadAhead{ctx: ctx, cancel: cancel, done: make(chan struct{}), workerErrors: make([]error, workers)}
	wrapped := append([]tarstream.Datavec(nil), vecs...)
	var wg sync.WaitGroup
	for idx := range workers {
		// Reusable buffers include all free, queued, producer- and consumer-held
		// bytes. Completed files can remain queued while the worker reads another
		// file; four buffers per slot keep small files from stalling that pipeline.
		perWorker := budget / workers
		count := min(4, perWorker)
		slot := &fileReadSlot{free: make(chan []byte, count), ready: make(chan fileReadChunk, count)}
		for range count {
			slot.free <- make([]byte, perWorker/count)
		}
		for file := idx; file < len(fileIndices); file += workers {
			vecIdx := fileIndices[file]
			wrapped[vecIdx] = &prefetchedFileVec{ctx: ctx, slot: slot, size: vecs[vecIdx].GetSize()}
		}
		wg.Go(func() {
			for file := idx; file < len(fileIndices); file += workers {
				if ctx.Err() != nil {
					return
				}
				if err := slot.prefetch(ctx, vecs[fileIndices[file]]); err != nil {
					r.workerErrors[idx] = errors.Join(r.workerErrors[idx], err)
				}
			}
		})
	}
	r.tar = tarstream.NewTarVec(wrapped)
	go func() { wg.Wait(); close(r.done) }()
	return r
}

func (s *fileReadSlot) prefetch(ctx context.Context, vec tarstream.Datavec) (closeErr error) {
	src, err := vec.Open()
	if err != nil {
		select {
		case <-ctx.Done():
		case s.ready <- fileReadChunk{err: errors.Wrap(err, "open source file")}:
		}
		return nil
	}
	// The worker owns both ReadAt and Close, including during cancellation.
	defer func() { closeErr = src.Close() }()
	for offset := int64(0); offset < vec.GetSize(); {
		var buf []byte
		select {
		case <-ctx.Done():
			return nil
		case buf = <-s.free:
		}
		want := int(min(int64(len(buf)), vec.GetSize()-offset))
		n, err := readFileChunk(ctx, src, buf[:want], offset)
		offset += int64(n)
		if err == nil && offset == vec.GetSize() {
			err = io.EOF
		}
		select {
		case <-ctx.Done():
			return nil
		case s.ready <- fileReadChunk{buf: buf, n: n, err: err}:
		}
		if err != nil {
			return nil
		}
	}
	return nil
}

func readFileChunk(ctx context.Context, src io.ReaderAt, p []byte, offset int64) (int, error) {
	n := 0
	for n < len(p) {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		nn, err := src.ReadAt(p[n:], offset+int64(n))
		n += nn
		if err == io.EOF {
			if n < len(p) {
				err = io.ErrUnexpectedEOF
			} else {
				err = nil
			}
		}
		if err != nil {
			return n, err
		}
		if nn == 0 {
			return n, io.ErrNoProgress
		}
	}
	return n, nil
}

func (r *fileReadAhead) Read(p []byte) (int, error) {
	r.readMu.Lock()
	defer r.readMu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	// Coalesce headers, contents and padding in the caller's buffer. Feeding
	// gzip one tiny header at a time costs CPU and loses the old stream buffer's
	// batching benefit; this needs no additional buffer or filesystem reads.
	n := 0
	for n < len(p) {
		if err := r.ctx.Err(); err != nil {
			return n, err
		}
		nn, err := r.tar.Read(p[n:])
		n += nn
		if err != nil {
			return n, err
		}
		if nn == 0 {
			return n, io.ErrNoProgress
		}
	}
	return n, nil
}

func (r *fileReadAhead) Close() error {
	r.closeOnce.Do(func() {
		r.cancel()
		<-r.done
		r.readMu.Lock()
		defer r.readMu.Unlock()
		r.closeErr = errors.Join(append(r.workerErrors, r.tar.Close())...)
	})
	return r.closeErr
}

type prefetchedFileVec struct {
	ctx  context.Context
	slot *fileReadSlot
	size int64
}

func (v *prefetchedFileVec) GetSize() int64           { return v.size }
func (v *prefetchedFileVec) Clone() tarstream.Datavec { return v }
func (v *prefetchedFileVec) Open() (tarstream.DataReader, error) {
	return &prefetchedFileReader{ctx: v.ctx, slot: v.slot}, nil
}

type prefetchedFileReader struct {
	ctx      context.Context
	slot     *fileReadSlot
	current  fileReadChunk
	offset   int
	pos      int64
	terminal error
}

// This private ReaderAt is only used by the forward-only tar assembler. It
// consumes file buffers in order rather than issuing more filesystem reads.
func (r *prefetchedFileReader) ReadAt(p []byte, off int64) (int, error) {
	if off != r.pos {
		return 0, errors.Newf("non-sequential prefetched file read: offset %d, want %d", off, r.pos)
	}
	n := 0
	for n < len(p) {
		nn, err := r.read(p[n:])
		n += nn
		r.pos += int64(nn)
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (r *prefetchedFileReader) read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if r.terminal != nil {
		return 0, r.terminal
	}
	if r.current.buf == nil {
		select {
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		case r.current = <-r.slot.ready:
		}
		r.offset = 0
	}
	n := copy(p, r.current.buf[r.offset:r.current.n])
	r.offset += n
	if r.offset == r.current.n {
		r.terminal = r.current.err
		r.releaseBuffer()
	}
	return n, r.terminal
}

func (r *prefetchedFileReader) releaseBuffer() {
	if r.current.buf != nil {
		r.slot.free <- r.current.buf
	}
	r.current = fileReadChunk{}
}

func (r *prefetchedFileReader) Close() error {
	r.releaseBuffer()
	return nil
}
