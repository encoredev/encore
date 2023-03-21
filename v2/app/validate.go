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
		d.validateAPIs(pc, fw, result)
		d.validateMiddleware(pc, fw)
		d.validateServiceStructs(pc, result)
	}

	// Validate infrastructure
	d.validateCaches(pc, result)
	d.validateConfigs(pc, result)
	d.validateCrons(pc, result)
	d.validatePubSub(pc, result)

	// TODO: validate that all resources are defined in services

	// TODO: validate that the ET package is only used within test files
}
