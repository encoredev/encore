package pubsub

import (
	"fmt"
	"go/ast"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	literals "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

type DeliveryGuarantee int

const (
	AtLeastOnce DeliveryGuarantee = iota
	ExactlyOnce
)

type Topic struct {
	AST               *ast.CallExpr
	File              *pkginfo.File
	Name              string              // The unique name of the pub sub topic
	Doc               string              // The documentation on the pub sub topic
	DeliveryGuarantee DeliveryGuarantee   // What guarantees does the pub sub topic have?
	OrderingKey       string              // What field in the message type should be used to ensure First-In-First-Out (FIFO) for messages with the same key
	MessageType       *schema.TypeDeclRef // The message type of the pub sub topic
}

func (t *Topic) Kind() resource.Kind       { return resource.PubSubTopic }
func (t *Topic) Package() *pkginfo.Package { return t.File.Pkg }
func (t *Topic) ASTExpr() ast.Expr         { return t.AST }
func (t *Topic) ResourceName() string      { return t.Name }

var TopicParser = &resource.Parser{
	Name: "PubSub Topic",

	InterestingImports: []paths.Pkg{"encore.dev/pubsub"},
	Run: func(p *resource.Pass) {
		name := pkginfo.QualifiedName{Name: "NewTopic", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil.ReferenceSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 1,
			MaxTypeArgs: 1,
			Parse:       parsePubSubTopic,
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

func parsePubSubTopic(d parseutil.ReferenceInfo) {
	errs := d.Pass.Errs

	if len(d.Call.Args) != 2 {
		errs.Add(errNewTopicArgCount(len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	topicName := parseutil.ParseResourceName(d.Pass.Errs, "pubsub.NewTopic", "topic name",
		d.Call.Args[0], parseutil.KebabName, "")
	if topicName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	messageType, ok := schemautil.ResolveNamedStruct(d.TypeArgs[0], false)
	if !ok {
		errs.Add(errInvalidMessageType.AtGoNode(d.TypeArgs[0].ASTExpr(), errors.AsError(fmt.Sprintf("got %s", parseutil.NodeType(d.TypeArgs[0].ASTExpr())))))
		return
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "pubsub.TopicConfig", d.Call.Args[1])
	if !ok {
		return // error reported by ParseStruct
	}

	// Decode the config
	type decodedConfig struct {
		DeliveryGuarantee int    `literal:",required"`
		OrderingKey       string `literal:",optional"`
	}
	config := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit)

	// Get the ordering key
	if config.OrderingKey != "" {
		// Make sure the OrderingKey value exists in the struct.
		if str, ok := messageType.Decl.Type.(schema.StructType); ok {
			var foundField ast.Node
			for _, field := range str.Fields {
				if option.Contains(field.Name, config.OrderingKey) {
					foundField = field.AST
					break
				}
			}

			if foundField == nil || !ast.IsExported(config.OrderingKey) {
				if foundField == nil {
					foundField = cfgLit.Expr("OrderingKey")
				}
				errs.Add(errOrderingKeyNotExported.AtGoNode(foundField))
			}
		}
	}

	topic := &Topic{
		AST:               d.Call,
		File:              d.File,
		Name:              topicName,
		Doc:               d.Doc,
		DeliveryGuarantee: AtLeastOnce,
		OrderingKey:       config.OrderingKey,
		MessageType:       messageType,
	}
	d.Pass.RegisterResource(topic)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, topic)
	}
}
