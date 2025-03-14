package clientgentypes

import (
	"bytes"
	"slices"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Options for the client generator.
type Options struct {
	OpenAPIExcludePrivateEndpoints bool
}

type GenerateParams struct {
	Buf      *bytes.Buffer
	AppSlug  string
	Meta     *meta.Data
	Services ServiceSet
	Tags     TagSet
	Options  Options
}

type ServiceSet struct {
	list []string
	set  map[string]bool
}

func (s ServiceSet) List() []string {
	return s.list
}

func (s ServiceSet) Has(svc string) bool {
	return s.set[svc]
}

// NewServiceSet constructs a new service set.
// If the list contains "*", include all services in the metadata.
// Finally, exclude any services in the exclude list.
func NewServiceSet(md *meta.Data, include, exclude []string) ServiceSet {
	set := make(map[string]bool, len(include))
	if slices.Contains(include, "*") {
		// If the list contains "*", include all services.
		for _, svc := range md.Svcs {
			set[svc.Name] = true
		}
	} else {
		for _, svc := range include {
			set[svc] = true
		}
	}

	// Remove excludes.
	for _, svc := range exclude {
		delete(set, svc)
	}

	list := make([]string, 0, len(set))
	for svc := range set {
		list = append(list, svc)
	}
	slices.Sort(list)

	return ServiceSet{
		list: list,
		set:  set,
	}
}

func AllServices(md *meta.Data) ServiceSet {
	return NewServiceSet(md, []string{"*"}, nil)
}

type TagSet struct {
	included map[string]bool
	excluded map[string]bool
}

func NewTagSet(tags, excludedTags []string) TagSet {
	tagSet := TagSet{
		included: make(map[string]bool),
		excluded: make(map[string]bool),
	}

	for _, tag := range tags {
		tagSet.included[tag] = true
	}
	for _, tag := range excludedTags {
		tagSet.excluded[tag] = true
	}

	return tagSet
}

func (t TagSet) IsRPCIncluded(rpc *meta.RPC) bool {
	// First check if the RPC has any of the excluded tags.
	for _, selector := range rpc.Tags {
		if selector.Type != meta.Selector_TAG {
			continue
		}

		if excluded, ok := t.excluded[selector.Value]; ok && excluded {
			return false
		}
	}

	// If no included tags are specified, all tags are included.
	if len(t.included) == 0 {
		return true
	}

	// Check if the RPC has any of the included tags.
	for _, selector := range rpc.Tags {
		if selector.Type != meta.Selector_TAG {
			continue
		}

		if included, ok := t.included[selector.Value]; ok && included {
			return true
		}
	}

	// If no included tags are found, the RPC is not included.
	return false
}
