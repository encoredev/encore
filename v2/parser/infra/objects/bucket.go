package objects

import (
	"go/ast"
	"go/token"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	literals "encr.dev/v2/parser/infra/internal/literals"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

type Bucket struct {
	AST       *ast.CallExpr
	File      *pkginfo.File
	Name      string // The unique name of the bucket
	Doc       string // The documentation on the bucket
	Versioned bool
	Public    bool
}

func (t *Bucket) Kind() resource.Kind       { return resource.Bucket }
func (t *Bucket) Package() *pkginfo.Package { return t.File.Pkg }
func (t *Bucket) ASTExpr() ast.Expr         { return t.AST }
func (t *Bucket) ResourceName() string      { return t.Name }
func (t *Bucket) Pos() token.Pos            { return t.AST.Pos() }
func (t *Bucket) End() token.Pos            { return t.AST.End() }
func (t *Bucket) SortKey() string           { return t.Name }

var BucketParser = &resourceparser.Parser{
	Name: "Bucket",

	InterestingImports: []paths.Pkg{"encore.dev/storage/objects"},
	Run: func(p *resourceparser.Pass) {
		name := pkginfo.QualifiedName{Name: "NewBucket", PkgPath: "encore.dev/storage/objects"}

		spec := &parseutil.ReferenceSpec{
			MinTypeArgs: 0,
			MaxTypeArgs: 0,
			Parse:       parseBucket,
		}

		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseBucket(d parseutil.ReferenceInfo) {
	errs := d.Pass.Errs

	if len(d.Call.Args) != 2 {
		errs.Add(errNewBucketArgCount(len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	bucketName := parseutil.ParseResourceName(d.Pass.Errs, "objects.NewBucket", "bucket name",
		d.Call.Args[0], parseutil.KebabName, "")
	if bucketName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "objects.BucketConfig", d.Call.Args[1])
	if !ok {
		return // error reported by ParseStruct
	}

	// Decode the config
	type decodedConfig struct {
		Versioned bool `literal:",optional"`
		Public    bool `literal:",optional"`
	}
	config := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit, nil)

	bkt := &Bucket{
		AST:       d.Call,
		File:      d.File,
		Name:      bucketName,
		Doc:       d.Doc,
		Versioned: config.Versioned,
		Public:    config.Public,
	}
	d.Pass.RegisterResource(bkt)
	d.Pass.AddBind(d.File, d.Ident, bkt)
}
