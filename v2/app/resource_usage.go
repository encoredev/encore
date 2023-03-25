package app

import (
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/resource/usage"
)

// locateResourceBinds finds all resource binds and assigns them to the appropriate service.
func (d *Desc) locateResourceBinds(result *parser.Result) {
	allBinds := result.AllBinds()

	for _, bind := range allBinds {
		res := result.ResourceForBind(bind)
		svc, found := d.ServiceForPath(bind.Package().FSPath)
		if found {
			// If we found the service, then we know the resource is used within the service.
			svc.ResourceBinds[res] = append(svc.ResourceBinds[res], bind)
		}
	}
}

// locateResourceUsage finds all resource usages and assigns them to the appropriate service.
// If a resource is used outside of a service, it is assigned to the top-level app description.
func (d *Desc) locateResourceUsage(result *parser.Result) {
	allUsages := result.AllUsages()

	for _, use := range allUsages {
		res := result.ResourceForBind(use.ResourceBind())

		svc, found := d.ServiceForPath(use.DeclaredIn().Pkg.FSPath)
		if found {
			// If we found the service, then we know the resource is used within the service.
			resUsages, found := svc.ResourceUsage[res]
			if !found {
				resUsages = make([]usage.Usage, 0, 1)
				svc.ResourceUsage[res] = resUsages
			}

			svc.ResourceUsage[res] = append(resUsages, use)
		} else {
			// Otherwise, the resource is used outside of a service.
			resUsages, found := d.ResourceUsageOutsideServices[res]
			if !found {
				resUsages = make([]usage.Usage, 0, 1)
				d.ResourceUsageOutsideServices[res] = resUsages
			}

			d.ResourceUsageOutsideServices[res] = append(resUsages, use)
		}
	}
}
