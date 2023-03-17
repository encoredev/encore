package app

import (
	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/config"
)

func (d *Desc) validateConfigs(pc *parsectx.Context, result *parser.Result) {
	// validate all config loads
	for _, res := range result.Resources() {
		switch res := res.(type) {
		case *config.Load:
			d.validateConfig(pc, result, res)
		}
	}
}

func (d *Desc) validateConfig(pc *parsectx.Context, result *parser.Result, cfg *config.Load) {
	// Verify the config
	svc, ok := d.ServiceForPath(cfg.File.FSPath)
	if !ok {
		pc.Errs.Add(config.ErrConfigUsedOutsideOfService.AtGoNode(cfg))
		return
	}
	if svc.FSRoot != cfg.File.Pkg.FSPath {
		pc.Errs.Add(config.ErrConfigUsedInSubPackage.AtGoNode(cfg))
	}

	// Verify usages are in the same service
	for _, use := range result.Usages(cfg) {
		if !use.DeclaredIn().FSPath.HasPrefix(svc.FSRoot) {
			pc.Errs.Add(
				config.ErrCrossServiceConfigUse.
					AtGoNode(use, errors.AsError("used here")).
					AtGoNode(cfg, errors.AsHelp("defined here")),
			)
		}
	}
}
