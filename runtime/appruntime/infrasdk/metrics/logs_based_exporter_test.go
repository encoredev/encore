package metrics

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestLogCounter(t *testing.T) {
	testCases := []struct {
		Name    string
		Counter string
		Tags    []string
		Want    string
	}{
		{
			Name:    "Increase counter with one dimension",
			Counter: "test_counter",
			Tags:    []string{"tag", "value"},
			Want: `{"level":"trace","encore_metric_name":"test_counter","tag":"value"}
`,
		},
		{
			Name:    "Increase counter with two dimensions",
			Counter: "test_counter",
			Tags:    []string{"tag_1", "value_1", "tag_2", "value_2"},
			Want: `{"level":"trace","encore_metric_name":"test_counter","tag_1":"value_1","tag_2":"value_2"}
`,
		},
		{
			Name:    "Increase counter with three dimensions",
			Counter: "test_counter",
			Tags:    []string{"tag_1", "value_1", "tag_2", "value_2", "tag_3", "value_3"},
			Want: `{"level":"trace","encore_metric_name":"test_counter","tag_1":"value_1","tag_2":"value_2","tag_3":"value_3"}
`,
		},
		{
			Name:    "Drop tag without value",
			Counter: "test_counter",
			Tags:    []string{"tag_1", "value_1", "tag_2"},
			Want: `{"level":"trace","encore_metric_name":"test_counter","tag_1":"value_1"}
`,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			newLogsBasedEmitter(logger).logCounter(testCase.Counter, testCase.Tags...)
			actual := buf.String()
			if actual != testCase.Want {
				t.Fatalf("\nwant:\n\t%q\ngot:\n\t%q\n", testCase.Want, actual)
			}
		})
	}
}
