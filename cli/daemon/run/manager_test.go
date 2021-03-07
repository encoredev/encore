package run

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.uber.org/goleak"
)

// TestNewListener tests that newListener tries multiple ports.
func TestNewListener(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	c := qt.New(t)
	mgr := &Manager{}

	ln1, port1, err1 := mgr.newListener()
	ln2, port2, err2 := mgr.newListener()
	defer closeAll(ln1, ln2)
	c.Assert(err1, qt.IsNil)
	c.Assert(err2, qt.IsNil)
	c.Assert(port1 >= BasePort && port1 <= (BasePort+10), qt.IsTrue)
	c.Assert(port2, qt.Equals, port1+1)

	// newListener should pick up the original port again once ln1 is closed.
	ln1.Close()
	ln3, port3, err3 := mgr.newListener()
	defer closeAll(ln3)
	c.Assert(err3, qt.IsNil)
	c.Assert(port3, qt.Equals, port1)
}
