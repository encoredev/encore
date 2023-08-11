package maingen

import (
	"encore.dev/appruntime/exported/config"
	"encr.dev/pkg/option"
	"encr.dev/v2/codegen"
)

func genTestConfigs(p GenParams, test codegen.TestConfig) *config.Static {
	for _, pkg := range test.Packages {
		// HACK(andre) Ensure we always import the testsupport package in every test binary,
		// since the "testing" package depends on it for the testing runtime hooks.
		p.Gen.InsertTestSupport(pkg)

		//	f := p.Gen.InjectFile(pkg.ImportPath+"!test", pkg.Name+"_test", pkg.FSPath, "encore_internal__test.go", "encoretest")
		//	f.ImportAnon("encore.dev/appruntime/shared/testsupport")
		//
		//	if fw, ok := p.Desc.Framework.Get(); ok {
		//		if ah, ok := fw.AuthHandler.Get(); ok {
		//			f.ImportAnon(ah.Decl.File.Pkg.ImportPath)
		//		}
		//		for _, mw := range fw.GlobalMiddleware {
		//			f.ImportAnon(mw.File.Pkg.ImportPath)
		//		}
		//	}
	}

	return GenAppConfig(p, option.Some(testParams{
		EnvsToEmbed:        test.EnvsToEmbed,
		ExternalTestBinary: len(test.EnvsToEmbed) > 0,
	}))
}
