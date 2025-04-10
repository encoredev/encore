package api

import (
	"fmt"
	"testing"

	"github.com/frankban/quicktest"

	"encore.dev/appruntime/exported/model"
)

func TestParseTraceParent(t *testing.T) {
	q := quicktest.New(t)

	expTraceID, _ := model.GenTraceID()
	expSpanID, _ := model.GenSpanID()
	expSampled := "01"

	traceID, spanID, sampled, ok := parseTraceParent(fmt.Sprintf("00-%x-%x-%s", expTraceID[:], expSpanID[:], expSampled))
	q.Assert(ok, quicktest.IsTrue)
	q.Assert(traceID, quicktest.DeepEquals, expTraceID)
	q.Assert(spanID, quicktest.DeepEquals, expSpanID)
	q.Assert(sampled, quicktest.Equals, true)
}
