package pubsub

import (
	"go/ast"
	"time"

	"encr.dev/v2/internal/pkginfo"
	literals2 "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil2 "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

type Subscription struct {
	File  *pkginfo.File
	Topic *Topic
	Name  string // The unique name of the pub sub subscription
	Doc   string // The documentation on the pub sub subscription
}

func (s *Subscription) Kind() resource.Kind       { return resource.PubSubSubscription }
func (s *Subscription) DeclaredIn() *pkginfo.File { return s.File }

var SubscriptionParser = &resource.Parser{
	Name:      "PubSub Subscription",
	DependsOn: []*resource.Parser{TopicParser},

	RequiredImports: []string{"encore.dev/pubsub"},
	Run: func(p *resource.Pass) []resource.Resource {
		name := pkginfo.QualifiedName{Name: "NewSubscription", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil2.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 0,
			MaxTypeArgs: 1,
			Parse:       parsePubSubSubscription,
		}

		var resources []resource.Resource
		parseutil2.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			r := parseutil2.ParseResourceCreation(p, spec, parseutil2.ReferenceData{
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

func parsePubSubSubscription(d parseutil2.ParseData) resource.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 3 {
		d.Pass.Errs.Addf(d.Call.Pos(), "%s expects 3 arguments", displayName)
		return nil
	}

	subscriptionName := parseutil2.ParseResourceName(d.Pass.Errs, displayName, "subscription name",
		d.Call.Args[1], parseutil2.KebabName, "")
	if subscriptionName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	// Parse the literal struct representing the subscription configuration
	// so we can extract the reference to the handler function
	cfgLit, ok := literals2.ParseStruct(d.Pass.Errs, d.File, "pubsub.SubscriptionConfig", d.Call.Args[2])
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

	cfg := literals2.Decode[decodedConfig](d.Pass.Errs, cfgLit)
	_ = cfg

	// TODO(andre) validate value ranges

	// TODO(andre) Handle pubsub attribute parsing

	// TODO(andre) fill in this
	return &Subscription{
		File: d.File,
		Name: subscriptionName,
		Doc:  d.Doc,
	}
}
