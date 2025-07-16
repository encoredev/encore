package tarstream

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"testing"
)

var teststring = `test`
var testfile1 = `testfile1`
var testfile2 = `testfile2`

func createFile(file string) error {
	err := os.WriteFile(file, []byte(teststring), 0644)
	if err != nil {
		return err
	}
	return nil
}

func TestGenVec(t *testing.T) {
	err := createFile(testfile1)
	if err != nil {
		t.Errorf("create file1: %v", err)
	}
	err = createFile(testfile2)
	if err != nil {
		t.Errorf("create file2: %v", err)
	}

	filelist := []string{testfile1, testfile2}
	tv, _, err := GenVec(filelist)
	if err != nil {
		t.Errorf("GenVec(): %v", err)
	}

	buffer := make([]byte, 2048)
	tot := int(0)
	for {
		n, err := tv.Read(buffer[tot:])
		if n == 0 && err == io.EOF {
			break
		}
		if err != nil && err != io.EOF {
			t.Errorf("Read(): %v", err)
		}
		tot += n
	}

	if len(buffer[:tot]) != 2048 {
		t.Errorf("Final buffer size %v, want %v",
			len(buffer[:tot]), 2048)
	}

	r := bytes.NewReader(buffer[:tot])
	tr := tar.NewReader(r)

	hdr, err := tr.Next()
	if err != nil && err != io.EOF {
		t.Errorf("Tar read: %v", err)
	}
	if hdr.Name != testfile1 {
		t.Errorf("Read Tar file: '%v', want '%v'",
			hdr.Name, testfile1)
	}
	var filebuf bytes.Buffer
	_, err = io.Copy(&filebuf, tr)
	if err != nil {
		t.Errorf("Copy file1: %v", err)
	}
	if string(filebuf.Bytes()) != teststring {
		t.Errorf("file1 got %q, want %q",
			string(filebuf.Bytes()), teststring)
	}

	hdr, err = tr.Next()
	if err != nil && err != io.EOF {
		t.Errorf("Tar read: %v", err)
	}
	if hdr.Name != testfile2 {
		t.Errorf("Read Tar file: '%v', want '%v'",
			hdr.Name, testfile2)
	}
	filebuf.Reset()
	_, err = io.Copy(&filebuf, tr)
	if err != nil {
		t.Errorf("Copy file2: %v", err)
	}
	if string(filebuf.Bytes()) != teststring {
		t.Errorf("file2 got %q, want %q",
			string(filebuf.Bytes()), teststring)
	}

	badfilelist := []string{"bogus1"}
	tv, _, err = GenVec(badfilelist)
	if err != nil {
		t.Errorf("GenVec(): %v", err)
	}

	for _, file := range []string{testfile1, testfile2} {
		err = os.Remove(file)
		if err != nil {
			t.Errorf("remove %v: %v", file, err)
		}
	}
}
