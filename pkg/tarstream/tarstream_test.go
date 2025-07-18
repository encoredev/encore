package tarstream

import (
	"math/rand"
	"slices"
	"testing"
	"testing/iotest"
	"testing/quick"
)

func TestReader(t *testing.T) {
	err := quick.Check(func(data []byte) bool {
		tv := genRandomVec(data)
		if err := iotest.TestReader(tv, data); err != nil {
			t.Logf("got read err %v", err)
			return false
		}
		return true
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func genRandomVec(data []byte) *TarVec {
	var vecs []Datavec
	for len(data) > 0 {
		n := rand.Intn(len(data) + 1)
		vecData := data[:n]
		data = data[n:]
		allZeroes := !slices.ContainsFunc(vecData, func(b byte) bool {
			return b != 0
		})

		if allZeroes {
			vecs = append(vecs, PadVec{Size: int64(len(vecData))})
		} else {
			vecs = append(vecs, MemVec{Data: vecData})
		}
	}
	return NewTarVec(vecs)
}
