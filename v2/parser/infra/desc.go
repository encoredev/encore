package infra

import (
	"fmt"
	"strings"

	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/infra/usage"
)

// ComputeDesc computes the infrastructure description
// given a list of resources and binds.
func ComputeDesc(errs *perr.List, resources []resource.Resource, binds []resource.Bind, usage []usage.Usage) *Desc {
	bindMap := computeBindMap(errs, resources, binds)
	usageMap := computeUsageMap(resources, usage, bindMap)
	return &Desc{
		resources: resources,
		binds:     binds,
		bindMap:   bindMap,
		usageMap:  usageMap,
	}
}

type Desc struct {
	resources []resource.Resource
	binds     []resource.Bind
	bindMap   map[resource.Resource][]resource.Bind
	usageMap  map[resource.Resource][]usage.Usage
}

func (s *Desc) Resources() []resource.Resource {
	return s.resources
}

func (s *Desc) Binds(resource resource.Resource) []resource.Bind {
	return s.bindMap[resource]
}

func (s *Desc) Usages(resource resource.Resource) []usage.Usage {
	return s.usageMap[resource]
}

func computeBindMap(errs *perr.List, resources []resource.Resource, binds []resource.Bind) map[resource.Resource][]resource.Bind {
	result := make(map[resource.Resource][]resource.Bind, len(resources))
	byPath := make(map[string]resource.Resource, len(resources))

	for _, r := range resources {
		// If we have a named resource, add it to the path map.
		if named, ok := r.(resource.Named); ok {
			p := resource.Path{{named.Kind(), named.ResourceName()}}
			byPath[pathKey(p)] = r
		}
	}

	for _, b := range binds {
		// Do we have a specific resource reference?
		if r := b.Resource.Resource; r != nil {
			result[r] = append(result[r], b)
			continue
		}

		// Otherwise figure out the resource from the bind path.
		key := pathKey(b.Resource.Path)
		if r, ok := byPath[key]; ok {
			result[r] = append(result[r], b)
		} else {
			// NOTE(andre): We could end up here in the future when we support
			// named references to PubSub subscriptions, since those would
			// involve a two-segment resource path (first the topic and then the subscription),
			// which we don't support today (the construction of byPath above only handles
			// the case of single-segment resource paths).
			// Since we don't support that today, this is fine for now.
			errs.Addf(b.BoundName.Pos(), "internal compiler error: unknown resource (path %q)", key)
		}
	}

	return result
}

func computeUsageMap(resources []resource.Resource, usages []usage.Usage, bindMap map[resource.Resource][]resource.Bind) map[resource.Resource][]usage.Usage {
	resourcesByBindName := make(map[pkginfo.QualifiedName]resource.Resource, len(resources))
	for r, binds := range bindMap {
		for _, bind := range binds {
			resourcesByBindName[bind.QualifiedName()] = r
		}
	}

	result := make(map[resource.Resource][]usage.Usage, len(resources))
	for _, u := range usages {
		if r, ok := resourcesByBindName[u.ResourceBind().QualifiedName()]; ok {
			result[r] = append(result[r], u)
		}
	}
	return result
}

func pathKey(path resource.Path) string {
	var b strings.Builder
	for i, e := range path {
		if i > 0 {
			b.WriteString("/")
		}
		fmt.Fprintf(&b, "%s:%s", e.Kind.String(), e.Name)
	}
	return b.String()
}
