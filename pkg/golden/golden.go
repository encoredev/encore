package golden

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	update      bool // should we update golden files?
	testMainRan bool // did TestMain get called?
)

// Test checks the test output against the golden file where the path to the golden file is
// based on the test name; i.e. `testName.golden` within the `testdata` folder.
// If -golden-update was passed to "go test", it writes new golden files instead.
func Test(t testing.TB, output string) {
	fn := strings.Replace(t.Name(), "/", "__", -1)
	TestAgainst(t, fn+".golden", output)
}

// TestAgainst checks the test output against the golden file.
// If -golden-update was passed to "go test", it writes new golden files instead.
func TestAgainst(t testing.TB, goldenFileName string, output string) {
	if !testMainRan {
		t.Fatal("golden.TestMain was not called")
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wd, "testdata", goldenFileName)

	if update {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		err := os.WriteFile(path, []byte(output), 0644)
		if err != nil {
			t.Fatalf("update golden: %v", err)
		}
	} else {
		expect, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden: %v", err)
		}
		if diff := cmp.Diff(output, string(expect)); diff != "" {
			t.Fatalf("bad output:\n%s", diff)
		}
	}
}

// TestMain sets up the golden testing functionality for the package.
// Packages that want to integrate golden testing should themselves
// implement TestMain and call this function.
func TestMain(m *testing.M) {
	flag.BoolVar(&update, "golden-update", false, "update golden files")
	flag.Parse()
	testMainRan = true
	os.Exit(m.Run())
}
