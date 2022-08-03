package api

import "encore.dev/middleware"

type Middleware struct {
	PkgName string
	Name    string
	Global  bool
	DefLoc  int32
	Invoke  middleware.Signature
}
