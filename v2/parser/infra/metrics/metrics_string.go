// Code generated by "stringer -type=MetricType -output=metrics_string.go"; DO NOT EDIT.

package metrics

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Counter-0]
	_ = x[Gauge-1]
}

const _MetricType_name = "CounterGauge"

var _MetricType_index = [...]uint8{0, 7, 12}

func (i MetricType) String() string {
	if i < 0 || i >= MetricType(len(_MetricType_index)-1) {
		return "MetricType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _MetricType_name[_MetricType_index[i]:_MetricType_index[i+1]]
}
