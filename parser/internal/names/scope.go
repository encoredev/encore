package names

// scope maps names to information about them.
type scope struct {
	names  map[string]*Name
	parent *scope
}

func newScope(parent *scope) *scope {
	return &scope{
		names:  make(map[string]*Name),
		parent: parent,
	}
}

func (s *scope) Pop() *scope {
	return s.parent
}

func (s *scope) Insert(name string, r *Name) {
	if name != "_" {
		s.names[name] = r
	}
}

func (s *scope) Lookup(name string) *Name {
	return s.names[name]
}

func (s *scope) LookupParent(name string) *Name {
	if r := s.names[name]; r != nil {
		return r
	} else if s.parent != nil {
		return s.parent.LookupParent(name)
	}
	return nil
}
