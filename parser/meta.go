package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"encr.dev/parser/est"
	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/errinsrc/srcerrors"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

type TraceNodes map[ast.Node]*meta.TraceNode

// ParseMeta parses app metadata.
func ParseMeta(appRevision string, appHasUncommittedChanges bool, appRoot string, app *est.Application, fset *token.FileSet) (*meta.Data, map[*est.Package]TraceNodes, error) {
	data := &meta.Data{
		ModulePath:         app.ModulePath,
		AppRevision:        appRevision,
		UncommittedChanges: appHasUncommittedChanges,
		Decls:              app.Decls,
	}
	pkgMap := make(map[string]*meta.Package)
	nodes := parceTraceNodes(app, fset)
	for _, pkg := range app.Packages {
		p := &meta.Package{
			RelPath: pkg.RelPath,
			Name:    pkg.Name,
			Doc:     pkg.Doc,
			Secrets: pkg.Secrets,
		}
		if pkg.Service != nil {
			p.ServiceName = pkg.Service.Name
		}
		tx := nodes[pkg]
		for _, n := range tx {
			p.TraceNodes = append(p.TraceNodes, n)
		}
		sort.Slice(p.TraceNodes, func(i, j int) bool { return p.TraceNodes[i].Id < p.TraceNodes[j].Id })
		data.Pkgs = append(data.Pkgs, p)
		pkgMap[p.RelPath] = p
	}

	for _, svc := range app.Services {
		s, err := parseSvc(appRoot, svc)
		if err != nil {
			return nil, nil, err
		}
		data.Svcs = append(data.Svcs, s)
	}

	// Populate rpc calls based on rewrite information
	for _, pkg := range app.Packages {
		type key struct {
			pkg string
			rpc string
		}
		seen := make(map[key]bool)
		for _, f := range pkg.Files {
			for _, r := range sortedRefs(f.References) {
				if r.Node.Type == est.RPCRefNode {
					k := key{pkg: r.Node.RPC.Svc.Root.RelPath, rpc: r.Node.RPC.Name}
					if !seen[k] {
						p := pkgMap[pkg.RelPath]
						p.RpcCalls = append(p.RpcCalls, &meta.QualifiedName{
							Pkg:  k.pkg,
							Name: k.rpc,
						})
						seen[k] = true
					}
				}
			}
		}
	}

	for _, job := range app.CronJobs {
		cj, err := parseCronJob(job)
		if err != nil {
			return nil, nil, err
		}
		data.CronJobs = append(data.CronJobs, cj)
	}

	selectors := app.SelectorLookup()
	for _, topic := range app.PubSubTopics {
		t := parsePubsubTopic(topic, selectors)
		data.PubsubTopics = append(data.PubsubTopics, t)
	}

	for _, cluster := range app.CacheClusters {
		cc := parseCacheCluster(cluster)
		data.CacheClusters = append(data.CacheClusters, cc)
	}

	if app.AuthHandler != nil {
		data.AuthHandler = parseAuthHandler(app.AuthHandler)
	}

	for _, mw := range app.Middleware {
		data.Middleware = append(data.Middleware, parseMiddleware(mw))
	}

	return data, nodes, nil
}

func parsePubsubTopic(topic *est.PubSubTopic, selectors *est.SelectorLookup) *meta.PubSubTopic {
	parsePublisher := func(pubs ...*est.PubSubPublisher) (rtn []*meta.PubSubTopic_Publisher) {
		for _, p := range pubs {
			switch {
			case p.Service != nil:
				rtn = append(rtn, &meta.PubSubTopic_Publisher{ServiceName: p.Service.Name})
			case p.GlobalMiddleware != nil:
				for _, svc := range selectors.GetServices(p.GlobalMiddleware.Target) {
					rtn = append(rtn, &meta.PubSubTopic_Publisher{ServiceName: svc.Name})
				}
			default:
				panic("impossible publish without a service or middleware reference")
			}
		}
		return rtn
	}
	parseSubscribers := func(subs ...*est.PubSubSubscriber) (rtn []*meta.PubSubTopic_Subscription) {
		for _, s := range subs {
			rtn = append(rtn, &meta.PubSubTopic_Subscription{
				Name:             s.Name,
				ServiceName:      s.DeclFile.Pkg.Service.Name,
				AckDeadline:      int64(s.AckDeadline),
				MessageRetention: int64(s.MessageRetention),
				RetryPolicy: &meta.PubSubTopic_RetryPolicy{
					MinBackoff: int64(s.MinRetryBackoff),
					MaxBackoff: int64(s.MaxRetryBackoff),
					MaxRetries: s.MaxRetries,
				},
			})
		}
		return rtn
	}
	return &meta.PubSubTopic{
		Name:              topic.Name,
		Doc:               topic.Doc,
		MessageType:       topic.MessageType.Type,
		DeliveryGuarantee: meta.PubSubTopic_DeliveryGuarantee(topic.DeliveryGuarantee),
		OrderingKey:       topic.OrderingKey,
		Publishers:        parsePublisher(topic.Publishers...),
		Subscriptions:     parseSubscribers(topic.Subscribers...),
	}
}

var migrationRe = regexp.MustCompile(`^(\d+)_([^.]+)\.(up|down).sql$`)

func parseSvc(appRoot string, svc *est.Service) (*meta.Service, error) {
	s := &meta.Service{
		Name:      svc.Name,
		RelPath:   svc.Root.RelPath,
		HasConfig: len(svc.ConfigLoads) > 0,
	}
	for _, rpc := range svc.RPCs {
		r, err := parseRPC(rpc)
		if err != nil {
			return nil, err
		}
		s.Rpcs = append(s.Rpcs, r)
	}

	relPath := filepath.Join(svc.Root.RelPath, "migrations")
	migs, err := parseMigrations(appRoot, relPath)
	if err != nil {
		return nil, fmt.Errorf("%s: could not parse sqldb migrations: %v", svc.Root.RelPath, err)
	}
	s.Migrations = migs

	// Compute which databases this connects to
	seenDBs := make(map[string]bool)
	if len(migs) > 0 {
		seenDBs[svc.Name] = true
	}
	for _, pkg := range svc.Pkgs {
		for _, res := range pkg.Resources {
			if res.Type() == est.SQLDBResource {
				name := res.(*est.SQLDB).DBName
				seenDBs[name] = true
			}
		}
	}
	var dbNames []string
	for name := range seenDBs {
		dbNames = append(dbNames, name)
	}
	sort.Strings(dbNames)
	s.Databases = dbNames

	return s, nil
}

func parseRPC(rpc *est.RPC) (*meta.RPC, error) {
	proto := meta.RPC_REGULAR
	if rpc.Raw {
		proto = meta.RPC_RAW
	}
	var accessType meta.RPC_AccessType
	switch rpc.Access {
	case est.Public:
		accessType = meta.RPC_PUBLIC
	case est.Private:
		accessType = meta.RPC_PRIVATE
	case est.Auth:
		accessType = meta.RPC_AUTH
	default:
		return nil, fmt.Errorf("unhandled access type %v", rpc.Access)
	}

	var req, resp *schema.Type
	if rpc.Request != nil {
		req = rpc.Request.Type
	}
	if rpc.Response != nil {
		resp = rpc.Response.Type
	}
	r := &meta.RPC{
		Name:           rpc.Name,
		ServiceName:    rpc.Svc.Name,
		Doc:            rpc.Doc,
		AccessType:     accessType,
		RequestSchema:  req,
		ResponseSchema: resp,
		Proto:          proto,
		Loc:            parseLoc(rpc.File, rpc.Func),
		Path:           rpc.Path.ToProto(),
		HttpMethods:    rpc.HTTPMethods,
		Tags:           rpc.Tags.ToProto(),
	}
	return r, nil
}

func parseCronJob(job *est.CronJob) (*meta.CronJob, error) {
	j := &meta.CronJob{
		Id:       job.ID,
		Title:    job.Title,
		Doc:      job.Doc,
		Schedule: job.Schedule,
		Endpoint: &meta.QualifiedName{
			Name: job.RPC.Name,
			Pkg:  job.RPC.Svc.Root.RelPath,
		},
	}
	return j, nil
}

func parseCacheCluster(cluster *est.CacheCluster) *meta.CacheCluster {
	parseKeyspaces := func(keyspaces ...*est.CacheKeyspace) (rtn []*meta.CacheCluster_Keyspace) {
		for range keyspaces {
			// TODO implement
			rtn = append(rtn, &meta.CacheCluster_Keyspace{})
		}
		return rtn
	}

	return &meta.CacheCluster{
		Name:           cluster.Name,
		Doc:            cluster.Doc,
		EvictionPolicy: cluster.EvictionPolicy,
		Keyspaces:      parseKeyspaces(cluster.Keyspaces...),
	}
}

func parseMigrations(appRoot, relPath string) ([]*meta.DBMigration, error) {
	absPath := filepath.Join(appRoot, relPath)
	fi, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", relPath)
	}

	files, err := ioutil.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("could not read migrations: %v", err)
	}
	migrations := make([]*meta.DBMigration, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		match := migrationRe.FindStringSubmatch(f.Name())
		if match == nil {
			return nil, fmt.Errorf("migration %s/%s has an invalid name (must be of the format '[123]_[description].[up|down].sql')",
				relPath, f.Name())
		}
		num, _ := strconv.Atoi(match[1])
		if match[3] == "up" {
			migrations = append(migrations, &meta.DBMigration{
				Filename:    f.Name(),
				Number:      int32(num),
				Description: match[2],
			})
		}
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Number < migrations[j].Number
	})
	for i := int32(0); i < int32(len(migrations)); i++ {
		fn := migrations[i].Filename
		num := migrations[i].Number
		if num <= 0 {
			return nil, fmt.Errorf("%s/%s: invalid migration number %d", relPath, fn, num)
		} else if num < (i + 1) {
			return nil, fmt.Errorf("%s/%s: duplicate migration with number %d", relPath, fn, num)
		} else if num > (i + 1) {
			return nil, fmt.Errorf("%s/%s: missing migration with number %d", relPath, fn, i+1)
		}
	}
	return migrations, nil
}

func parseAuthHandler(h *est.AuthHandler) *meta.AuthHandler {
	pb := &meta.AuthHandler{
		Name:    h.Name,
		Doc:     h.Name,
		PkgPath: h.Svc.Root.ImportPath,
		PkgName: h.Svc.Root.Name,
		Loc:     parseLoc(h.File, h.Func),
		Params:  h.Params,
	}
	if h.AuthData != nil {
		pb.AuthData = h.AuthData.Type
	}
	return pb
}

func parseMiddleware(mw *est.Middleware) *meta.Middleware {
	pb := &meta.Middleware{
		Name:   &meta.QualifiedName{Pkg: mw.Pkg.RelPath, Name: mw.Name},
		Doc:    mw.Doc,
		Loc:    parseLoc(mw.File, mw.Func),
		Global: mw.Global,
		Target: mw.Target.ToProto(),
	}
	if mw.Svc != nil {
		pb.ServiceName = &mw.Svc.Name
	}
	return pb
}

func parceTraceNodes(app *est.Application, fset *token.FileSet) map[*est.Package]TraceNodes {
	var lastReference *refPair

	defer func() {
		if err := recover(); err != nil {
			err := srcerrors.UnhandledPanic(err)
			errinsrc.AddHintFromGo(err, fset, lastReference.AST, fmt.Sprintf("paniced on this node while processing: %+v", lastReference.Node))
			panic(err)
		}
	}()

	var id int32
	res := make(map[*est.Package]TraceNodes)
	for _, pkg := range app.Packages {
		nodes := make(TraceNodes)
		res[pkg] = nodes
		for _, file := range pkg.Files {

			for _, r := range sortedRefs(file.References) {
				lastReference = &r

				file := file

				switch r.Node.Type {
				// Secret nodes are not relevant for tracing
				case est.SecretsNode, est.PubSubTopicDefNode:
					continue
				case est.PubSubSubscriberNode:
					// Subscribers can be declared in a different file than the reference
					file = r.Node.Res.(*est.PubSubSubscriber).DeclFile
				}

				tx := newTraceNode(&id, pkg, file, r.AST)
				nodes[r.AST] = tx
				start := file.Token.Offset(r.AST.Pos())
				end := file.Token.Offset(r.AST.End())

				switch r.Node.Type {
				case est.RPCRefNode:
					tx.Context = &meta.TraceNode_RpcCall{
						RpcCall: &meta.RPCCallNode{
							ServiceName: r.Node.RPC.Svc.Name,
							RpcName:     r.Node.RPC.Name,
							Context:     string(file.Contents[start:end]),
						},
					}

				case est.PubSubPublisherNode:
					tx.Context = &meta.TraceNode_PubsubPublish{
						PubsubPublish: &meta.PubSubPublishNode{
							TopicName: r.Node.Res.(*est.PubSubTopic).Name,
							Context:   string(file.Contents[start:end]),
						},
					}

				case est.PubSubSubscriberNode:
					sub := r.Node.Res.(*est.PubSubSubscriber)
					tx.Context = &meta.TraceNode_PubsubSubscriber{
						PubsubSubscriber: &meta.PubSubSubscriberNode{
							TopicName:      sub.Topic.Name,
							SubscriberName: sub.Name,
							ServiceName:    sub.DeclFile.Pkg.Service.Name,
							Context:        string(file.Contents[start:end]),
						},
					}

				case est.CacheKeyspaceDefNode:
					ks := r.Node.Res.(*est.CacheKeyspace)
					tx.Context = &meta.TraceNode_CacheKeyspace{
						CacheKeyspace: &meta.CacheKeyspaceDefNode{
							PkgRelPath:  pkg.RelPath,
							VarName:     ks.Ident().Name,
							ClusterName: ks.Cluster.Name,
							Context:     string(file.Contents[start:end]),
						},
					}
				}
			}
		}
	}
	lastReference = nil

	for _, svc := range app.Services {
		for _, rpc := range svc.RPCs {
			fd := rpc.Func
			tx := newTraceNode(&id, rpc.Svc.Root, rpc.File, fd)
			res[svc.Root][fd] = tx
			f := rpc.File
			start := f.Token.Offset(fd.Type.Pos())
			end := f.Token.Offset(fd.Type.End())
			tx.Context = &meta.TraceNode_RpcDef{
				RpcDef: &meta.RPCDefNode{
					ServiceName: svc.Name,
					RpcName:     fd.Name.Name,
					Context:     string(f.Contents[start:end]),
				},
			}
		}

		if ss := svc.Struct; ss != nil && ss.Init != nil {
			fd := ss.Init
			f := ss.InitFile
			nod := newTraceNode(&id, svc.Root, f, ss.Init)
			res[svc.Root][fd] = nod

			start := f.Token.Offset(fd.Type.Pos())
			end := f.Token.Offset(fd.Type.End())
			nod.Context = &meta.TraceNode_ServiceInit{
				ServiceInit: &meta.ServiceInitNode{
					ServiceName:   svc.Name,
					SetupFuncName: ss.Init.Name.Name,
					Context:       string(f.Contents[start:end]),
				},
			}
		}
	}

	if h := app.AuthHandler; h != nil {
		fd := h.Func
		tx := newTraceNode(&id, h.Svc.Root, h.File, fd)
		res[h.Svc.Root][fd] = tx
		f := h.File
		start := f.Token.Offset(fd.Type.Pos())
		end := f.Token.Offset(fd.Type.End())
		tx.Context = &meta.TraceNode_AuthHandlerDef{
			AuthHandlerDef: &meta.AuthHandlerDefNode{
				ServiceName: h.Svc.Name,
				Name:        fd.Name.Name,
				Context:     string(f.Contents[start:end]),
			},
		}
	}

	for _, mw := range app.Middleware {
		fd := mw.Func
		f := mw.File
		tx := newTraceNode(&id, mw.Pkg, f, fd)
		res[mw.Pkg][fd] = tx
		start := f.Token.Offset(fd.Type.Pos())
		end := f.Token.Offset(fd.Type.End())
		tx.Context = &meta.TraceNode_MiddlewareDef{
			MiddlewareDef: &meta.MiddlewareDefNode{
				PkgRelPath: mw.Pkg.RelPath,
				Name:       fd.Name.Name,
				Context:    string(f.Contents[start:end]),
				Target:     mw.Target.ToProto(),
			},
		}
	}

	return res
}

func newTraceNode(id *int32, pkg *est.Package, f *est.File, node ast.Node) *meta.TraceNode {
	*id++
	var expr *meta.TraceNode
	file := f.Token
	filename := filepath.Base(f.Path)
	nodeFilePath := path.Join(pkg.RelPath, filename)

	switch node := node.(type) {
	case *ast.CallExpr:
		expr = &meta.TraceNode{
			Id:       *id,
			Filepath: nodeFilePath,
			StartPos: int32(file.Offset(node.Lparen)),
			EndPos:   int32(file.Offset(node.Rparen)),
		}
	case *ast.FuncDecl:
		expr = &meta.TraceNode{
			Id:       *id,
			Filepath: nodeFilePath,
			StartPos: int32(file.Offset(node.Type.Pos())),
			EndPos:   int32(file.Offset(node.Type.End())),
		}
	case *ast.SelectorExpr:
		expr = &meta.TraceNode{
			Id:       *id,
			Filepath: nodeFilePath,
			StartPos: int32(file.Offset(node.Pos())),
			EndPos:   int32(file.Offset(node.End())),
		}
	case *ast.Ident:
		expr = &meta.TraceNode{
			Id:       *id,
			Filepath: nodeFilePath,
			StartPos: int32(file.Offset(node.Pos())),
			EndPos:   int32(file.Offset(node.End())),
		}
	default:
		panic(fmt.Sprintf("unhandled trace expression node %T", node))
	}

	start := file.Pos(int(expr.StartPos))
	end := file.Pos(int(expr.EndPos))
	sPos, ePos := file.Position(start), file.Position(end)
	expr.SrcLineStart = int32(sPos.Line)
	expr.SrcLineEnd = int32(ePos.Line)
	expr.SrcColStart = int32(sPos.Column)
	expr.SrcColEnd = int32(ePos.Column)
	return expr
}

func parseLoc(f *est.File, node ast.Node) *schema.Loc {
	sPos, ePos := f.Token.Position(node.Pos()), f.Token.Position(node.Pos())
	return &schema.Loc{
		PkgName:      f.Pkg.Name,
		PkgPath:      f.Pkg.ImportPath,
		Filename:     f.Name,
		StartPos:     int32(f.Token.Offset(node.Pos())),
		EndPos:       int32(f.Token.Offset(node.End())),
		SrcLineStart: int32(sPos.Line),
		SrcLineEnd:   int32(ePos.Line),
		SrcColStart:  int32(sPos.Column),
		SrcColEnd:    int32(ePos.Column),
	}
}

type refPair struct {
	AST  ast.Node
	Node *est.Node
}

func sortedRefs(refs map[ast.Node]*est.Node) []refPair {
	p := make([]refPair, 0, len(refs))
	for astNode, estNode := range refs {
		p = append(p, refPair{astNode, estNode})
	}
	sort.Slice(p, func(i, j int) bool {
		return p[i].AST.Pos() < p[j].AST.Pos()
	})
	return p
}
