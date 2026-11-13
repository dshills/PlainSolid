package trimmed

import (
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// EdgeSplit identifies the new vertex and the two edges in the original
// model-curve direction. First retains the input edge ID.
type EdgeSplit struct{ First, Second, Vertex int }

// SplitEdge inserts a model-space vertex at a strict interior curve parameter.
// Both uses are split in place, including reverse orientation and seams.
// The curve is retained with disjoint parameter intervals. Collapsed pole
// edges cannot be split. The resulting UV polygons must remain simple.
func (s Solid) SplitEdge(edgeID int, t float64) (Solid, EdgeSplit, error) {
	draft, split, err := s.splitEdgeDraft(edgeID, t)
	if err != nil {
		return Solid{}, EdgeSplit{}, err
	}
	result, err := NewSolid(draft.vertices, draft.edges, draft.faces, s.tolerance)
	if err != nil {
		return Solid{}, EdgeSplit{}, err
	}
	return result, split, nil
}

// splitEdgeDraft is private so a multi-step topology edit can validate the
// complete boundary once, before returning a Solid to the caller.
func (s Solid) splitEdgeDraft(edgeID int, t float64) (Solid, EdgeSplit, error) {
	var h EdgeSplit
	if edgeID < 0 || edgeID >= len(s.edges) {
		return Solid{}, h, fmt.Errorf("trimmed: edge index out of range")
	}
	edge := s.edges[edgeID]
	if edge.Curve == nil || !finite(t) || t <= edge.Range.Min || t >= edge.Range.Max {
		return Solid{}, h, fmt.Errorf("trimmed: split parameter outside edge interior")
	}
	x := (t - edge.Range.Min) / (edge.Range.Max - edge.Range.Min)
	if x <= 0 || x >= 1 {
		return Solid{}, h, fmt.Errorf("trimmed: unrepresentable edge parameter")
	}
	p, e := edge.Curve.Eval(t)
	if e != nil {
		return Solid{}, h, e
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	h = EdgeSplit{edgeID, len(edges), len(vertices)}
	vertices = append(vertices, p)
	edges[h.First] = parametric.Edge{Start: edge.Start, End: h.Vertex, Curve: edge.Curve, Range: parametric.Interval{Min: edge.Range.Min, Max: t}}
	edges = append(edges, parametric.Edge{Start: h.Vertex, End: edge.End, Curve: edge.Curve, Range: parametric.Interval{Min: t, Max: edge.Range.Max}})
	split := func(loop []Use) ([]Use, error) {
		r := make([]Use, 0, len(loop)+1)
		for _, u := range loop {
			if u.Edge != edgeID {
				r = append(r, u)
				continue
			}
			k := x
			if u.Reversed {
				k = 1 - k
			}
			if k <= 0 || k >= 1 {
				return nil, fmt.Errorf("trimmed: unrepresentable reversed split parameter")
			}
			m, e := u.uv(k)
			if e != nil {
				return nil, e
			}
			a, b := u, u
			a.UVEnd, b.UVStart = m, m
			b.Edge = h.Second
			if u.Reversed {
				a.Edge, b.Edge = b.Edge, a.Edge
			}
			if u.Curve != nil {
				a.Curve, b.Curve = subCurve(u.Curve, 0, k), subCurve(u.Curve, k, 1)
			}
			r = append(r, a, b)
		}
		return r, nil
	}
	for i, f := range faces {
		f.Outer, e = split(f.Outer)
		if e != nil {
			return Solid{}, EdgeSplit{}, e
		}
		for j, l := range f.Holes {
			f.Holes[j], e = split(l)
			if e != nil {
				return Solid{}, EdgeSplit{}, e
			}
		}
		faces[i] = f
	}
	return Solid{vertices: vertices, edges: edges, faces: faces, tolerance: s.tolerance}, h, nil
}

func subCurve(c PCurve, a, b float64) PCurve {
	if old, ok := c.(subTrim); ok {
		m, n := (1-a)*old.min+a*old.max, (1-b)*old.min+b*old.max
		return subTrim{old.base, m, n}
	}
	return subTrim{c, a, b}
}

type subTrim struct {
	base     PCurve
	min, max float64
}

func (c subTrim) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	return c.base.EvalUV((1-t)*c.min + t*c.max)
}
func (c subTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	d, ok := c.base.(differentiableTrim)
	if !ok {
		return geom.Vec2{}, fmt.Errorf("trimmed: underlying p-curve has no derivative")
	}
	p, e := d.DerivativeUV((1-t)*c.min + t*c.max)
	return scale2(p, c.max-c.min), e
}
