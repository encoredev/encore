package api

import "encore.dev/middleware"

type Middleware struct {
	PkgName string
	Name    string
	Global  bool
	DefLoc  int32
	Invoke  middleware.Signature
}

// Validator is the interface implemented by types
// that can validate incoming requests.
type Validator interface {
	Validate() error
}
