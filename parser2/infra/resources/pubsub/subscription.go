package pubsub

import (
	"go/ast"
	"time"

	"encr.dev/parser2/infra/internal/literals"
	"encr.dev/parser2/infra/internal/locations"
	"encr.dev/parser2/infra/internal/parseutil"
	"encr.dev/parser2/infra/resources"
	"encr.dev/parser2/internal/pkginfo"
)

type Subscription struct {
	Topic *Topic
	Name  string // The unique name of the pub sub subscription
	Doc   string // The documentation on the pub sub subscription
}

func (t *Subscription) Kind() resources.Kind { return resources.PubSubSubscription }

var SubscriptionParser = &resources.Parser{
	Name:      "PubSub Subscription",
	DependsOn: []*resources.Parser{TopicParser},

	RequiredImports: []string{"encore.dev/pubsub"},
	Run: func(p *resources.Pass) {
		name := pkginfo.QualifiedName{Name: "NewSubscription", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 1,
			Parse:       parsePubSubSubscription,
		}

		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parsePubSubSubscription(d parseutil.ParseData) resources.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 3 {
		d.Pass.Errs.Addf(d.Call.Pos(), "%s expects 3 arguments", displayName)
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
		Name: subscriptionName,
		Doc:  d.Doc,
	}
}
