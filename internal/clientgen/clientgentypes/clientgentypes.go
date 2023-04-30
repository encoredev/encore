package clientgentypes

import (
	"bytes"

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

func NewServiceSet(svcs ...string) ServiceSet {
	set := make(map[string]bool, len(svcs))
	for _, svc := range svcs {
		set[svc] = true
	}
	return ServiceSet{
		list: svcs,
		set:  set,
	}
}
