package rewrite

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
)

func New(data []byte, base int) *Rewriter {
	return &Rewriter{
		base: 0,
		segs: []segment{{
			data:  data,
			start: base,
			end:   base + len(data),
		}},
	}
}

type Rewriter struct {
	base int
	segs []segment
}

func (r *Rewriter) Replace(start, end token.Pos, data []byte) {
	si, so := r.seg(start, false)
	ei, eo := r.seg(end, true)
	r.replace(si, so, ei, eo, data)
}

func (r *Rewriter) ReplaceNode(node ast.Node, data []byte) {
	r.Replace(node.Pos(), node.End(), data)
}

func (r *Rewriter) Append(data []byte) {
	start := 0
	if len(r.segs) > 0 {
		start = r.segs[len(r.segs)-1].end
	}
	r.segs = append(r.segs, segment{
		start: int(start),
		end:   int(start) + len(data),
		data:  data,
	})
}

func (r *Rewriter) Insert(start token.Pos, data []byte) {
	// If the pos is at the very end of the file, insert a new segment directly,
	// since calling r.seg(start) would panic.
	if len(r.segs) > 0 && r.segs[len(r.segs)-1].end == int(start) {
		r.segs = append(r.segs, segment{
			start: int(start),
			end:   int(start) + len(data),
			data:  data,
		})
		return
	}

	si, so := r.seg(start, false)
	r.replace(si, so, si, so, data)
}

func (r *Rewriter) Delete(start, end token.Pos) {
	si, so := r.seg(start, false)
	ei, eo := r.seg(end, true)
	r.replace(si, so, ei, eo, nil)
}

func (r *Rewriter) Data() []byte {
	var buf bytes.Buffer
	for _, seg := range r.segs {
		buf.Write(seg.data)
	}
	return buf.Bytes()
}

func (r *Rewriter) replace(si, so, ei, eo int, data []byte) {
	if si == ei {
		// Same segment; cut it into two
		start := r.segs[si]
		end := segment{
			start: start.start + eo,
			end:   start.end,
			data:  start.data[eo:],
		}
		start.data = start.data[:so]
		start.end = start.start + so
		mid := segment{
			start: start.end,
			end:   end.start,
			data:  data,
		}
		r.segs = append(r.segs[:si], append([]segment{start, mid, end}, r.segs[ei+1:]...)...)
	} else {
		// Already different segments; adjust start/end and replace segments in-between
		start := r.segs[si]
		end := r.segs[ei]
		start.end = start.start + so
		start.data = start.data[:so]
		end.start += eo
		end.data = end.data[eo:]
		mid := segment{
			start: start.end,
			end:   end.start,
			data:  data,
		}
		r.segs = append(r.segs[:si], append([]segment{start, mid, end}, r.segs[ei+1:]...)...)
	}
}

func (r *Rewriter) seg(pos token.Pos, isEnd bool) (idx int, offset int) {
	p := int(pos)
	if isEnd {
		for i, seg := range r.segs {
			if seg.start < p && p <= seg.end {
				return i, min(int(p-seg.start), len(seg.data))
			}
		}
	} else {
		for i, seg := range r.segs {
			if seg.start <= p && p < seg.end {
				return i, min(int(p-seg.start), len(seg.data))
			}
		}
	}

	panic(fmt.Sprintf("original file does not contain pos %v", pos))
}

type segment struct {
	start int // inclusive
	end   int // exclusive
	data  []byte
}
