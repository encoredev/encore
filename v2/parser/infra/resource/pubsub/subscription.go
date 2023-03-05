package pubsub

import (
	"go/ast"
	"time"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

type Subscription struct {
	AST   *ast.CallExpr
	File  *pkginfo.File
	Ident *ast.Ident // The identifier of the pub sub subscription
	Name  string     // The unique name of the pub sub subscription
	Doc   string     // The documentation on the pub sub subscription
	Topic pkginfo.QualifiedName
}

func (s *Subscription) Kind() resource.Kind       { return resource.PubSubSubscription }
func (s *Subscription) DeclaredIn() *pkginfo.File { return s.File }
func (s *Subscription) ASTExpr() ast.Expr         { return s.AST }
func (s *Subscription) BoundTo() option.Option[pkginfo.QualifiedName] {
	return parseutil.BoundTo(s.File, s.Ident)
}

var SubscriptionParser = &resource.Parser{
	Name: "PubSub Subscription",

	RequiredImports: []paths.Pkg{"encore.dev/pubsub"},
	Run: func(p *resource.Pass) []resource.Resource {
		name := pkginfo.QualifiedName{Name: "NewSubscription", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 1,
			Parse:       parsePubSubSubscription,
		}

		var resources []resource.Resource
		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			r := parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
			if r != nil {
				resources = append(resources, r)
			}
		})
		return resources
	},
}

func parsePubSubSubscription(d parseutil.ParseData) resource.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 3 {
		d.Pass.Errs.Addf(d.Call.Pos(), "%s expects 3 arguments", displayName)
		return nil
	}

	topicExpr := d.Call.Args[0]
	topicObj, ok := d.File.Names().ResolvePkgLevelRef(topicExpr)
	if !ok {
		d.Pass.Errs.Addf(topicExpr.Pos(), "could not resolve topic to a package-level variable")
		return nil
	}

	subscriptionName := parseutil.ParseResourceName(d.Pass.Errs, displayName, "subscription name",
		d.Call.Args[1], parseutil.KebabName, "")
	if subscriptionName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	// Parse the literal struct representing the subscription configuration
	// so we can extract the reference to the handler function
	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "pubsub.SubscriptionConfig", d.Call.Args[2])
	if !ok {
		return nil // error reported by ParseStruct
	}

	type decodedConfig struct {
		Handler ast.Expr `literal:",dynamic,required"`

		// Optional configuration
		AckDeadline      time.Duration `literal:",optional"`
		MessageRetention time.Duration `literal:",optional"`
		RetryPolicy      struct {
			MinRetryBackoff time.Duration `literal:"MinBackoff,optional"`
			MaxRetryBackoff time.Duration `literal:"MaxBackoff,optional"`
			MaxRetries      int           `literal:"MaxRetries,optional"`
		} `literal:",optional"`
	}

	cfg := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit)
	_ = cfg

	// TODO(andre) validate value ranges

	// TODO(andre) Handle pubsub attribute parsing

	// TODO(andre) fill in this
	return &Subscription{
		AST:   d.Call,
		File:  d.File,
		Ident: d.Ident,
		Name:  subscriptionName,
		Doc:   d.Doc,
		Topic: topicObj,
	}
}
