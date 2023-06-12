package legacymeta

import (
	"fmt"
	"go/ast"
	"path"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
	"encr.dev/v2/parser/infra/pubsub"
)

func newTraceNodes(b *builder) *TraceNodes {
	return &TraceNodes{
		b:           b,
		nodes:       make(map[paths.Pkg][]*meta.TraceNode),
		middlewares: make(map[*middleware.Middleware]*meta.TraceNode),
		subs:        make(map[*pubsub.Subscription]*meta.TraceNode),
		svcStructs:  make(map[*servicestruct.ServiceStruct]*meta.TraceNode),
		endpoints:   make(map[*api.HTTPEndpoint]*meta.TraceNode),
	}
}

// TraceNodes describes the allocated [meta.TraceNode] in the app.
// The public methods are safe to use even on a nil TraceNodes.
type TraceNodes struct {
	b     *builder
	id    int32
	nodes map[paths.Pkg][]*meta.TraceNode

	authHandler *meta.TraceNode
	middlewares map[*middleware.Middleware]*meta.TraceNode
	subs        map[*pubsub.Subscription]*meta.TraceNode
	svcStructs  map[*servicestruct.ServiceStruct]*meta.TraceNode
	endpoints   map[*api.HTTPEndpoint]*meta.TraceNode
}

func (n *TraceNodes) AuthHandler() uint32 {
	if n == nil {
		return 0
	}
	return nodeID(n.authHandler)
}

func (n *TraceNodes) Middleware(mw *middleware.Middleware) uint32 {
	if n == nil {
		return 0
	}
	return nodeID(n.middlewares[mw])
}

func (n *TraceNodes) Sub(sub *pubsub.Subscription) uint32 {
	if n == nil {
		return 0
	}
	return nodeID(n.subs[sub])
}

func (n *TraceNodes) SvcStruct(svcStruct *servicestruct.ServiceStruct) uint32 {
	if n == nil {
		return 0
	}
	return nodeID(n.svcStructs[svcStruct])
}

func (n *TraceNodes) Endpoint(ep *api.HTTPEndpoint) uint32 {
	if n == nil {
		return 0
	}
	return nodeID(n.endpoints[ep])
}

func (n *TraceNodes) addAuthHandler(ah *authhandler.AuthHandler, svcName string) {
	traceNode, context := n.alloc(ah.Decl.File, ah.Decl.AST)
	traceNode.Context = &meta.TraceNode_AuthHandlerDef{
		AuthHandlerDef: &meta.AuthHandlerDefNode{
			ServiceName: svcName,
			Name:        ah.Decl.Name,
			Context:     string(context),
		},
	}
	n.authHandler = traceNode
}

func (n *TraceNodes) addServiceStruct(s *servicestruct.ServiceStruct, svcName string) {
	traceNode, context := n.alloc(s.Decl.File, s.Decl.AST)
	traceNode.Context = &meta.TraceNode_ServiceInit{
		ServiceInit: &meta.ServiceInitNode{
			ServiceName:   svcName,
			SetupFuncName: option.Map(s.Init, func(init *schema.FuncDecl) string { return init.Name }).GetOrElse(""),
			Context:       string(context),
		},
	}
	n.svcStructs[s] = traceNode
}

func (n *TraceNodes) addSub(sub *pubsub.Subscription, svcName, topicName string) {
	traceNode, context := n.alloc(sub.File, sub.AST)
	traceNode.Context = &meta.TraceNode_PubsubSubscriber{
		PubsubSubscriber: &meta.PubSubSubscriberNode{
			TopicName:      topicName,
			SubscriberName: sub.Name,
			ServiceName:    svcName,
			Context:        string(context),
		},
	}
	n.subs[sub] = traceNode
}

func (n *TraceNodes) addMiddleware(mw *middleware.Middleware) {
	relPath := n.b.relPath(mw.File.Pkg.ImportPath)
	traceNode, context := n.alloc(mw.Decl.File, mw.Decl.AST)
	traceNode.Context = &meta.TraceNode_MiddlewareDef{
		MiddlewareDef: &meta.MiddlewareDefNode{
			PkgRelPath: relPath,
			Name:       mw.Decl.Name,
			Context:    string(context),
			Target:     mw.Target.ToProto(),
		},
	}
	n.middlewares[mw] = traceNode
}

func (n *TraceNodes) addEndpoint(ep *api.HTTPEndpoint, svcName string) {
	traceNode, context := n.alloc(ep.Decl.File, ep.Decl.AST)
	traceNode.Context = &meta.TraceNode_RpcDef{
		RpcDef: &meta.RPCDefNode{
			ServiceName: svcName,
			RpcName:     ep.Name,
			Context:     string(context),
		},
	}
	n.endpoints[ep] = traceNode
}

// alloc allocates a trace node.
func (n *TraceNodes) alloc(file *pkginfo.File, node ast.Node) (traceNode *meta.TraceNode, context []byte) {
	pkgPath := file.Pkg.ImportPath
	fileRelPath := path.Join(n.b.relPath(pkgPath), file.Name)

	tokenFile := file.Token()
	startIdx := tokenFile.Offset(node.Pos())
	endIdx := tokenFile.Offset(node.End())
	context = file.Contents()[startIdx:endIdx]

	n.id++

	switch node := node.(type) {
	case *ast.CallExpr:
		traceNode = &meta.TraceNode{
			Id:       n.id,
			Filepath: fileRelPath,
			StartPos: int32(tokenFile.Offset(node.Lparen)),
			EndPos:   int32(tokenFile.Offset(node.Rparen)),
		}
	case *ast.FuncDecl:
		traceNode = &meta.TraceNode{
			Id:       n.id,
			Filepath: fileRelPath,
			StartPos: int32(tokenFile.Offset(node.Type.Pos())),
			EndPos:   int32(tokenFile.Offset(node.Type.End())),
		}
	case *ast.TypeSpec:
		traceNode = &meta.TraceNode{
			Id:       n.id,
			Filepath: fileRelPath,
			StartPos: int32(tokenFile.Offset(node.Pos())),
			EndPos:   int32(tokenFile.Offset(node.End())),
		}
	case *ast.SelectorExpr:
		traceNode = &meta.TraceNode{
			Id:       n.id,
			Filepath: fileRelPath,
			StartPos: int32(tokenFile.Offset(node.Pos())),
			EndPos:   int32(tokenFile.Offset(node.End())),
		}
	case *ast.Ident:
		traceNode = &meta.TraceNode{
			Id:       n.id,
			Filepath: fileRelPath,
			StartPos: int32(tokenFile.Offset(node.Pos())),
			EndPos:   int32(tokenFile.Offset(node.End())),
		}
	default:
		panic(fmt.Sprintf("unhandled trace expression node %T", node))
	}
	n.nodes[pkgPath] = append(n.nodes[pkgPath], traceNode)

	start := tokenFile.Pos(int(traceNode.StartPos))
	end := tokenFile.Pos(int(traceNode.EndPos))
	sPos, ePos := tokenFile.Position(start), tokenFile.Position(end)
	traceNode.SrcLineStart = int32(sPos.Line)
	traceNode.SrcLineEnd = int32(ePos.Line)
	traceNode.SrcColStart = int32(sPos.Column)
	traceNode.SrcColEnd = int32(ePos.Column)

	return
}

func (n *TraceNodes) forPkg(pkgPath paths.Pkg) []*meta.TraceNode {
	return n.nodes[pkgPath]
}

// nodeID returns the trace node id for the given node.
// If node is nil it returns 0.
func nodeID(node *meta.TraceNode) uint32 {
	if node == nil {
		return 0
	}
	return uint32(node.Id)
}
