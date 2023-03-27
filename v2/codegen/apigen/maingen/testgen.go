package maingen

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/codegen"
)

func genTestConfigs(p GenParams, test codegen.TestConfig) {
	// TEST
	//for _, pkg := range p.Desc.Parse.AppPackages() {
	//	//hasTestFile := func(f *pkginfo.File) bool { return f.TestFile }
	//	file := p.Gen.InjectFile(pkg.ImportPath, pkg.Name+"_test", pkg.FSPath, "encore_internal__dummy_test.go", "dummy")
	//	f := file.Jen
	//	f.Anon("encore.dev/appruntime/testsupport")
	//}

	for _, pkg := range test.Packages {
		file := p.Gen.InjectFile(pkg.ImportPath+"!test", pkg.Name+"_test", pkg.FSPath, "encore_internal__testmain_test.go", "testmain")
		f := file.Jen
		var serviceName string
		if svc, ok := p.Desc.ServiceForPath(pkg.FSPath); ok {
			serviceName = svc.Name
		}
		f.Anon("encore.dev/appruntime/testsupport")
		f.Anon("encore.dev/appruntime/app/appinit")

		genLoadApp(p, f, option.Some(testParams{
			ServiceName: serviceName,
			EnvsToEmbed: test.EnvsToEmbed,
		}))
	}
}
