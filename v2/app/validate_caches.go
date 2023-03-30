package app

import (
	"fmt"

	"encr.dev/pkg/errors"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/resourcepaths"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/caches"
)

func (d *Desc) validateCaches(pc *parsectx.Context, results *parser.Result) {
	type cache struct {
		resource *caches.Cluster
		paths    *resourcepaths.Set
	}
	found := make(map[string]cache)
	byBinding := make(map[pkginfo.QualifiedName]string)

	// First find all clusters
	var keyspaces []*caches.Keyspace
	for _, res := range d.Parse.Resources() {
		switch res := res.(type) {
		case *caches.Cluster:
			if existing, ok := found[res.Name]; ok {
				pc.Errs.Add(
					caches.ErrDuplicateCacheCluster.
						AtGoNode(existing.resource.AST.Args[0]).
						AtGoNode(res.AST.Args[0]),
				)
				continue
			}

			found[res.Name] = cache{
				resource: res,
				paths:    resourcepaths.NewSet(),
			}

			for _, bind := range d.Parse.PkgDeclBinds(res) {
				byBinding[bind.QualifiedName()] = res.Name
			}

		case *caches.Keyspace:
			keyspaces = append(keyspaces, res)
		}
	}

	// Then verify all keyspaces
	for _, ks := range keyspaces {
		clusterName := byBinding[ks.Cluster]
		cluster, ok := found[clusterName]
		if !ok {
			pc.Errs.Add(caches.ErrCouldNotResolveCacheCluster.AtGoNode(ks.AST.Args[0]))
			continue
		}

		cluster.paths.Add(pc.Errs, "*", ks.Path)

		svc, ok := d.ServiceForPath(ks.File.FSPath)
		if !ok {
			pc.Errs.Add(caches.ErrKeyspaceNotInService.AtGoNode(ks.AST.Fun))
			continue
		}

		for _, use := range results.Usages(ks) {
			errTxt := "used here"
			useSvc, ok := d.ServiceForPath(use.DeclaredIn().FSPath)
			if ok {
				errTxt = fmt.Sprintf("used in %q", useSvc.Name)
			}

			if useSvc != svc {
				pc.Errs.Add(caches.ErrKeyspaceUsedInOtherService.
					AtGoNode(use, errors.AsError(errTxt)).
					AtGoNode(ks, errors.AsHelp(fmt.Sprintf("declared in %q", svc.Name))),
				)
			}
		}
	}
}
