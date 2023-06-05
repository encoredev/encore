package dash

import (
	"testing"

	qt "github.com/frankban/quicktest"
	jsoniter "github.com/json-iterator/go"
)

func TestListEncoder(t *testing.T) {
	c := qt.New(t)

	jsoniterAPI := jsoniter.Config{SortMapKeys: true}.Froze()
	jsoniterAPI.RegisterExtension(NewListEncoderExtension())
	marshal, err := jsoniterAPI.Marshal(struct {
		StringList    []string `json:",omitempty"`
		IntList       []string
		FloatList     []float64
		NilBytes      []byte
		EmptyBytes    []byte
		SomeBytes     []byte
		StringPointer *string
		ZeroArray     [0]int
	}{
		FloatList:  []float64{1.0, 2.0, 3.0},
		NilBytes:   nil,
		EmptyBytes: []byte{},
		SomeBytes:  []byte("foobar"),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(string(marshal), qt.Equals, `{"IntList":[],"FloatList":[1,2,3],"NilBytes":"","EmptyBytes":"","SomeBytes":"Zm9vYmFy","StringPointer":null,"ZeroArray":[]}`)
}
