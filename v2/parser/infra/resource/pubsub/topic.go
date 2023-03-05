package pubsub

import (
	"go/ast"

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
	Ident             *ast.Ident          // The identifier of the pub sub topic
	Name              string              // The unique name of the pub sub topic
	Doc               string              // The documentation on the pub sub topic
	DeliveryGuarantee DeliveryGuarantee   // What guarantees does the pub sub topic have?
	OrderingKey       string              // What field in the message type should be used to ensure First-In-First-Out (FIFO) for messages with the same key
	MessageType       *schema.TypeDeclRef // The message type of the pub sub topic
}

func (t *Topic) Kind() resource.Kind       { return resource.PubSubTopic }
func (t *Topic) DeclaredIn() *pkginfo.File { return t.File }
func (t *Topic) ASTExpr() ast.Expr         { return t.AST }
func (t *Topic) BoundTo() option.Option[pkginfo.QualifiedName] {
	return parseutil.BoundTo(t.File, t.Ident)
}

var TopicParser = &resource.Parser{
	Name: "PubSub Topic",

	RequiredImports: []paths.Pkg{"encore.dev/pubsub"},
	Run: func(p *resource.Pass) []resource.Resource {
		name := pkginfo.QualifiedName{Name: "NewTopic", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 1,
			MaxTypeArgs: 1,
			Parse:       parsePubSubTopic,
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

func parsePubSubTopic(d parseutil.ParseData) resource.Resource {
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Add(d.Call.Pos(), "pubsub.NewTopic expects 2 arguments")
		return nil
	}

	topicName := parseutil.ParseResourceName(d.Pass.Errs, "pubsub.NewTopic", "topic name",
		d.Call.Args[0], parseutil.KebabName, "")
	if topicName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	messageType, ok := schemautil.ResolveNamedStruct(d.TypeArgs[0], false)
	if !ok {
		d.Pass.Errs.Add(d.Call.Pos(), "pubsub.NewTopic message type expects a named struct type as its type argument")
		return nil
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "pubsub.TopicConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
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
			found := false
			for _, field := range str.Fields {
				if field.Name.Value == config.OrderingKey {
					found = true
					break
				}
			}

			if !found || !ast.IsExported(config.OrderingKey) {
				//p.errInSrc(srcerrors.PubSubOrderingKeyMustBeExported(p.fset, cfgLit.Expr("OrderingKey")))
				d.Pass.Errs.Addf(cfgLit.Pos("OrderingKey"), "Ordering Key must refer to an exported field in the message type")
			}
		}
	}

	return &Topic{
		AST:               d.Call,
		File:              d.File,
		Ident:             d.Ident,
		Name:              topicName,
		Doc:               d.Doc,
		DeliveryGuarantee: AtLeastOnce,
		OrderingKey:       config.OrderingKey,
		MessageType:       messageType,
	}
}
