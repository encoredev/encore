package errors

import (
	"fmt"
)

var (
	nextRangeStart = 1000
)

// Range creates a new error code range for a given module.
func Range(
	module string,
	defaultDetails string,
	options ...RangeOption,
) *TemplateRange {
	if nextRangeStart > 9999 {
		// If we hit this, we need to increase the number of digits in the error code renderer
		panic("too many error code ranges")
	}

	// Configure the range
	cfg := &rangeConfig{
		rangeSize: 100,
	}
	for _, c := range options {
		c(cfg)
	}

	// Create the Range
	rangeStart := nextRangeStart
	nextRangeStart += cfg.rangeSize

	return &TemplateRange{
		module:         module,
		defaultDetails: defaultDetails,
		nextErrorCode:  rangeStart,
		codeRangeEnd:   nextRangeStart,
	}
}

type rangeConfig struct {
	rangeSize int
}

type RangeOption func(*rangeConfig)

// WithRangeSize sets the size of the range. The default is 100.
func WithRangeSize(size int) RangeOption {
	return func(cfg *rangeConfig) {
		cfg.rangeSize = size
	}
}

// TemplateRange is a helper for creating a range of error codes for a given module
// and generating templates for those errors.
type TemplateRange struct {
	module         string
	defaultDetails string
	nextErrorCode  int
	codeRangeEnd   int // Exclusive
}

// New creates a new template for an error in this range.
func (r *TemplateRange) New(title, summary string, options ...TemplateOption) Template {
	return r.Newf(title, summary, options...)()
}

// Newf creates a function to return a template for an error in this range where the summary is a format string.
func (r *TemplateRange) Newf(title, summaryFmt string, options ...TemplateOption) func(summaryArgs ...any) Template {
	if r.nextErrorCode >= r.codeRangeEnd {
		panic("too many errors in range")
	}
	errorCode := r.nextErrorCode
	r.nextErrorCode++

	return func(summaryArgs ...any) Template {
		tmp := Template{
			Code:    errorCode,
			Title:   title,
			Summary: fmt.Sprintf(summaryFmt, summaryArgs...),
			Detail:  r.defaultDetails,
		}

		for _, o := range options {
			o(&tmp)
		}

		return tmp
	}
}
