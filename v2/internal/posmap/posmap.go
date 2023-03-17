package posmap

import (
	"go/ast"
	"sort"

	"encr.dev/pkg/option"
)

func Build[NodeLike ast.Node](nodes ...NodeLike) Map[NodeLike] {
	return (&Builder[NodeLike]{}).Add(nodes...).Build()
}

type Builder[NodeLike ast.Node] struct {
	nodes []NodeLike
}

func (b *Builder[NodeLike]) Add(nodes ...NodeLike) *Builder[NodeLike] {
	b.nodes = append(b.nodes, nodes...)
	return b
}

func (b *Builder[NodeLike]) Build() Map[NodeLike] {
	sort.Slice(b.nodes, func(i, j int) bool {
		return b.nodes[i].Pos() < b.nodes[j].Pos()
	})
	return Map[NodeLike]{
		nodes: b.nodes,
	}
}

type Map[NodeLike ast.Node] struct {
	nodes []NodeLike
}

func (m Map[NodeLike]) All() []NodeLike {
	return m.nodes
}

func (m Map[NodeLike]) Within(node ast.Node) []NodeLike {
	startIdx := sort.Search(len(m.nodes), func(i int) bool {
		return m.nodes[i].Pos() >= node.Pos()
	})

	endIdx := startIdx
	for endIdx < len(m.nodes) && m.nodes[endIdx].End() <= node.End() {
		endIdx++
	}

	nodes := make([]NodeLike, endIdx-startIdx)
	copy(nodes, m.nodes[startIdx:endIdx])
	return nodes
}

func (m Map[NodeLike]) Containing(node ast.Node) option.Option[NodeLike] {
	idx := sort.Search(len(m.nodes), func(i int) bool {
		return m.nodes[i].Pos() >= node.Pos()
	})

	// The containing resource, if any, has a starting position before this node
	// and an ending position after this node.
	if idx > 0 {
		candidate := m.nodes[idx-1]
		if candidate.Pos() <= node.Pos() && candidate.End() >= node.End() {
			return option.Some(candidate)
		}
	}

	return option.None[NodeLike]()
}
