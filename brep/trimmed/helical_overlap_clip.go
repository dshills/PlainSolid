package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type entryClipNode struct {
	uv     geom.Vec2
	vertex int
}
type entryClipSegment struct {
	a, b     entryClipNode
	old      int
	reversed bool
}
type entryClipVertex struct {
	id        int
	parameter float64
}
type entryClipper struct {
	ctx      context.Context
	source   Solid
	r        HelicalGrooveRecipe
	depth    float64
	vertices []geom.Vec3
	edges    []parametric.Edge
	faces    []Face
	cuts     map[int]entryClipVertex
	edgeIDs  map[int]int
}

func (c *entryClipper) axial(surface geom.Surface, uv geom.Vec2) (*big.Rat, error) {
	switch s := surface.(type) {
	case geom.TwistedCylinderSurface, geom.CylinderSurface:
		return ratInset(uv.Y), nil
	case geom.HelicalProfileSurface:
		_, a, b, lead, _ := s.Parameters()
		z := new(big.Rat).Add(new(big.Rat).Mul(ratInset(lead), ratInset(uv.X)), ratInset(a.Y))
		z.Add(z, new(big.Rat).Mul(new(big.Rat).Sub(ratInset(b.Y), ratInset(a.Y)), ratInset(uv.Y)))
		return z, nil
	case geom.PlaneSurface:
		f := s.Frame()
		z := c.r.Frame.ToLocal(f.Origin()).Z
		dx, dy := f.XAxis().Dot(c.r.Frame.ZAxis()), f.YAxis().Dot(c.r.Frame.ZAxis())
		q := new(big.Rat).Add(ratInset(z), new(big.Rat).Mul(ratInset(dx), ratInset(uv.X)))
		q.Add(q, new(big.Rat).Mul(ratInset(dy), ratInset(uv.Y)))
		return q, nil
	}
	return nil, fmt.Errorf("unsupported overlap clipping surface %T", surface)
}
func (c *entryClipper) intersection(u Use, surface geom.Surface, za, zb *big.Rat) (entryClipNode, error) {
	t := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(c.depth), za), new(big.Rat).Sub(zb, za))
	fraction, _ := t.Float64()
	uv := geom.Vec2{X: u.UVStart.X + (u.UVEnd.X-u.UVStart.X)*fraction, Y: u.UVStart.Y + (u.UVEnd.Y-u.UVStart.Y)*fraction}
	if _, ok := surface.(geom.TwistedCylinderSurface); ok {
		uv.Y = c.depth
	}
	cut, ok := c.cuts[u.Edge]
	if !ok {
		edge := c.source.edges[u.Edge]
		parameterFraction := fraction
		if u.Reversed {
			parameterFraction = 1 - fraction
		}
		parameter := edge.Range.Min + (edge.Range.Max-edge.Range.Min)*parameterFraction
		p, e := edge.Curve.Eval(parameter)
		if e != nil {
			return entryClipNode{}, e
		}
		if math.Abs(c.r.Frame.ToLocal(p).Z-c.depth) > c.source.tolerance {
			return entryClipNode{}, geom.ErrConvergence
		}
		cut = entryClipVertex{len(c.vertices), parameter}
		c.vertices = append(c.vertices, p)
		c.cuts[u.Edge] = cut
	}
	p, e := surface.Eval(uv.X, uv.Y)
	if e != nil || p.Sub(c.vertices[cut.id]).Length() > c.source.tolerance {
		return entryClipNode{}, geom.ErrConvergence
	}
	return entryClipNode{uv, cut.id}, nil
}
func (c *entryClipper) clipLoop(surface geom.Surface, loop []Use) ([]entryClipSegment, error) {
	var segments []entryClipSegment
	for _, u := range loop {
		if u.Curve != nil {
			return nil, geom.ErrDomain
		}
		edge := c.source.edges[u.Edge]
		a, b := edge.Start, edge.End
		if u.Reversed {
			a, b = b, a
		}
		na, nb := entryClipNode{u.UVStart, a}, entryClipNode{u.UVEnd, b}
		za, e := c.axial(surface, u.UVStart)
		if e != nil {
			return nil, e
		}
		zb, e := c.axial(surface, u.UVEnd)
		if e != nil {
			return nil, e
		}
		ia, ib := za.Cmp(ratInset(c.depth)) >= 0, zb.Cmp(ratInset(c.depth)) >= 0
		if !ia && !ib {
			continue
		}
		if ia != ib {
			n, e := c.intersection(u, surface, za, zb)
			if e != nil {
				return nil, e
			}
			if ia {
				nb = n
			} else {
				na = n
			}
		}
		segments = append(segments, entryClipSegment{na, nb, u.Edge, u.Reversed})
	}
	if len(segments) == 0 {
		return nil, nil
	}
	var closed []entryClipSegment
	for i, s := range segments {
		closed = append(closed, s)
		next := segments[(i+1)%len(segments)]
		if s.b.vertex != next.a.vertex || s.b.uv != next.a.uv {
			closed = append(closed, entryClipSegment{s.b, next.a, -1, false})
		}
	}
	return closed, nil
}
func entryNotch(outer, hole []entryClipSegment) ([]entryClipSegment, error) {
	hi := -1
	for i, s := range hole {
		if s.old < 0 {
			if hi >= 0 {
				return nil, geom.ErrDomain
			}
			hi = i
		}
	}
	if hi < 0 {
		return nil, geom.ErrDomain
	}
	cut := hole[hi]
	for i, s := range outer {
		if s.old >= 0 || s.a.uv.Y != s.b.uv.Y || cut.a.uv.Y != s.a.uv.Y || cut.b.uv.Y != s.a.uv.Y {
			continue
		}
		if !(s.a.uv.X < cut.b.uv.X && cut.b.uv.X < cut.a.uv.X && cut.a.uv.X < s.b.uv.X) {
			return nil, geom.ErrDomain
		}
		out := append([]entryClipSegment(nil), outer[:i]...)
		out = append(out, entryClipSegment{s.a, cut.b, -1, false})
		for k := 1; k < len(hole); k++ {
			out = append(out, hole[(hi+k)%len(hole)])
		}
		out = append(out, entryClipSegment{cut.a, s.b, -1, false})
		out = append(out, outer[i+1:]...)
		return out, nil
	}
	return nil, geom.ErrDomain
}
func (c *entryClipper) uses(surface geom.Surface, segments []entryClipSegment) ([]Use, error) {
	var uses []Use
	for _, s := range segments {
		id := -1
		reverse := false
		if s.old >= 0 {
			if prior, ok := c.edgeIDs[s.old]; ok {
				id = prior
			} else {
				edge := c.source.edges[s.old]
				if cut, ok := c.cuts[s.old]; ok {
					za := c.r.Frame.ToLocal(c.source.vertices[edge.Start]).Z
					zb := c.r.Frame.ToLocal(c.source.vertices[edge.End]).Z
					if za < c.depth && zb > c.depth {
						edge.Start = cut.id
						edge.Range.Min = cut.parameter
					} else if zb < c.depth && za > c.depth {
						edge.End = cut.id
						edge.Range.Max = cut.parameter
					} else {
						return nil, geom.ErrDomain
					}
				}
				id = len(c.edges)
				c.edges = append(c.edges, edge)
				c.edgeIDs[s.old] = id
			}
			reverse = s.reversed
		} else {
			id = len(c.edges)
			c.edges = append(c.edges, parametric.Edge{Start: s.a.vertex, End: s.b.vertex, Range: parametric.Interval{Max: 1}, Curve: surfaceTrimCurve{surface, s.a.uv, s.b.uv, nil}})
		}
		uses = append(uses, Use{Edge: id, Reversed: reverse, UVStart: s.a.uv, UVEnd: s.b.uv})
	}
	return uses, nil
}
func clipHelicalEntry(ctx context.Context, s Solid, r HelicalGrooveRecipe, depth float64) (*entryClipper, error) {
	c := &entryClipper{ctx: ctx, source: s, r: r, depth: depth, vertices: s.Vertices(), cuts: map[int]entryClipVertex{}, edgeIDs: map[int]int{}}
	bottom, top := 1, 2
	if r.BoreRadius > 0 {
		bottom, top = 2, 3
	}
	for id, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if id == bottom {
			continue
		}
		untouched := id == top || r.BoreRadius > 0 && id == 0
		var outer []entryClipSegment
		var holes [][]entryClipSegment
		if untouched {
			convert := func(loop []Use) []entryClipSegment {
				var out []entryClipSegment
				for _, u := range loop {
					edge := s.edges[u.Edge]
					a, b := edge.Start, edge.End
					if u.Reversed {
						a, b = b, a
					}
					out = append(out, entryClipSegment{entryClipNode{u.UVStart, a}, entryClipNode{u.UVEnd, b}, u.Edge, u.Reversed})
				}
				return out
			}
			outer = convert(f.Outer)
			for _, hole := range f.Holes {
				holes = append(holes, convert(hole))
			}
		} else {
			var e error
			outer, e = c.clipLoop(f.Surface, f.Outer)
			if e != nil {
				return nil, e
			}
			if len(outer) == 0 {
				continue
			}
			for _, hole := range f.Holes {
				h, e := c.clipLoop(f.Surface, hole)
				if e != nil {
					return nil, e
				}
				if len(h) == 0 {
					continue
				}
				crosses := false
				for _, u := range h {
					crosses = crosses || u.old < 0
				}
				if crosses {
					outer, e = entryNotch(outer, h)
					if e != nil {
						return nil, e
					}
				} else {
					holes = append(holes, h)
				}
			}
		}
		var e error
		f.Outer, e = c.uses(f.Surface, outer)
		if e != nil {
			return nil, e
		}
		f.Holes = nil
		for _, hole := range holes {
			u, e := c.uses(f.Surface, hole)
			if e != nil {
				return nil, e
			}
			f.Holes = append(f.Holes, u)
		}
		c.faces = append(c.faces, f)
	}
	return c, nil
}
