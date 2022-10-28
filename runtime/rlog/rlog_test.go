package rlog

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestReserveEncoreKey(t *testing.T) {
	testCases := []struct {
		Key  string
		Want string
	}{
		{
			Key: "key",
			Want: `{"level":"info","key":"value"}
`,
		},
		{
			Key: "encore_key",
			Want: `{"level":"info","x_encore_key":"value"}
`,
		},
		{
			Key: "encorekey",
			Want: `{"level":"info","encorekey":"value"}
`,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Key, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			ev := logger.Info()
			addEventEntry(ev, testCase.Key, "value")
			ev.Send()
			actual := buf.String()
			if actual != testCase.Want {
				t.Fatalf("\nwant:\n\t%q\ngot:\n\t%q\n", testCase.Want, actual)
			}
		})
		t.Run(testCase.Key, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			logger = addContext(logger.With(), testCase.Key, "value").Logger()
			logger.Info().Send()
			actual := buf.String()
			if actual != testCase.Want {
				t.Fatalf("\nwant:\n\t%q\ngot:\n\t%q\n", testCase.Want, actual)
			}
		})
	}
}
