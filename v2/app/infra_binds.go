package app

import (
	"fmt"
	"strings"

	"encr.dev/v2/internal/perr"
	"encr.dev/v2/parser/infra/resource"
)

func computeInfraBindMap(errs *perr.List, resources []resource.Resource, binds []resource.Bind) map[resource.Resource][]resource.Bind {
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
			errs.Addf(b.PackageName.Pos(), "internal compiler error: unknown resource (path %q)", key)
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
