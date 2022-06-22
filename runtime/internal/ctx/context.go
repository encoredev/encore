// Package ctx is shorthand for context and contains a shared global context for the running Encore application
package ctx

import (
	"context"
	"os/signal"
	"syscall"
)

// App is a global context that is closed when the application has been given a SIGTERM or SIGINT signal.
var App context.Context

func init() {
	App, _ = signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
}
