package rtconfgen

import "fmt"

// A resource is an object with a unique resource id (rid).
type resource interface {
	GetRid() string
}

type resourceKey struct {
	typ string
	rid string
}

// A resourceSet tracks whether a resource has been seen before based on its id,
// and allows efficient lookup of a resource by id.
type resourceSet struct {
	m map[resourceKey]any
}

// rsAdd adds a resource to the set. It reports whether the resource was added.
func rsAdd[R any](rs *resourceSet, rid string, fn func() R) (val R, added bool) {
	key := internalRSKeyByID[R](rid)
	if existing := rs.m[key]; existing != nil {
		return existing.(R), false
	}
	if rs.m == nil {
		rs.m = make(map[resourceKey]any)
	}

	val = fn()
	rs.m[key] = val
	return val, true
}

func internalRSKeyByID[R any](rid string) resourceKey {
	var zero R
	typ := fmt.Sprintf("%T", zero)
	return resourceKey{typ, rid}
}

func addResFunc[R any](dst *[]R, rs *resourceSet, rid string, fn func() R) (stored R) {
	*dst, stored = appendResFunc(*dst, rs, rid, fn)
	return stored
}

func appendResFunc[R any](dst []R, rs *resourceSet, rid string, fn func() R) (result []R, stored R) {
	stored, updated := rsAdd(rs, rid, fn)
	if updated {
		dst = append(dst, stored)
	}
	return dst, stored
}
