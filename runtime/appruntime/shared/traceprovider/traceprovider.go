package traceprovider

import (
	"encore.dev/appruntime/exported/trace2"
)

type Factory interface {
	NewLogger() trace2.Logger
}

type DefaultFactory struct{}

func (*DefaultFactory) NewLogger() trace2.Logger { return &trace2.Log{} }
