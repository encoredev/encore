package errinsrc

import (
	"fmt"
	"sort"
)

// List is a list of ErrInSrc objects.
type List []*ErrInSrc

var _ sort.Interface = List{}
var _ ErrorList = List{}
var _ error = List{}

func (l List) Len() int {
	return len(l)
}

func (l List) Less(i, j int) bool {
	// This less function follows (as much as possible) the behabiour
	// of scanner.ErrorList's sort, that is filename, then line, then column
	// We then move onto extra data only Encore has
	iErr, jErr := l[i], l[j]

	numLocationsToCompare := len(iErr.Params.Locations)
	if otherNum := len(jErr.Params.Locations); otherNum < numLocationsToCompare {
		numLocationsToCompare = otherNum
	}

	for idx := 0; idx < numLocationsToCompare; idx++ {
		iLoc, jLoc := iErr.Params.Locations[idx], jErr.Params.Locations[idx]
		if iLoc.Less(jLoc) {
			return true
		}
	}

	// Now our custom sort logic
	if iErr.Params.Code != jErr.Params.Code {
		return iErr.Params.Code < jErr.Params.Code
	}

	if iErr.Params.Title != jErr.Params.Title {
		return iErr.Params.Title < jErr.Params.Title
	}

	if iErr.Params.Summary != jErr.Params.Summary {
		return iErr.Params.Summary < jErr.Params.Summary
	}

	if iErr.Params.Detail != jErr.Params.Detail {
		return iErr.Params.Detail < jErr.Params.Detail
	}

	if len(iErr.Params.Locations) != len(jErr.Params.Locations) {
		return len(iErr.Params.Locations) < len(jErr.Params.Locations)
	}

	return false
}

func (l List) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l List) Error() string {
	switch len(l) {
	case 0:
		return "no errors"
	case 1:
		return l[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", l[0], len(l)-1)
}
func (l List) ErrorList() []*ErrInSrc { return l }
