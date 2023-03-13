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
	Cfg   SubscriptionConfig
}

type SubscriptionConfig struct {
	AckDeadline      time.Duration
	MessageRetention time.Duration
	MinRetryBackoff  time.Duration
	MaxRetryBackoff  time.Duration
	MaxRetries       int
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
		errs.Add(ErrSubscriptionTopicNotResource.AtGoNode(topicExpr))
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
		AckDeadline      time.Duration `literal:",optional" default:"30*time.Second"`
		MessageRetention time.Duration `literal:",optional" default:"7*24*time.Hour"`
		RetryPolicy      struct {
			MinRetryBackoff time.Duration `literal:"MinBackoff,optional" default:"10*time.Second"`
			MaxRetryBackoff time.Duration `literal:"MaxBackoff,optional" default:"10*time.Minute"`
			MaxRetries      int           `literal:"MaxRetries,optional" default:"100"`
		} `literal:",optional"`
	}

	cfg := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit)

	// Set defaults
	if cfg.AckDeadline == 0 {
		cfg.AckDeadline = 30 * time.Second
	}
	if cfg.MessageRetention == 0 {
		cfg.MessageRetention = 7 * 24 * time.Hour
	}
	if cfg.RetryPolicy.MinRetryBackoff == 0 {
		cfg.RetryPolicy.MinRetryBackoff = 10 * time.Second
	}
	if cfg.RetryPolicy.MaxRetryBackoff == 0 {
		cfg.RetryPolicy.MaxRetryBackoff = 10 * time.Minute
	}
	if cfg.RetryPolicy.MaxRetries == 0 {
		cfg.RetryPolicy.MaxRetries = 100
	}

	// TODO(andre) validate value ranges

	// TODO(andre) Handle pubsub attribute parsing

	// TODO(andre) handle default values, validate ranges
	subCfg := SubscriptionConfig{
		AckDeadline:      cfg.AckDeadline,
		MessageRetention: cfg.MessageRetention,
		MinRetryBackoff:  cfg.RetryPolicy.MinRetryBackoff,
		MaxRetryBackoff:  cfg.RetryPolicy.MaxRetryBackoff,
		MaxRetries:       cfg.RetryPolicy.MaxRetries,
	}

	// TODO(andre) fill in this
	sub := &Subscription{
		AST:   d.Call,
		File:  d.File,
		Name:  subscriptionName,
		Doc:   d.Doc,
		Topic: topicObj,
		Cfg:   subCfg,
	}
	d.Pass.RegisterResource(sub)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, sub)
	}
}
