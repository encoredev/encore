package pubsub

import (
	"fmt"
	"go/ast"
	"go/token"
	"time"

	"golang.org/x/tools/go/ast/astutil"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

type Subscription struct {
	AST   *ast.CallExpr
	File  *pkginfo.File
	Name  string // The unique name of the pub sub subscription
	Doc   string // The documentation on the pub sub subscription
	Topic pkginfo.QualifiedName
	Cfg   SubscriptionConfig

	// Handler is the AST expression defining the handler function.
	Handler ast.Expr

	// MethodHandler specifies whether the handler is a method on a service struct.
	MethodHandler option.Option[MethodHandler]
}

// MethodHandler is used to describe a handler that references a method on a service struct.
type MethodHandler struct {
	// The type declaration the handler is a method on.
	Decl *pkginfo.PkgDeclInfo
	// Method is the name of the method.
	Method string
}

type SubscriptionConfig struct {
	AckDeadline      time.Duration
	MessageRetention time.Duration
	MinRetryBackoff  time.Duration
	MaxRetryBackoff  time.Duration
	MaxRetries       int
	MaxConcurrency   int
}

func (s *Subscription) Kind() resource.Kind       { return resource.PubSubSubscription }
func (s *Subscription) Package() *pkginfo.Package { return s.File.Pkg }
func (s *Subscription) ASTExpr() ast.Expr         { return s.AST }
func (s *Subscription) ResourceName() string      { return s.Name }
func (s *Subscription) Pos() token.Pos            { return s.AST.Pos() }
func (s *Subscription) End() token.Pos            { return s.AST.End() }
func (s *Subscription) SortKey() string {
	return s.Topic.PkgPath.String() + "." + s.Topic.Name + "." + s.Name
}

var SubscriptionParser = &resourceparser.Parser{
	Name: "PubSub Subscription",

	InterestingImports: []paths.Pkg{"encore.dev/pubsub"},
	Run: func(p *resourceparser.Pass) {
		name := pkginfo.QualifiedName{Name: "NewSubscription", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil.ReferenceSpec{
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

	type retryConfig struct {
		MinRetryBackoff time.Duration `literal:"MinBackoff,optional,default"`
		MaxRetryBackoff time.Duration `literal:"MaxBackoff,optional,default"`
		MaxRetries      int           `literal:"MaxRetries,optional,default"`
	}
	type decodedConfig struct {
		Handler ast.Expr `literal:",dynamic,required"`

		// Optional configuration
		MaxConcurrency   int           `literal:",optional,default"`
		AckDeadline      time.Duration `literal:",optional,default"`
		MessageRetention time.Duration `literal:",optional,default"`
		RetryPolicy      retryConfig   `literal:",optional,default"`
	}
	defaults := decodedConfig{
		MaxConcurrency:   100,
		AckDeadline:      30 * time.Second,
		MessageRetention: 7 * 24 * time.Hour,
		RetryPolicy: retryConfig{
			MinRetryBackoff: 10 * time.Second,
			MaxRetryBackoff: 10 * time.Minute,
			MaxRetries:      100,
		},
	}

	cfg := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit, &defaults)

	// Verify we have a config which is in-range of acceptable values
	if cfg.AckDeadline < 1*time.Second {
		errs.Add(errSubscriptionAckDeadlineTooShort.AtGoNode(cfgLit.Expr("AckDeadline"), errors.AsError(fmt.Sprintf("got %s", cfg.AckDeadline))))
	}

	if cfg.MessageRetention < 1*time.Minute {
		errs.Add(errSubscriptionMessageRetentionTooShort.AtGoNode(cfgLit.Expr("MessageRetention"), errors.AsError(fmt.Sprintf("got %s", cfg.MessageRetention))))
	}

	if cfg.RetryPolicy.MinRetryBackoff < 1*time.Second {
		errs.Add(errSubscriptionMinRetryBackoffTooShort.AtGoNode(cfgLit.Expr("RetryPolicy.MinBackoff"), errors.AsError(fmt.Sprintf("got %s", cfg.RetryPolicy.MinRetryBackoff))))
	}

	if cfg.RetryPolicy.MaxRetryBackoff < 1*time.Second {
		errs.Add(errSubscriptionMaxRetryBackoffTooShort.AtGoNode(cfgLit.Expr("RetryPolicy.MaxBackoff"), errors.AsError(fmt.Sprintf("got %s", cfg.RetryPolicy.MaxRetryBackoff))))
	}

	if cfg.RetryPolicy.MaxRetries < -2 {
		errs.Add(errSubscriptionMaxRetriesTooSmall.AtGoNode(cfgLit.Expr("RetryPolicy.MaxRetries"), errors.AsError(fmt.Sprintf("got %d", cfg.RetryPolicy.MaxRetries))))
	}

	subCfg := SubscriptionConfig{
		AckDeadline:      cfg.AckDeadline,
		MessageRetention: cfg.MessageRetention,
		MinRetryBackoff:  cfg.RetryPolicy.MinRetryBackoff,
		MaxRetryBackoff:  cfg.RetryPolicy.MaxRetryBackoff,
		MaxRetries:       cfg.RetryPolicy.MaxRetries,
		MaxConcurrency:   cfg.MaxConcurrency,
	}

	if cfg.Handler == nil {
		return
	}

	methodHandler := parseMethodHandler(d, cfg.Handler)
	sub := &Subscription{
		AST:           d.Call,
		File:          d.File,
		Name:          subscriptionName,
		Doc:           d.Doc,
		Topic:         topicObj,
		Cfg:           subCfg,
		Handler:       cfg.Handler,
		MethodHandler: methodHandler,
	}
	d.Pass.RegisterResource(sub)
	d.Pass.AddBind(d.File, d.Ident, sub)
}

// parseMethodHandler parses whether the subscription handler references
// a method on a type.
func parseMethodHandler(d parseutil.ReferenceInfo, handler ast.Expr) option.Option[MethodHandler] {
	var (
		none   = option.None[MethodHandler]()
		f      = d.File
		errs   = d.Pass.Errs
		parser = d.Pass.SchemaParser
	)

	// If the handler is a method handler it must be of the form:
	//  pubsub.MethodHandler(Service.Method) or pubsub.MethodHandler((*Service).Method)
	call, ok := handler.(*ast.CallExpr)
	if !ok {
		return none
	}

	qn, ok := f.Names().ResolvePkgLevelRef(call.Fun)
	if !ok || qn.PkgPath != "encore.dev/pubsub" || qn.Name != "MethodHandler" {
		return none
	} else if len(call.Args) != 1 {
		errs.Add(ErrInvalidMethodHandler.AtGoNode(call))
		return none
	}

	// The first arg must be in the form (*Service).Method or Service.Method.
	sel, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok {
		errs.Add(ErrInvalidMethodHandler.AtGoNode(call))
		return none
	}
	x := astutil.Unparen(sel.X)

	// Parse the type declaration.
	typ := parser.ParseType(f, x)
	decl, ok := schemautil.DerefNamedInfo(typ, false)
	if !ok {
		errs.Add(ErrInvalidMethodHandler.AtGoNode(call))
		return none
	}

	return option.Some(MethodHandler{
		Decl:   decl,
		Method: sel.Sel.Name,
	})
}
