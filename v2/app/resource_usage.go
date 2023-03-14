package app

import (
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/resource/usage"
)

// locateResourceUsage finds all resource usages and assigns them to the appropriate service.
// If a resource is used outside of a service, it is assigned to the top-level app description.
func (d *Desc) locateResourceUsage(result *parser.Result) {
	allUsages := result.AllUsages()

	for _, use := range allUsages {
		ref := use.ResourceBind().ResourceRef()
		if ref.Resource == nil {
			continue
		}

		svc, found := d.ServiceForPath(use.DeclaredIn().Pkg.FSPath)
		if found {
			// If we found the service, then we know the resource is used within the service.
			resUsages, found := svc.ResourceUsage[ref.Resource]
			if !found {
				resUsages = make([]usage.Usage, 0, 1)
				svc.ResourceUsage[ref.Resource] = resUsages
			}

			svc.ResourceUsage[ref.Resource] = append(resUsages, use)
		} else {
			// Otherwise, the resource is used outside of a service.
			resUsages, found := d.ResourceUsageOutsideServices[ref.Resource]
			if !found {
				resUsages = make([]usage.Usage, 0, 1)
				d.ResourceUsageOutsideServices[ref.Resource] = resUsages
			}

			d.ResourceUsageOutsideServices[ref.Resource] = append(resUsages, use)
		}
	}
}
