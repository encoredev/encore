//go:build !dev_build
// +build !dev_build

package internal

import (
	"go/scanner"
	"go/token"
)

// AddErrToList will add a parse error to the list, but also capture the position within our parse that the error originated
func AddErrToList(errors *scanner.ErrorList, position token.Position, msg string) {
	errors.Add(position, msg)
}
