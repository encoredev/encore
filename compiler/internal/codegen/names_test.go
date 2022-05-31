package codegen

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestAlloc(t *testing.T) {
	c := qt.New(t)

	var a nameAllocator
	c.Assert(a.Get("foo"), qt.Equals, "foo")
	c.Assert(a.Get("foo"), qt.Equals, "foo2")
	c.Assert(a.Get("foo"), qt.Equals, "foo3")
	c.Assert(a.Get("func"), qt.Equals, "_func")
	c.Assert(a.Get("func"), qt.Equals, "_func2")
}
