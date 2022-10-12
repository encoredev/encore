package internal

// ErrParams are used to create *errinsrc.ErrInSrc objects.
//
// It exists within an `internal` package so that it can only
// be used by other packages within the `errinsrc` folder.
// This is enforce through the Go compiler that all
// errors are created inside the `srcerrors` subpackage.
type ErrParams struct {
	Code      int          `json:"code"`
	Title     string       `json:"title"`
	Summary   string       `json:"summary"`
	Detail    string       `json:"detail,omitempty"`
	Cause     error        `json:"-,omitempty"`
	Locations SrcLocations `json:"locations,omitempty"`
}
