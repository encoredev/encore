package pubsub

import (
	"go/ast"
	"time"

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
	Name  string // The unique name of the pub sub subscription
	Doc   string // The documentation on the pub sub subscription
	Topic pkginfo.QualifiedName
}

func (s *Subscription) Kind() resource.Kind       { return resource.PubSubSubscription }
func (s *Subscription) Package() *pkginfo.Package { return s.File.Pkg }
func (s *Subscription) ASTExpr() ast.Expr         { return s.AST }
func (s *Subscription) ResourceName() string      { return s.Name }

var SubscriptionParser = &resource.Parser{
	Name: "PubSub Subscription",

	InterestingImports: []paths.Pkg{"encore.dev/pubsub"},
	Run: func(p *resource.Pass) {
		name := pkginfo.QualifiedName{Name: "NewSubscription", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil.ReferenceSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 1,
			Parse:       parsePubSubSubscription,
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

func parsePubSubSubscription(d parseutil.ReferenceInfo) {
	displayName := d.ResourceFunc.NaiveDisplayName()
	errs := d.Pass.Errs
	if len(d.Call.Args) != 3 {
		errs.Add(errNewSubscriptionArgCount(len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	topicExpr := d.Call.Args[0]
	topicObj, ok := d.File.Names().ResolvePkgLevelRef(topicExpr)
	if !ok {
		errs.Add(errSubscriptionTopicNotResource.AtGoNode(topicExpr))
		return
	}

	subscriptionName := parseutil.ParseResourceName(d.Pass.Errs, displayName, "subscription name",
		d.Call.Args[1], parseutil.KebabName, "")
	if subscriptionName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	// Parse the literal struct representing the subscription configuration
	// so we can extract the reference to the handler function
	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "pubsub.SubscriptionConfig", d.Call.Args[2])
	if !ok {
		return // error reported by ParseStruct
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
	sub := &Subscription{
		AST:   d.Call,
		File:  d.File,
		Name:  subscriptionName,
		Doc:   d.Doc,
		Topic: topicObj,
	}
	d.Pass.RegisterResource(sub)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, sub)
	}
}
