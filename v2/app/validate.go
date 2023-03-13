package app

import (
	"encr.dev/v2/internal/parsectx"
)

// validate checks that the application is in a valid state across all services and compilation units.
func (d *Desc) validate(pc *parsectx.Context) {
	// Validate the framework
	if fw, ok := d.Framework.Get(); ok {
		d.validateAuthHandlers(pc, fw)
		d.validateAPIs(pc, fw)
	}
}
