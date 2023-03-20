package legacymeta

import (
	"fmt"
	gotoken "go/token"

	"golang.org/x/exp/slices"

	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/app"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/resourcepaths"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/infra/caches"
	"encr.dev/v2/parser/infra/config"
	"encr.dev/v2/parser/infra/crons"
	"encr.dev/v2/parser/infra/metrics"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/infra/secrets"
	"encr.dev/v2/parser/infra/sqldb"
	"encr.dev/v2/parser/resource"
)

type builder struct {
	errs *perr.List
	app  *app.Desc
	md   *meta.Data // metadata being generated

	decls map[declKey]uint32
}

func Compute(errs *perr.List, appDesc *app.Desc) *meta.Data {
	b := &builder{
		errs:  errs,
		app:   appDesc,
		decls: make(map[declKey]uint32),
	}
	return b.Build()
}

func (b *builder) Build() *meta.Data {
	// TODO(andre) We assume the framework is used for now.
	// When we add support for not using the framework we'll need
	// to handle this differently.

	b.md = &meta.Data{
		ModulePath:         string(b.app.MainModule.Path),
		AppRevision:        b.app.BuildInfo.Revision,
		UncommittedChanges: b.app.BuildInfo.UncommittedChanges,
	}
	md := b.md

	relPath := func(pkg paths.Pkg) string {
		rel, ok := b.app.MainModule.Path.RelativePathToPkg(pkg)
		if !ok {
			panic("cannot compute relative path to package outside main module: " + pkg.String())
		}
		return rel
	}

	svcByName := make(map[string]*meta.Service, len(b.app.Services))
	for _, svc := range b.app.Services {
		out := &meta.Service{
			Name: svc.Name,
		}
		svcByName[svc.Name] = out
		md.Svcs = append(md.Svcs, out)

		if fw, ok := svc.Framework.Get(); ok {
			out.RelPath = relPath(fw.RootPkg.ImportPath)
			for _, ep := range fw.Endpoints {
				rpc := &meta.RPC{
					Name:           ep.Name,
					Doc:            ep.Doc,
					ServiceName:    svc.Name,
					RequestSchema:  b.schemaType(ep.Request),
					ResponseSchema: b.schemaType(ep.Response),
					Proto:          meta.RPC_REGULAR,
					Loc:            nil, // TODO
					Path:           b.apiPath(ep.Decl.AST.Pos(), ep.Path),
					HttpMethods:    ep.HTTPMethods,
					Tags:           ep.Tags.ToProto(),
				}
				if ep.Raw {
					rpc.Proto = meta.RPC_RAW
				}

				switch ep.Access {
				case api.Public:
					rpc.AccessType = meta.RPC_PUBLIC
				case api.Private:
					rpc.AccessType = meta.RPC_PRIVATE
				case api.Auth:
					rpc.AccessType = meta.RPC_AUTH
				default:
					b.errs.Addf(ep.Decl.AST.Pos(), "internal error: unknown API access type %v", ep.Access)
				}

				out.Rpcs = append(out.Rpcs, rpc)
			}

			// Do we have a database associated with the service?
			for res := range svc.ResourceUsage {
				switch res := res.(type) {
				case *sqldb.Database:
					out.Databases = append(out.Databases, res.Name)
					// If the database name is the same as the service,
					// it's the database defined by said service.
					if res.Name == svc.Name {
						for _, mig := range res.Migrations {
							out.Migrations = append(out.Migrations, &meta.DBMigration{
								Filename:    mig.Filename,
								Number:      int32(mig.Number),
								Description: mig.Description,
							})
						}
					}
				}
			}

		}
	}

	appPackages := b.app.Parse.AppPackages()
	pkgByPath := make(map[paths.Pkg]*meta.Package, len(appPackages))
	for _, pkg := range appPackages {
		metaPkg := &meta.Package{
			RelPath:     relPath(pkg.ImportPath),
			Name:        pkg.Name,
			Doc:         pkg.Doc,
			ServiceName: "",
			Secrets:     nil,
			RpcCalls:    nil,
			TraceNodes:  nil,
		}
		pkgByPath[pkg.ImportPath] = metaPkg

		if svc, ok := b.app.ServiceForPath(pkg.FSPath); ok {
			metaPkg.ServiceName = svc.Name
		}

		// Don't add main packages to the list of packages.
		// Still track it in the map since other resources
		// may depend on the package being known.
		if pkg.Name != "main" {
			md.Pkgs = append(md.Pkgs, metaPkg)
		}

		seenRPCCalls := make(map[pkginfo.QualifiedName]bool)
		addRPCCall := func(ep *api.Endpoint) {
			pkg := ep.Package()
			qn := pkginfo.Q(pkg.ImportPath, ep.Name)
			if !seenRPCCalls[qn] {
				seenRPCCalls[qn] = true
				metaPkg.RpcCalls = append(metaPkg.RpcCalls, &meta.QualifiedName{
					Pkg:  relPath(pkg.ImportPath),
					Name: ep.Name,
				})
			}
		}

		for _, u := range b.app.Parse.UsagesInPkg(pkg.ImportPath) {
			switch u := u.(type) {
			case *api.CallUsage:
				addRPCCall(u.Endpoint)
			case *api.ReferenceUsage:
				// NOTE: The legacy meta does not distinguish between calls and references,
				// and adds both to the list of RPC calls. Replicate this behavior.
				addRPCCall(u.Endpoint)
			}
		}
	}

	// Keep track of state needed for dependent resources.
	var (
		// dependent are the resources that depend on other resources.
		// They're processed in a second pass.
		dependent []resource.Resource

		topicMap   = make(map[pkginfo.QualifiedName]*meta.PubSubTopic)
		clusterMap = make(map[pkginfo.QualifiedName]*meta.CacheCluster)
	)

	selectorLookup := computeSelectorLookup(b.app)
	for _, r := range b.app.Parse.Resources() {
		switch r := r.(type) {
		case *crons.Job:
			cj := &meta.CronJob{
				Id:       r.Name,
				Title:    r.Title,
				Doc:      r.Doc,
				Schedule: r.Schedule,
				Endpoint: nil,
			}
			md.CronJobs = append(md.CronJobs, cj)
			if ep, ok := b.app.Parse.ResourceForQN(r.Endpoint).Get(); ok {
				cj.Endpoint = &meta.QualifiedName{
					Pkg:  relPath(ep.Package().ImportPath),
					Name: ep.(*api.Endpoint).Name,
				}
			} else {
				b.errs.Addf(r.EndpointAST.Pos(), "could not find endpoint %q", r.Endpoint)
			}

		case *authhandler.AuthHandler:
			ah := &meta.AuthHandler{
				Name:    r.Name,
				Doc:     r.Doc,
				PkgPath: r.Package().ImportPath.String(),
				PkgName: r.Package().Name,
				Loc:     nil,
				Params:  b.schemaType(r.Param),
			}
			if data, ok := r.AuthData.Get(); ok {
				ah.AuthData = b.typeDeclRef(data)
			}
			md.AuthHandler = ah

		case *pubsub.Topic:
			topic := &meta.PubSubTopic{
				Name:          r.Name,
				Doc:           r.Doc,
				MessageType:   b.typeDeclRef(r.MessageType),
				OrderingKey:   r.OrderingKey,
				Publishers:    nil,
				Subscriptions: nil, // filled in later
			}

			seenPublishers := make(map[string]bool)
			addPublisher := func(svcName string) {
				if !seenPublishers[svcName] {
					seenPublishers[svcName] = true
					topic.Publishers = append(topic.Publishers, &meta.PubSubTopic_Publisher{
						ServiceName: svcName,
					})
				}
			}

			// Find all the publishers
			for _, u := range b.app.Parse.Usages(r) {
				if _, ok := u.(*pubsub.PublishUsage); ok {
					if svc, ok := b.app.ServiceForPath(u.DeclaredIn().FSPath); ok {
						// Is the publish call within a service? If so add that service as the publisher.
						addPublisher(svc.Name)
					} else if res2, ok := b.app.Parse.ResourceConstructorContaining(u).Get(); ok {
						// Otherwise, is the publish call within a global middleware?
						// If so add all services that that middleware applies to.
						switch res2 := res2.(type) {
						case *middleware.Middleware:
							if res2.Global {
								for _, svc := range selectorLookup.GetServices(res2.Target) {
									addPublisher(svc.Name)
								}
							}
						}
					}
				}
			}

			// Sort the publishers
			slices.SortFunc(topic.Publishers, func(a, b *meta.PubSubTopic_Publisher) bool {
				return a.ServiceName < b.ServiceName
			})

			switch r.DeliveryGuarantee {
			case pubsub.ExactlyOnce:
				topic.DeliveryGuarantee = meta.PubSubTopic_EXACTLY_ONCE
			case pubsub.AtLeastOnce:
				topic.DeliveryGuarantee = meta.PubSubTopic_AT_LEAST_ONCE
			default:
				panic(fmt.Sprintf("unknown delivery guarantee %v", r.DeliveryGuarantee))
			}

			for _, b := range b.app.Parse.PkgDeclBinds(r) {
				topicMap[b.QualifiedName()] = topic
			}
			md.PubsubTopics = append(md.PubsubTopics, topic)

		case *caches.Cluster:
			cluster := &meta.CacheCluster{
				Name:           r.Name,
				Doc:            r.Doc,
				Keyspaces:      nil,
				EvictionPolicy: r.EvictionPolicy,
			}
			for _, b := range b.app.Parse.PkgDeclBinds(r) {
				clusterMap[b.QualifiedName()] = cluster
			}
			md.CacheClusters = append(md.CacheClusters, cluster)

		case *metrics.Metric:
			var svcName *string
			if svc, ok := b.app.ServiceForPath(r.File.Pkg.FSPath); ok {
				svcName = &svc.Name
			}

			m := &meta.Metric{
				Name:        r.Name,
				ValueType:   b.builtinType(r.ValueType),
				Doc:         r.Doc,
				ServiceName: svcName,
				Labels:      nil, // TODO
			}
			switch r.Type {
			case metrics.Counter:
				m.Kind = meta.Metric_COUNTER
			case metrics.Gauge:
				m.Kind = meta.Metric_GAUGE
			default:
				panic(fmt.Sprintf("unknown metric type %v", r.Type))
			}

			md.Metrics = append(md.Metrics, m)

		case *config.Load:
			if svc, ok := b.app.ServiceForPath(r.File.Pkg.FSPath); ok {
				if metaSvc, ok := svcByName[svc.Name]; ok {
					metaSvc.HasConfig = true
				}
			}
			// Register the types.
			b.schemaType(r.Type)

		case *secrets.Secrets:
			pkg, ok := pkgByPath[r.Package().ImportPath]
			if !ok {
				b.errs.Addf(r.ASTExpr().Pos(), "could not find package %q", r.Package().ImportPath)
				continue
			}
			pkg.Secrets = append(pkg.Secrets, r.Keys...)

		case *middleware.Middleware:
			mw := &meta.Middleware{
				Name: &meta.QualifiedName{
					Pkg:  relPath(r.Package().ImportPath),
					Name: r.Decl.Name,
				},
				Doc:         r.Doc,
				Loc:         nil,
				Global:      r.Global,
				ServiceName: nil,
				Target:      r.Target.ToProto(),
			}
			md.Middleware = append(md.Middleware, mw)
			if svc, ok := b.app.ServiceForPath(r.File.Pkg.FSPath); ok {
				mw.ServiceName = &svc.Name
			}

		case *pubsub.Subscription, *caches.Keyspace:
			dependent = append(dependent, r)
		}
	}

	// Make a second pass for resources that depend on other resources.
	for _, r := range dependent {
		switch r := r.(type) {
		case *pubsub.Subscription:
			topic, ok := topicMap[r.Topic]
			if !ok {
				b.errs.Addf(r.ASTExpr().Pos(), "topic %q not found",
					r.Topic.NaiveDisplayName())
				continue
			}

			svc, ok := b.app.ServiceForPath(r.File.Pkg.FSPath)
			if !ok {
				b.errs.Addf(r.ASTExpr().Pos(), "pubsub subscription %q must be defined within a service",
					r.Name)
				continue
			}

			topic.Subscriptions = append(topic.Subscriptions, &meta.PubSubTopic_Subscription{
				Name:             r.Name,
				ServiceName:      svc.Name,
				AckDeadline:      r.Cfg.AckDeadline.Nanoseconds(),
				MessageRetention: r.Cfg.MessageRetention.Nanoseconds(),
				RetryPolicy: &meta.PubSubTopic_RetryPolicy{
					MinBackoff: r.Cfg.MinRetryBackoff.Nanoseconds(),
					MaxBackoff: r.Cfg.MaxRetryBackoff.Nanoseconds(),
					MaxRetries: int64(r.Cfg.MaxRetries),
				},
			})

		case *caches.Keyspace:
			cluster, ok := clusterMap[r.Cluster]
			if !ok {
				b.errs.Addf(r.ASTExpr().Pos(), "cluster %q not found",
					r.Cluster.NaiveDisplayName())
				continue
			}

			svc, ok := b.app.ServiceForPath(r.File.Pkg.FSPath)
			if !ok {
				b.errs.Addf(r.ASTExpr().Pos(), "cache keyspace must be defined within a service")
				continue
			}

			cluster.Keyspaces = append(cluster.Keyspaces, &meta.CacheCluster_Keyspace{
				Service:     svc.Name,
				KeyType:     b.schemaType(r.KeyType),
				ValueType:   b.schemaType(r.ValueType),
				PathPattern: b.keyspacePath(r.Path),
				Doc:         r.Doc,
			})
		}
	}

	return md
}

func (b *builder) apiPath(pos gotoken.Pos, path *resourcepaths.Path) *meta.Path {
	res := &meta.Path{
		Type: meta.Path_URL,
	}
	for _, p := range path.Segments {
		seg := &meta.PathSegment{
			Value: p.Value,
		}

		switch p.ValueType {
		case schema.String:
			seg.ValueType = meta.PathSegment_STRING
		case schema.Bool:
			seg.ValueType = meta.PathSegment_BOOL
		case schema.Int8:
			seg.ValueType = meta.PathSegment_INT8
		case schema.Int16:
			seg.ValueType = meta.PathSegment_INT16
		case schema.Int32:
			seg.ValueType = meta.PathSegment_INT32
		case schema.Int64:
			seg.ValueType = meta.PathSegment_INT64
		case schema.Int:
			seg.ValueType = meta.PathSegment_INT
		case schema.Uint8:
			seg.ValueType = meta.PathSegment_UINT8
		case schema.Uint16:
			seg.ValueType = meta.PathSegment_UINT16
		case schema.Uint32:
			seg.ValueType = meta.PathSegment_UINT32
		case schema.Uint64:
			seg.ValueType = meta.PathSegment_UINT64
		case schema.Uint:
			seg.ValueType = meta.PathSegment_UINT
		case schema.UUID:
			seg.ValueType = meta.PathSegment_UUID
		default:
			b.errs.Addf(pos, "internal error: unknown path segment value type %v", p.ValueType)
		}

		switch p.Type {
		case resourcepaths.Literal:
			seg.Type = meta.PathSegment_LITERAL
		case resourcepaths.Param:
			seg.Type = meta.PathSegment_PARAM
		case resourcepaths.Wildcard:
			seg.Type = meta.PathSegment_WILDCARD
		}

		res.Segments = append(res.Segments, seg)
	}
	return res
}

func (b *builder) keyspacePath(path *resourcepaths.Path) *meta.Path {
	res := &meta.Path{
		Type: meta.Path_CACHE_KEYSPACE,
	}
	for _, p := range path.Segments {
		seg := &meta.PathSegment{
			Value: p.Value,
		}

		switch p.Type {
		case resourcepaths.Literal:
			seg.Type = meta.PathSegment_LITERAL
		case resourcepaths.Param:
			seg.Type = meta.PathSegment_PARAM
		}

		res.Segments = append(res.Segments, seg)
	}
	return res
}
