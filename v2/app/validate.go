package app

import (
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/parser"
)

// validate checks that the application is in a valid state across all services and compilation units.
func (d *Desc) validate(pc *parsectx.Context, result *parser.Result) {
	// Validate the framework
	if fw, ok := d.Framework.Get(); ok {
		d.validateAuthHandlers(pc, fw)
		d.validateAPIs(pc, fw)
	}

	// Validate infrastructure
	d.validateCrons(pc, result)
	d.validatePubSub(pc)
}
