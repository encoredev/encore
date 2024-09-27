package clientgentypes

import (
	"bytes"
	"slices"
	"strconv"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type GenerateParams struct {
	Buf      *bytes.Buffer
	AppSlug  string
	Meta     *meta.Data
	Services ServiceSet
	Tags     *TagSet
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
	tagMap       map[string]bool
	includedTags []string
}

func NewTagSet(tags map[string]string) (*TagSet, error) {
	tagSet := TagSet{
		tagMap:       make(map[string]bool),
		includedTags: make([]string, 0, len(tags)),
	}
	for tag, includedStr := range tags {
		included, err := strconv.ParseBool(includedStr)
		if err != nil {
			return nil, err
		}

		tagSet.tagMap[tag] = included
		if included {
			tagSet.includedTags = append(tagSet.includedTags, tag)
		}
	}

	return &tagSet, nil
}

func (t TagSet) IsRPCIncluded(rpc *meta.RPC) bool {
	// First check if the RPC has any of the excluded tags.
	for _, selector := range rpc.Tags {
		if selector.Type != meta.Selector_TAG {
			continue
		}

		if included, ok := t.tagMap[selector.Value]; ok && !included {
			return false
		}
	}

	// If no included tags are specified, all tags are included.
	if len(t.includedTags) == 0 {
		return true
	}

	// Check if the RPC has any of the included tags.
	for _, selector := range rpc.Tags {
		if selector.Type != meta.Selector_TAG {
			continue
		}

		if included, ok := t.tagMap[selector.Value]; ok && included {
			return true
		}
	}

	// If no included tags are found, the RPC is not included.
	return false
}
