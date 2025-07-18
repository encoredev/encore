package tarstream

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/pkg/errors"
)

func NewTarVec(vecs []Datavec) *TarVec {
	var totalSize int64
	starts := make([]int64, len(vecs))
	ends := make([]int64, len(vecs))
	for i, dv := range vecs {
		size := dv.GetSize()
		starts[i] = totalSize
		ends[i] = totalSize + size
		totalSize += size
	}

	return &TarVec{
		vecs:   vecs,
		starts: starts,
		ends:   ends,
		size:   totalSize,
	}
}

// TarVec is an array of datavecs representing a tarball
type TarVec struct {
	vecs   []Datavec
	starts []int64 // starting offset for each vec, inclusive
	ends   []int64 // ending offset for each vec, exclusive
	size   int64

	// Current reading pos
	pos  int64
	curr *currReader
}

type currReader struct {
	idx  int // datavec index
	size int64
	data DataReader
}

// Size gets the size of the tarball represented by the tarvec
func (tv *TarVec) Size() int64 {
	return tv.size
}

func (tv *TarVec) Clone() *TarVec {
	return &TarVec{
		vecs:   tv.vecs,
		starts: tv.starts,
		ends:   tv.ends,
		pos:    tv.pos,
	}
}

func (tv *TarVec) getReader() (*currReader, error) {
	// Do we have a current reader?
	if tv.curr != nil {
		// Is the current position within the current datavec?
		if tv.pos >= tv.starts[tv.curr.idx] && tv.pos < tv.ends[tv.curr.idx] {
			return tv.curr, nil
		}
		_ = tv.curr.data.Close()
		tv.curr = nil
	}

	// Find the datavec that contains the current position
	candidate := sort.Search(len(tv.ends), func(i int) bool {
		return tv.ends[i] > tv.pos
	})
	if candidate == len(tv.ends) {
		// Position exceeds the end of the last data vec.
		return nil, io.EOF
	}

	vec := tv.vecs[candidate]
	data, err := vec.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening data vec: %v", err)
	}

	tv.curr = &currReader{
		idx:  candidate,
		size: vec.GetSize(),
		data: data,
	}
	return tv.curr, nil
}

// Read the data represented by the tarvec
func (tv *TarVec) Read(b []byte) (int, error) {
	cr, err := tv.getReader()
	if err != nil {
		return 0, err
	}

	off := tv.pos - tv.starts[cr.idx]

	// Sanity checks
	remaining := cr.size - off
	hasMoreVecs := cr.idx+1 < len(tv.vecs)
	if remaining < 0 {
		panic("TarVec: negative remaining size")
	} else if remaining == 0 && hasMoreVecs && cr.size > 0 {
		panic("TarVec: zero remaining size but more vecs exist")
	}

	n, err := cr.data.ReadAt(b, off)
	if err == io.EOF {
		// Ignore EOF from individual readers.
		// getReader reports EOF when running out of readers.
		err = nil
	}

	if n == 0 && len(b) > 0 && (err == nil || err == io.EOF) {
		panic("TarVec: empty read from vec, more data remaining")
	}
	tv.pos += int64(n)

	return n, err
}

// Seek the virtual offset of the tarvec
func (tv *TarVec) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return 0, os.ErrInvalid
		}
		tv.pos = offset
		return tv.pos, nil
	case io.SeekCurrent:
		if tv.pos+offset < 0 {
			return 0, os.ErrInvalid
		}
		tv.pos += offset
		return tv.pos, nil
	case io.SeekEnd:
		if tv.size+offset < 0 {
			return 0, os.ErrInvalid
		}
		tv.pos = tv.size + offset
		return tv.pos, nil
	}
	return 0, os.ErrInvalid
}

func (tv *TarVec) Close() error {
	if tv.curr != nil {
		err := tv.curr.data.Close()
		tv.curr = nil
		return err
	}

	return nil
}

// Validate gets and validates the next header within the tarfile
func Validate(r io.Reader) (*tar.Header, error) {
	tr := tar.NewReader(r)
	// Next will find the next header and read it in
	// this skips all meta-headers like long names, etc
	// hopefully thats what we want here
	hdr, err := tr.Next()
	if err != nil {
		return &tar.Header{}, errors.Wrap(err, fmt.Sprintf("read header"))
	}
	return hdr, nil
}
