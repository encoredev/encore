package traceprovider

import (
	trace2 "encore.dev/appruntime/exported/trace"
)

type Factory interface {
	NewLogger() trace2.Logger
}
