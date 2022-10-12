package errinsrc

import (
	"os"
	"path"
	"testing"

	"encr.dev/pkg/golden"
)

var testDataFullPath string

func TestMain(m *testing.M) {
	ColoursInErrors(false)
	golden.TestMain(m)
}

func init() {
	var err error
	testDataFullPath, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	testDataFullPath = path.Join(testDataFullPath, "testdata")
}
