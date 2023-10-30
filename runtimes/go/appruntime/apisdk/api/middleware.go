package api

import "encore.dev/middleware"

type Middleware struct {
	ID      string
	PkgName string
	Name    string
	Global  bool
	DefLoc  uint32
	Invoke  middleware.Signature
}

// Validator is the interface implemented by types
// that can validate incoming requests.
type Validator interface {
	Validate() error
}
