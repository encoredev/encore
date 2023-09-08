package editors

import (
	"context"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestResolve(t *testing.T) {
	c := qt.New(t)

	editors, err := Resolve(context.Background())
	c.Assert(err, qt.IsNil)
	fmt.Printf("Found editors:\n%+v\n", editors)
}
