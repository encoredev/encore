package clientgentypes

import (
	"bytes"
	"slices"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type GenerateParams struct {
	Buf      *bytes.Buffer
	AppSlug  string
	Meta     *meta.Data
	Services ServiceSet
	Tags     TagSet
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
	includedTags []string
	excludedTags []string
}

func NewTagSet(tags, excludedTags []string) TagSet {
	filteredTags := make([]string, 0, len(tags))
	for _, t := range tags {
		if !slices.Contains(excludedTags, t) {
			filteredTags = append(filteredTags, t)
		}
	}

	return TagSet{
		includedTags: filteredTags,
		excludedTags: excludedTags,
	}
}

func (t TagSet) IsRPCIncluded(rpc *meta.RPC) bool {
	// First check if the RPC has any of the excluded tags.
	for _, selector := range rpc.Tags {
		if selector.Type != meta.Selector_TAG {
			continue
		}

		if slices.Contains(t.excludedTags, selector.Value) {
			return false
		}
	}

	// If `tags` is empty, all tags are included.
	if len(t.includedTags) == 0 {
		return true
	}

	// Check if the RPC has any of the included tags.
	for _, selector := range rpc.Tags {
		if selector.Type != meta.Selector_TAG {
			continue
		}

		if slices.Contains(t.includedTags, selector.Value) {
			return true
		}
	}

	return false
}
