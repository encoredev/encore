package app

import (
	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/infra/secrets"
)

// validate checks that the application is in a valid state across all services and compilation units.
func (d *Desc) validate(pc *parsectx.Context, result *parser.Result) {
	defer pc.Trace("app.validate").Done()

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

	// Validate all resources are defined within a service
	for _, r := range result.Resources() {
		switch r.(type) {
		case *pubsub.Topic:
			// We allow pubsub topics to be declared outside of service code
			continue
		case *middleware.Middleware:
			// Middleware is also allowed to be declared outside of service code if it's global (validateMiddleware checks this already)
			continue
		case *authhandler.AuthHandler:
			// AuthHandlers are also allowed to be declared outside of service code as it's shared code between all services
			continue
		case *secrets.Secrets:
			// Secrets are allowed anywhere
			continue

		default:
			_, ok := d.ServiceForPath(r.Package().FSPath)
			if !ok {
				pc.Errs.Add(errResourceDefinedOutsideOfService.AtGoNode(r))
			}
		}
	}

	// Validate nothing is accessing an et package if it isn't a test file
	etPkg := paths.Pkg("encore.dev/et")
	for _, pkg := range result.AppPackages() {
		for _, file := range pkg.Files {
			if !file.TestFile {
				for importPath, importSpec := range file.Imports {
					if etPkg.LexicallyContains(importPath) {
						pc.Errs.Add(errETPackageUsedOutsideOfTestFile.AtGoNode(importSpec, errors.AsError("imported here")))
					}
				}
			}
		}
	}
}
