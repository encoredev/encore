package traceprovider

import (
	"encore.dev/appruntime/exported/trace"
)

type Factory interface {
	NewLogger() trace.Logger
}

type DefaultFactory struct{}

func (*DefaultFactory) NewLogger() trace.Logger { return &trace.Log{} }
