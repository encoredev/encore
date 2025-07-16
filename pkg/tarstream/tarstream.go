package tarstream

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
)

// TarVec is an array of datavecs representing a tarball
type TarVec struct {
	Dvecs []Datavec
	Pos   int64
	Size  int64
}

// PositionInfo stores information on where files get placed in the tarball
type PositionInfo struct {
	Name   string
	Offset int64
	Size   int64
}

// GetSize gets the size of the tarball represented by the tarvec
func (tv TarVec) GetSize() int64 {
	return tv.Size
}

// set the size field in the tarvec to represent
// what the tarball size will be
func (tv *TarVec) ComputeSize() {
	tv.Size = 0
	for _, dv := range tv.Dvecs {
		tv.Size += dv.GetSize()
	}
}

func (tv *TarVec) Clone() *TarVec {
	vecs := make([]Datavec, len(tv.Dvecs))
	for i, dv := range tv.Dvecs {
		vecs[i] = dv.Clone()
	}

	return &TarVec{
		Dvecs: vecs,
		Pos:   tv.Pos,
		Size:  tv.Size,
	}
}

// Read the data represented by the tarvec
func (tv *TarVec) Read(b []byte) (int, error) {
	off := int64(0)
	// loop through the datavecs to find our current position
	// then start reading when we find it
	// we will fill the buffer with either the buffersize
	// amount of data or the rest of the current datavec,
	// whichever is less
	for _, dv := range tv.Dvecs {
		size := dv.GetSize()
		if off+size <= tv.Pos {
			off += size
		} else {
			err := dv.Open()
			// XXX if os.IsNotExist(err), return 0s?
			if err != nil {
				return 0, errors.Wrap(err, fmt.Sprintf("opening vec"))
			}
			defer dv.Close()

			n, err := dv.ReadAt(b, tv.Pos-off)
			tv.Pos += int64(n)
			return n, err
		}
	}
	return 0, io.EOF
}

// Seek the virtual offset of the tarvec
func (tv *TarVec) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return 0, os.ErrInvalid
		}
		tv.Pos = offset
		return tv.Pos, nil
	case io.SeekCurrent:
		if tv.Pos+offset < 0 {
			return 0, os.ErrInvalid
		}
		tv.Pos += offset
		return tv.Pos, nil
	case io.SeekEnd:
		if tv.Size+offset < 0 {
			return 0, os.ErrInvalid
		}
		tv.Pos = tv.Size + offset
		return tv.Pos, nil
	}
	return 0, os.ErrInvalid
}

func (tv *TarVec) Close() error {
	for _, dv := range tv.Dvecs {
		dv.Close()
	}
	return nil
}

// GenVec generates the tarvec and positioninfo from a list of files
func GenVec(files []string) (TarVec, []PositionInfo, error) {
	var tv TarVec
	pinfo := make([]PositionInfo, len(files))

	for i, file := range files {
		// book keeping for file offsets within archive file
		pinfo[i].Name = file
		pinfo[i].Offset = tv.Size

		fi, err := os.Lstat(file)
		if err != nil {
			continue
		}
		pinfo[i].Size = fi.Size()

		// create buffer to write tar header to
		buf := new(bytes.Buffer)
		tw := tar.NewWriter(buf)

		// generate tar header from file stat info
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return TarVec{}, []PositionInfo{},
				errors.Wrap(err, fmt.Sprintf("generating header %v", file))
		}

		// write tar header to buffer
		err = tw.WriteHeader(hdr)
		if err != nil {
			return TarVec{}, []PositionInfo{},
				errors.Wrap(err, fmt.Sprintf("writing header %v", file))
		}

		memv := MemVec{
			Data: buf.Bytes(),
		}

		// add the tar header mem buffer to the tarvec
		tv.Dvecs = append(tv.Dvecs, memv)
		tv.Size += memv.GetSize()

		pathv := PathVec{
			Path: file,
			Info: fi,
		}

		// add the file path info to the tarvec
		tv.Dvecs = append(tv.Dvecs, &pathv)
		tv.Size += pathv.GetSize()

		// tar requires file entries to be padded out to
		// 512 byte offset
		// if needed, record how much padding is needed
		// and add to the tarvec
		if fi.Size()%512 != 0 {
			padv := PadVec{
				Size: 512 - (fi.Size() % 512),
			}

			tv.Dvecs = append(tv.Dvecs, padv)
			tv.Size += padv.GetSize()
		}
	}

	tv.ComputeSize()
	tv.Pos = 0
	return tv, pinfo, nil
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
