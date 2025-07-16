package tarstream

import (
	"io"
	"os"
	"testing"
)

var testsize = int64(4)
var testfile = `testfile`

func createVecs(file string) (MemVec, PadVec, PathVec, error) {
	memv := MemVec{
		Data: []byte(teststring),
	}
	padv := PadVec{
		Size: testsize,
	}
	pathv := PathVec{
		Path: testfile,
	}

	err := createFile(file)
	if err != nil {
		return MemVec{}, PadVec{}, PathVec{}, err
	}

	fi, err := os.Lstat(file)
	if err != nil {
		return MemVec{}, PadVec{}, PathVec{}, err
	}

	pathv.Info = fi

	return memv, padv, pathv, nil
}

func TestGetSize(t *testing.T) {
	memv, padv, pathv, err := createVecs(testfile)
	if err != nil {
		t.Errorf("write test file: %v", err)
	}

	if memv.GetSize() != int64(len(teststring)) {
		t.Errorf("memv.GetSize() = %v, want %v",
			memv.GetSize(), len(teststring))
	}
	if padv.GetSize() != testsize {
		t.Errorf("padv.GetSize() = %v, want %v",
			padv.GetSize(), testsize)
	}
	if pathv.GetSize() != int64(len(teststring)) {
		t.Errorf("pathv.GetSize() = %v, want %v",
			pathv.GetSize(), len(teststring))
	}

	for _, file := range []string{testfile} {
		err = os.Remove(file)
		if err != nil {
			t.Errorf("remove %v: %v", file, err)
		}
	}
}

func TestReadAt(t *testing.T) {
	memv, padv, pathv, err := createVecs(testfile)
	if err != nil {
		t.Errorf("write test file: %v", err)
	}

	if err = memv.Open(); err != nil {
		t.Errorf("memv.Open()")
	}
	defer memv.Close()

	if err = padv.Open(); err != nil {
		t.Errorf("padv.Open()")
	}
	defer padv.Close()

	if err = pathv.Open(); err != nil {
		t.Errorf("pathv.Open()")
	}
	defer pathv.Close()

	buffer := make([]byte, 10)
	n, err := memv.ReadAt(buffer, 0)
	if err != nil && err != io.EOF {
		t.Errorf("memv.ReadAt()")
	}
	if n != len(teststring) {
		t.Errorf("memv.ReadAt() got %v bytes, expected %v",
			n, len(teststring))
	}
	if string(buffer[:n]) != teststring {
		t.Errorf("memv.ReadAt() got '%v', expected '%v'",
			string(buffer[:n]), teststring)
	}

	buffer = make([]byte, 10)
	n, err = padv.ReadAt(buffer, 0)
	if err != nil && err != io.EOF {
		t.Errorf("padv.ReadAt()")
	}
	if n != int(testsize) {
		t.Errorf("padv.ReadAt() got %v bytes, expected %v",
			n, testsize)
	}

	n, err = pathv.ReadAt(buffer, 0)
	if err != nil && err != io.EOF {
		t.Errorf("pathv.ReadAt() %v", err)
	}
	if n != len(teststring) {
		t.Errorf("pathv.ReadAt() got %v bytes, expected %v",
			n, len(teststring))
	}
	if string(buffer[:n]) != teststring {
		t.Errorf("pathv.ReadAt() got '%v', expected '%v'",
			string(buffer[:n]), teststring)
	}

	buffer = make([]byte, 10)
	n, err = memv.ReadAt(buffer, 1)
	if err != nil && err != io.EOF {
		t.Errorf("memv.ReadAt()")
	}
	if n != len(teststring)-1 {
		t.Errorf("memv.ReadAt() got %v bytes, expected %v",
			n, len(teststring)-1)
	}
	if string(buffer[:n]) != string(teststring[1:]) {
		t.Errorf("memv.ReadAt() got '%v', expected '%v'",
			string(buffer[:n]), string(teststring[1:]))
	}

	buffer = make([]byte, 10)
	n, err = padv.ReadAt(buffer, 1)
	if err != nil && err != io.EOF {
		t.Errorf("padv.ReadAt()")
	}
	if n != int(testsize)-1 {
		t.Errorf("padv.ReadAt() got %v bytes, expected %v",
			n, testsize-1)
	}

	n, err = pathv.ReadAt(buffer, 1)
	if err != nil && err != io.EOF {
		t.Errorf("pathv.ReadAt() %v", err)
	}
	if n != len(teststring)-1 {
		t.Errorf("pathv.ReadAt() got %v bytes, expected %v",
			n, len(teststring)-1)
	}
	if string(buffer[:n]) != string(teststring[1:]) {
		t.Errorf("pathv.ReadAt() got '%v', expected '%v'",
			string(buffer[:n]), string(teststring[1:]))
	}

	for _, file := range []string{testfile} {
		err = os.Remove(file)
		if err != nil {
			t.Errorf("remove %v: %v", file, err)
		}
	}
}
