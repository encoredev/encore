package service

import "encr.dev/v2/internal/paths"

// Service describes an Encore service.
type Service struct {
	// Name is the name of the service.
	Name string

	// FSRoot is the root directory of the service.
	FSRoot paths.FS
}
