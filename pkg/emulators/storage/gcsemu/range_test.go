package gcsemu

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseByteRange(t *testing.T) {
	tcs := []struct {
		in     string
		expect byteRange
	}{
		{in: "bytes 0-8388607/*", expect: byteRange{lo: 0, hi: 8388607, sz: -1}},
		{in: "bytes 8388608-10485759/10485760", expect: byteRange{lo: 8388608, hi: 10485759, sz: 10485760}},
		{in: "bytes */10485760", expect: byteRange{lo: -1, hi: -1, sz: 10485760}},
	}

	for _, tc := range tcs {
		t.Logf("test case: %s", tc.in)
		assert.Equal(t, tc.expect, *parseByteRange(tc.in))
	}
}
