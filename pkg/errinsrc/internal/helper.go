package internal

// ErrParams is an internal data type to force the
// creation of ErrInSrc objects from only being inside
// the errinsrc package, ideally the `srcerrors` package.
type ErrParams struct {
	Code      int          `json:"code"`
	Title     string       `json:"title"`
	Summary   string       `json:"summary"`
	Detail    string       `json:"detail,omitempty"`
	Cause     error        `json:"-,omitempty"`
	Locations SrcLocations `json:"locations,omitempty"`
}
