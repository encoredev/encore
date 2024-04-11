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
