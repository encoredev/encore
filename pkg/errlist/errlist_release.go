//go:build !dev_build
// +build !dev_build

package errlist

import (
	"go/scanner"
	"go/token"
)

// addErrToList adds a parse error to the list.
func addErrToList(list *scanner.ErrorList, position token.Position, msg string) {
	list.Add(position, msg)
}
