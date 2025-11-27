package app

import (
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/testscript"

	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, nil))
}

// Parse will take the given archiveContent and parse it into a testutil.Context and parser.Result.
//
// If the parser errors, the test will immediately fail.
func Parse(c *qt.C, archiveContent string) (*testutil.Context, *parser.Result) {

	archive := testutil.ParseTxtar(`
-- go.mod --
module example.com

go 1.20

require encore.dev v1.52.0

` + archiveContent)

	tc := testutil.NewContext(c, false, archive)
	tc.GoModDownload()
	p := parser.NewParser(tc.Context)
	tc.FailTestOnBailout()
	parserResult := p.Parse()

	if tc.Errs.Len() > 0 {
		c.Fatalf("parsing failed: %v", tc.Errs.FormatErrors())
	}

	return tc, parserResult
}
