package dockerbuild

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cockroachdb/errors"

	"encr.dev/pkg/tarstream"
)

func TestFileReadAheadOverlapsWithinBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stats := &fileReadStats{}
		var vecs []tarstream.Datavec
		for range 8 {
			vecs = append(vecs, &testFileVec{data: bytes.Repeat([]byte("abcdefgh"), 16), stats: stats})
		}
		r := newFileReadAhead(t.Context(), vecs, allFileIndices(vecs), 32, 4)
		synctest.Wait()
		if stats.bytes.Load() != 32 || stats.opens.Load() != 4 {
			t.Fatalf("prefetched bytes=%d, files=%d", stats.bytes.Load(), stats.opens.Load())
		}
		if _, err := r.Read(make([]byte, 1)); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if stats.bytes.Load() != 32 {
			t.Fatal("partially consumed buffer exceeded budget")
		}
		if _, err := r.Read(make([]byte, 1)); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if stats.bytes.Load() != 34 {
			t.Fatalf("prefetch did not refill the released buffer: %d", stats.bytes.Load())
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if stats.active.Load() != 0 || stats.closes.Load() != 4 {
			t.Fatal("early close leaked a file")
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if stats.closes.Load() != 4 {
			t.Fatal("closed a file twice")
		}
	})
}

func TestFileReadAheadFourDistinctFilesAndOrderedTar(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stats := &fileReadStats{}
		var vecs []tarstream.Datavec
		var files []int
		var sources []*testFileVec
		var want []byte
		gates := make([]chan struct{}, 4)
		for i := range 8 {
			data := bytes.Repeat([]byte{byte('a' + i)}, 4)
			src := &testFileVec{data: data, stats: stats}
			if i < 4 {
				gates[i] = make(chan struct{})
				src.gate = gates[i]
			}
			sources = append(sources, src)
			vecs = append(vecs, tarstream.MemVec{Data: []byte("header")})
			files = append(files, len(vecs))
			vecs = append(vecs, src, tarstream.PadVec{Size: 2})
			want = append(want, []byte("header")...)
			want = append(want, data...)
			want = append(want, 0, 0)
		}
		r := newFileReadAhead(t.Context(), vecs, files, 16, 4)
		synctest.Wait()
		if stats.reading.Load() != 4 || stats.active.Load() != 4 {
			t.Fatalf("not four concurrent disk reads: reading=%d, open=%d", stats.reading.Load(), stats.active.Load())
		}
		// Later files complete before the first one, then workers can open more
		// files while their contents remain queued. Bytes still cannot overtake
		// the earlier file or exceed the buffer budget.
		for _, i := range []int{3, 2, 1} {
			close(gates[i])
			synctest.Wait()
		}
		if stats.bytes.Load() != 12 || stats.peak.Load() != 4 {
			t.Fatal("exceeded the buffer or open-file bound")
		}
		close(gates[0])
		synctest.Wait()
		if stats.bytes.Load() != 16 || stats.peak.Load() != 4 {
			t.Fatal("exceeded the buffer or open-file bound")
		}
		got, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("ordered tar contents changed: %q, %v", got, err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if stats.peak.Load() != 4 || stats.active.Load() != 0 || stats.closes.Load() != 8 {
			t.Fatalf("file lifecycle: peak=%d active=%d closes=%d", stats.peak.Load(), stats.active.Load(), stats.closes.Load())
		}
		for _, src := range sources {
			if src.opens.Load() != 1 {
				t.Fatal("split a source file across readers or reopened it")
			}
		}
	})
}

func TestFileReadAheadLargeFileWithSmallFollowers(t *testing.T) {
	for _, workers := range []int{1, 2, 4} {
		for _, budget := range []int{1, 3, 16, 100} {
			stats := &fileReadStats{}
			var vecs []tarstream.Datavec
			var want []byte
			for i, size := range []int{1001, 1, 19, 5, 2001, 3, 0} {
				data := bytes.Repeat([]byte{byte('a' + i)}, size)
				vecs = append(vecs, &testFileVec{data: data, stats: stats})
				want = append(want, data...)
			}
			r := newFileReadAhead(t.Context(), vecs, allFileIndices(vecs), budget, workers)
			if n, err := r.Read(nil); n != 0 || err != nil {
				t.Fatalf("empty read: %d, %v", n, err)
			}
			got, err := io.ReadAll(r)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("workers=%d budget=%d: got %d bytes, %v", workers, budget, len(got), err)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			if stats.active.Load() != 0 || stats.peak.Load() > int64(min(workers, budget)) {
				t.Fatal("exceeded open file bound or leaked files")
			}
		}
	}
}

func TestFileReadAheadSourceErrors(t *testing.T) {
	sentinel := errors.New("source failed")
	for _, tc := range []struct {
		name   string
		source *testFileVec
		want   string
		err    error
	}{
		{"open", &testFileVec{data: []byte("abcdefgh"), openErr: sentinel}, "", sentinel},
		{"truncated", &testFileVec{data: []byte("abc"), recordedSize: 8}, "abc", io.ErrUnexpectedEOF},
		{"grown", &testFileVec{data: []byte("abcdefgh"), recordedSize: 4}, "abcd", nil},
		{"full-chunk-error", &testFileVec{data: []byte("abcdefgh"), readErr: sentinel}, "ab", sentinel},
		{"no-progress", &testFileVec{data: []byte("abcdefgh"), noProgress: true}, "", io.ErrNoProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stats := &fileReadStats{}
			tc.source.stats = stats
			r := newFileReadAhead(t.Context(), []tarstream.Datavec{tc.source}, []int{0}, 8, 4)
			got, err := io.ReadAll(r)
			if string(got) != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("got %q / %v, want %q / %v", got, err, tc.want, tc.err)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			if stats.active.Load() != 0 || stats.opens.Load() != stats.closes.Load() {
				t.Fatal("failed read leaked a file")
			}
		})
	}
}

func TestFileReadAheadReportsIntermediateCloseErrors(t *testing.T) {
	sentinel := errors.New("close failed")
	stats := &fileReadStats{}
	var vecs []tarstream.Datavec
	for i := range 9 {
		src := &testFileVec{data: []byte("abcdefgh"), stats: stats}
		if i == 0 {
			src.closeErr = sentinel
		} // Not the last file of its worker.
		vecs = append(vecs, src)
	}
	r := newFileReadAhead(t.Context(), vecs, allFileIndices(vecs), 16, 4)
	if _, err := io.ReadAll(r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); !errors.Is(err, sentinel) {
		t.Fatalf("Close lost error: %v", err)
	}
	if stats.opens.Load() != 9 || stats.closes.Load() != 9 {
		t.Fatal("did not close every file")
	}
}

func TestFileReadAheadCancellationDuringReads(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stats := &fileReadStats{}
		gate := make(chan struct{})
		var vecs []tarstream.Datavec
		for range 8 {
			vecs = append(vecs, &testFileVec{data: []byte("abcdefgh"), gate: gate, stats: stats})
		}
		r := newFileReadAhead(t.Context(), vecs, allFileIndices(vecs), 16, 4)
		readDone := make(chan error, 1)
		go func() { _, err := io.ReadAll(r); readDone <- err }()
		synctest.Wait()
		if stats.reading.Load() != 4 {
			t.Fatal("four file reads did not start")
		}
		closed := make(chan error, 1)
		go func() { closed <- r.Close() }()
		synctest.Wait()
		if err := <-readDone; !errors.Is(err, context.Canceled) {
			t.Fatalf("Read did not wake on cancellation: %v", err)
		}
		if stats.closes.Load() != 0 {
			t.Fatal("closed a file while ReadAt was in flight")
		}
		select {
		case <-closed:
			t.Fatal("Close did not join the filesystem reads")
		default:
		}
		close(gate)
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
		if stats.opens.Load() != 4 || stats.closes.Load() != 4 || stats.active.Load() != 0 {
			t.Fatal("cancellation opened another file or leaked one")
		}
	})
}

func TestFileReadAheadLaterErrorWaitsForEarlierFile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stats := &fileReadStats{}
		gate := make(chan struct{})
		sentinel := errors.New("later file failed")
		vecs := []tarstream.Datavec{&testFileVec{data: []byte("first"), stats: stats, gate: gate}, &testFileVec{data: []byte("second"), stats: stats, openErr: sentinel}}
		r := newFileReadAhead(t.Context(), vecs, []int{0, 1}, 32, 4)
		type result struct {
			data []byte
			err  error
		}
		done := make(chan result, 1)
		go func() { data, err := io.ReadAll(r); done <- result{data, err} }()
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("later error overtook earlier file")
		default:
		}
		close(gate)
		got := <-done
		if string(got.data) != "first" || !errors.Is(got.err, sentinel) {
			t.Fatalf("got %q, %v", got.data, got.err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func allFileIndices(vecs []tarstream.Datavec) []int {
	var indices []int
	for i, vec := range vecs {
		if vec.GetSize() > 0 {
			indices = append(indices, i)
		}
	}
	return indices
}

type fileReadStats struct{ opens, closes, active, peak, bytes, reading atomic.Int64 }
type testFileVec struct {
	data                       []byte
	recordedSize               int64
	stats                      *fileReadStats
	readLatency                time.Duration
	gate                       <-chan struct{}
	openErr, readErr, closeErr error
	noProgress                 bool
	opens                      atomic.Int64
}

func (v *testFileVec) GetSize() int64 {
	if v.recordedSize > 0 {
		return v.recordedSize
	}
	return int64(len(v.data))
}
func (v *testFileVec) Clone() tarstream.Datavec { return v }
func (v *testFileVec) Open() (tarstream.DataReader, error) {
	if v.openErr != nil {
		return nil, v.openErr
	}
	v.opens.Add(1)
	v.stats.opens.Add(1)
	n := v.stats.active.Add(1)
	for old := v.stats.peak.Load(); n > old && !v.stats.peak.CompareAndSwap(old, n); old = v.stats.peak.Load() {
	}
	return &testFileReader{vec: v, reader: bytes.NewReader(v.data)}, nil
}

type testFileReader struct {
	vec    *testFileVec
	reader *bytes.Reader
}

func (r *testFileReader) ReadAt(p []byte, off int64) (int, error) {
	r.vec.stats.reading.Add(1)
	defer r.vec.stats.reading.Add(-1)
	if r.vec.readLatency > 0 {
		time.Sleep(r.vec.readLatency)
	}
	if r.vec.gate != nil {
		<-r.vec.gate
	}
	if r.vec.noProgress {
		return 0, nil
	}
	n, err := r.reader.ReadAt(p, off)
	r.vec.stats.bytes.Add(int64(n))
	if r.vec.readErr != nil {
		err = r.vec.readErr
	}
	return n, err
}
func (r *testFileReader) Close() error {
	r.vec.stats.closes.Add(1)
	r.vec.stats.active.Add(-1)
	return r.vec.closeErr
}

// A deterministic latency model, not a measurement of SSD speed: each file fits
// one read and that read waits 1 ms. Four workers must overlap those waits while
// preserving identical output. The test uses virtual time, without a VM or sleep
// in wall-clock time.
func TestFileReadAheadOverlapsReadLatency(t *testing.T) {
	for _, workers := range []int{1, 4} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				stats := &fileReadStats{}
				var vecs []tarstream.Datavec
				var want []byte
				for i := range 16 {
					data := bytes.Repeat([]byte{byte(i)}, 1024)
					vecs = append(vecs, &testFileVec{data: data, stats: stats, readLatency: time.Millisecond})
					want = append(want, data...)
				}
				start := time.Now()
				r := newFileReadAhead(t.Context(), vecs, allFileIndices(vecs), 64<<10, workers)
				got, err := io.ReadAll(r)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("read: %v", err)
				}
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
				if elapsed := time.Since(start); elapsed != time.Duration(16/workers)*time.Millisecond {
					t.Fatalf("workers %d: read waits did not overlap: %s", workers, elapsed)
				}
			})
		})
	}
}
