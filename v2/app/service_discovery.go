package app

import (
	"golang.org/x/exp/slices"

	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/resource"
)

// discoverServices discovers the services in the whole application.
//
// Services are defined as a collection of packages which are considered
// to form a single logical service, which can be deployed and scaled as an
// atomic unit.
//
// Services are defined by a root filesystem directory, with everything under
// that directory considered part of the service.
//
// This means services cannot be nested, and a service cannot be split across
// multiple filesystem directories (such as in separate git repositories).
//
// It operates in three phases:
//
// 1. Find any API Framework service structs and mark those packages as services (service structs are always the root of a service)
// 2. Find any API Framework API's not currently in a service and mark those packages as services
// 3. Find any PubSub subscribers not currently in a service and mark those packages as services
//
// This does not setup the service framework data.
//
// The returned slice is sorted by service name.
func discoverServices(pc *parsectx.Context, result parser.Result) []*Service {
	defer pc.Trace("app.discoverServices").Done()

	sd := &serviceDiscovery{
		services:   make(map[paths.FS]*Service),
		strongRoot: make(map[paths.FS]struct{}),
	}

	// We can loop over all packages from the API result in one pass
	// as service roots get marked as strong roots
	for _, pkg := range result.APIs {
		if len(pkg.ServiceStructs) > 0 {
			sd.possibleServiceRoot(pkg.Pkg.Name, pkg.Pkg.FSPath, true)
		}

		if len(pkg.Endpoints) > 0 {
			sd.possibleServiceRoot(pkg.Pkg.Name, pkg.Pkg.FSPath, false)
		}
	}

	for _, res := range result.Infra.Resources() {
		if res.Kind() == resource.PubSubSubscription {
			pkg := res.Package()
			sd.possibleServiceRoot(pkg.Name, pkg.FSPath, false)
		}
	}

	// We then make an ordered slice of the services discovered
	services := make([]*Service, 0, len(sd.services))
	for _, s := range sd.services {
		services = append(services, s)
	}

	// Note: we sort by the FSRoot because we use that for binary searches in [ServiceForPath]
	slices.SortStableFunc(services, func(a, b *Service) bool {
		return a.FSRoot.ToIO() < b.FSRoot.ToIO()
	})

	// Finally, let's validate our services so we can report errors
	// FIXME: we should put this back in as otherwise we need to specify
	//        the expected behaviour for an Encore app with no services!
	// if len(services) == 0 {
	//	pc.Errs.AddStd(srcerrors.NoServicesFound())
	// }

	// Finally, let's validate our services
	for i, s := range services {
		for j := i + 1; j < len(services); j++ {
			// we need to check both directions of nesting as we only loop over one direction
			if s.FSRoot.HasPrefix(services[j].FSRoot) {
				pc.Errs.Add(errServiceContainedWitinAnother(s.Name, services[j].Name))
			} else if services[j].FSRoot.HasPrefix(s.FSRoot) {
				pc.Errs.Add(errServiceContainedWitinAnother(services[j].Name, s.Name))
			}

			if s.Name == services[j].Name {
				pc.Errs.Add(errDuplicateServiceNames(s.Name).InFile(s.FSRoot.ToIO()).InFile(services[j].FSRoot.ToIO()))
			}
		}
	}
	return services
}

type serviceDiscovery struct {
	errs     *perr.List            // errs is the error list to add errors to.
	services map[paths.FS]*Service // services maps the root folder to a service to the service

	// strongRoot marks a folder as a strong root, meaning we won't allow it to be merged
	// with a service in a parent folder.
	strongRoot map[paths.FS]struct{}
}

// possibleServiceRoot marks a folder as a possible service if it is not already marked as being part of a service.
//
// If strong is true, the root path always becomes a new service, event if it is within an existing service.
//
// If any existing services are descendants of the new service, they are removed from the list, unless
// the previous service was marked as a strong root.
func (sd *serviceDiscovery) possibleServiceRoot(name string, root paths.FS, strong bool) {
	if strong {
		// Always mark the root as a strong root, even if it is already marked as a service.
		sd.strongRoot[root] = struct{}{}
	}

	// If the service is already marked, we don't need to do anything.
	if _, ok := sd.services[root]; ok {
		return
	}

	// Loop over the existing services and remove any that are descendants of this root
	// but also look for any existing services which are ancestors of this root.
	for existingRoot := range sd.services {
		switch {
		// If the existing service is a descendant of the new service, we can remove it.
		case existingRoot.HasPrefix(root):
			// If the existing service is a strong root, we can't merge it with this service.
			if _, ok := sd.strongRoot[existingRoot]; ok {
				continue
			}

			// If the existing service is a descendant of the new service, we can remove it.
			delete(sd.services, existingRoot)

		// If the new service is a descendant of an existing service, we're done
		// because we don't allow nested services so this directory is part of the existing service.
		case root.HasPrefix(existingRoot) && !strong:
			return
		}
	}

	// If we get here, the new service is not a descendant of any existing services.
	// We can add it to the list of services.
	sd.services[root] = &Service{
		Name:   name,
		FSRoot: root,
	}
}
