package scrub

import (
	"bytes"
	"fmt"
	"strings"
	"unsafe"

	"github.com/fmstephe/unsafeutil"
)

type EntryKind int

const (
	ObjectField EntryKind = iota
	MapKey
	MapValue
)

// scrubNode is an internal EntryKind for scrubbing the node
const scrubNode EntryKind = -1

type Path []PathEntry

type PathEntry struct {
	Kind          EntryKind
	FieldName     string
	CaseSensitive bool
}

// JSON scrubs the input JSON data, substituting values at the given paths
// with replaceWith.
//
// It returns the scrubbed data. If no substitutions were made scrubbed
// is the same slice as input.
func JSON(input []byte, paths []Path, replaceWith []byte) (scrubbed []byte) {
	indices := JSONIndices(input, paths)
	return scrub(input, indices, replaceWith)
}

func scrub(input []byte, indices []Bounds, replaceWith []byte) (scrubbed []byte) {
	if len(indices) == 0 {
		return input
	}

	// Compute the new length
	newLen := len(input)
	replaceLen := len(replaceWith)
	for _, idx := range indices {
		segLen := idx.To - idx.From
		newLen = newLen - segLen + replaceLen
	}

	scrubbed = make([]byte, newLen)
	prevEnd := 0
	dstIdx := 0
	for _, idx := range indices {
		dstIdx += copy(scrubbed[dstIdx:], input[prevEnd:idx.From])
		dstIdx += copy(scrubbed[dstIdx:], replaceWith)
		prevEnd = idx.To
	}

	copy(scrubbed[dstIdx:], input[prevEnd:])

	return scrubbed
}

type Bounds struct {
	From, To int
}

// JSONIndices computes the indices to replace in order to scrub the input JSON data.
func JSONIndices(input []byte, paths []Path) []Bounds {
	nodes := groupNodes(paths)
	str := newStream(input, nodes)
	return str.Process()
}

func newStream(input []byte, rootNodes []node) *stream {
	return &stream{
		s:         newScanner(bytes.NewReader(input)),
		input:     input,
		rootNodes: rootNodes,
	}
}

type stream struct {
	s         *scanner
	input     []byte
	rootNodes []node

	debugLit string
	tok      token
	pos      Bounds
	mapKey   bool

	unreadItem scanItem

	toScrub []Bounds
}

func (s *stream) Process() []Bounds {
	s.next() // initialize
	for s.tok != unknown {
		s.processValue(s.rootNodes)
	}
	return s.toScrub
}

// processValue processes a single complete value (such as an entire object
// or array recursively) and returns its bounds.
func (s *stream) processValue(nodes []node) {
	if s.scrubNow(nodes) {
		bounds := s.skipValue()
		s.toScrub = append(s.toScrub, bounds)
		return
	} else if len(nodes) == 0 {
		// no more nodes; just skip the value entirely.
		s.skipValue()
		return
	}

	switch s.tok {
	case objectBegin:
		s.processObject(nodes)
	case arrayBegin:
		s.processArray(nodes)
	default:
		// nothing to do
		s.next()
	}
}

func (s *stream) processArray(nodes []node) {
	for {
		s.next()
		if s.tok == unknown || s.tok == arrayEnd {
			return
		}
		s.processValue(nodes)
	}
}

func (s *stream) processObject(nodes []node) {
	s.next() // Move to the first key
	for {
		if s.tok == unknown || s.tok == objectEnd {
			return
		}

		// Determine which nodes to continue with
		currNodes, valueNodes := s.matchingMapNodes(nodes)
		s.processValue(currNodes)

		// If we have a map value next, process it.
		if s.isMapValue() {
			s.processValue(valueNodes)
		}
	}
}

func (s *stream) skipValue() Bounds {
	start := s.pos.From
	end := s.pos.To
	var depth int
	for {
		switch s.tok {
		case objectBegin, arrayBegin:
			depth++
		case objectEnd, arrayEnd:
			depth--
		}
		end = s.pos.To
		s.next()
		if depth <= 0 {
			break
		}
	}
	return Bounds{From: start, To: end}
}

func (s *stream) matchingMapNodes(nodes []node) (currNodes, valueNodes []node) {
	for _, n := range nodes {
		if (n.Kind == MapKey && s.mapKey) || (n.Kind == MapValue && !s.mapKey) {
			currNodes = append(currNodes, n.Children...)
		} else if n.Kind == ObjectField && s.mapKey {
			fieldName := s.input[s.pos.From:s.pos.To]
			cs := n.CaseSensitive

			// Have we found a matching field?
			var fieldMatch bool
			if cs {
				fieldMatch = bytes.Equal(fieldName, unsafeutil.StringToBytes(n.FieldName))
			} else {
				fieldMatch = bytes.EqualFold(fieldName, unsafeutil.StringToBytes(n.FieldName))
			}

			if fieldMatch {
				valueNodes = append(valueNodes, n.Children...)
			}
		}
	}
	return
}

// isMapValue reports whether the current token is a map value.
func (s *stream) isMapValue() bool {
	if s.mapKey {
		return false
	}
	switch s.tok {
	case objectBegin, arrayBegin, literal:
		return true
	default:
		return false
	}
}

func (s *stream) scrubNow(nodes []node) bool {
	for _, n := range nodes {
		if n.Kind == scrubNode {
			return true
		}
	}
	return false
}

func (s *stream) next() {
	// Handle unread
	if s.unreadItem.tok != 0 {
		it := s.unreadItem
		s.unreadItem = scanItem{}
		s.tok, s.pos, s.mapKey = it.tok, it.Bounds(), it.isMapKey
		s.debugLit = string(s.input[s.pos.From:s.pos.To])
		return
	}

	if !s.s.Next() {
		s.tok = unknown
		s.debugLit = ""
	} else {
		it := s.s.Item()
		s.tok, s.pos, s.mapKey = it.tok, it.Bounds(), it.isMapKey
		s.debugLit = string(s.input[s.pos.From:s.pos.To])
	}
}

func (s *stream) unread() {
	if s.unreadItem.tok != 0 {
		panic(fmt.Sprintf("double unread: %s followed by %s", s.unreadItem.tok, s.tok))
	}

	s.unreadItem = scanItem{
		tok:      s.tok,
		from:     int64(s.pos.From),
		to:       int64(s.pos.To),
		isMapKey: s.mapKey,
	}
}

// groupNodes transforms the paths into a node tree for efficient matching.
func groupNodes(paths []Path) []node {
	findMatch := func(parent *node, e PathEntry) int {
		for idx, c := range parent.Children {
			if c.Kind != e.Kind {
				continue
			}
			if e.Kind != ObjectField {
				// The other attributes only matter for ObjectField
				return idx
			}

			if e.CaseSensitive != c.CaseSensitive {
				continue
			}

			var equal bool
			if e.CaseSensitive {
				equal = strings.EqualFold(e.FieldName, c.FieldName)
			} else {
				equal = e.FieldName == c.FieldName
			}
			if equal {
				return idx
			}
		}
		return -1
	}

	hasScrub := func(parent *node) bool {
		return len(parent.Children) == 1 && parent.Children[0].Kind == scrubNode
	}

	root := node{}
	for _, path := range paths {
		parent := &root
		for _, e := range path {
			// If we're already scrubbing this path, we're done.
			if hasScrub(parent) {
				break
			}

			idx := findMatch(parent, e)
			if idx == -1 {
				parent.Children = append(parent.Children, node{
					Kind:          e.Kind,
					FieldName:     e.FieldName,
					CaseSensitive: e.CaseSensitive,
				})
				parent = &parent.Children[len(parent.Children)-1]
			} else {
				parent = &parent.Children[idx]
			}
		}

		// At the end of each path we want to scrub the current value.
		// Add a synthetic node to do so.
		parent.Children = []node{{Kind: scrubNode}}
	}

	return root.Children
}

type node struct {
	Kind          EntryKind
	FieldName     string
	CaseSensitive bool
	Children      []node
}

func unsafeStrToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}
