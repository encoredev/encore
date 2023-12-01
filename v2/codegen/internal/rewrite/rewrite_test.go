package rewrite

import (
	"bytes"
	"testing"
)

func TestSplit(t *testing.T) {
	rw := New([]byte("test"), 1)
	rw.Replace(2, 4, []byte("ou"))  // "tout"
	rw.Replace(1, 2, []byte("rea")) // "reaout"
	rw.Insert(4, []byte("h"))       // "reaouht"
	rw.Insert(4, []byte("a"))       // "reaouaht"
	if got, want := rw.Data(), []byte("reaouhat"); !bytes.Equal(got, want) {
		t.Errorf("got data %s, want %s", got, want)
	}
}

func TestReplaceAcrossSegments(t *testing.T) {
	rw := New([]byte("foo bar"), 1)
	rw.Replace(5, 6, []byte("l"))  // "foo lar"
	rw.Replace(2, 7, []byte("hi")) // "fhir"
	if got, want := rw.Data(), []byte("fhir"); !bytes.Equal(got, want) {
		t.Errorf("got data %s, want %s", got, want)
	}
}

func TestReplaceTwice(t *testing.T) {
	rw := New([]byte("foo bar"), 1)
	rw.Replace(5, 6, []byte("l"))     // "foo lar"
	rw.Replace(2, 7, []byte("hi"))    // "fhir"
	rw.Replace(2, 7, []byte("hello")) // "fhellor"
	if got, want := rw.Data(), []byte("fhellor"); !bytes.Equal(got, want) {
		t.Errorf("got data %s, want %s", got, want)
	}
}

func TestDelete(t *testing.T) {
	rw := New([]byte("foo bar"), 1)
	rw.Replace(5, 6, []byte("l")) // "foo lar"
	rw.Delete(2, 7)
	if got, want := rw.Data(), []byte("fr"); !bytes.Equal(got, want) {
		t.Errorf("got data %s, want %s", got, want)
	}
}

func TestInsertAtEnd(t *testing.T) {
	rw := New([]byte(""), 1)
	rw.Insert(1, []byte("// test"))
	if got, want := rw.Data(), []byte("// test"); !bytes.Equal(got, want) {
		t.Errorf("got data %s, want %s", got, want)
	}
	rw.Delete(1, 4)
	if got, want := rw.Data(), []byte("test"); !bytes.Equal(got, want) {
		t.Errorf("got data %s, want %s", got, want)
	}
}
