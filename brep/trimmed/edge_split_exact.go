package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplitSplineEdgeExact splits every use of one spline model edge at the
// given parameter, including a certified closed spline edge. All uses must
// be straight isoparametric spline trims
// proved to agree with the model edge. Their split UVs and the exact model
// point must be representable; there is no tolerance snapping or UV drift.
func (s Solid) SplitSplineEdgeExact(ctx context.Context, edgeID int, parameter float64, o edgecheck.Options) (Solid, EdgeSplit, error) {
	var h EdgeSplit
	if ctx == nil || edgeID < 0 || edgeID >= len(s.edges) || !finite(parameter) {
		return Solid{}, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	edge := s.edges[edgeID]
	if edge.Curve == nil || parameter <= edge.Range.Min || parameter >= edge.Range.Max {
		return Solid{}, h, geom.ErrDomain
	}
	descriptor, reverse, e := exactEdgeSplineWithOptions(edge, geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
	if e != nil {
		return Solid{}, h, e
	}
	if descriptor == nil {
		return Solid{}, h, geom.ErrInvalid
	}
	fraction := exactRectEdgeFraction(parameter, edge.Range.Min, edge.Range.Max)
	t := new(big.Rat).Set(fraction)
	if reverse {
		t.Sub(big.NewRat(1, 1), t)
	}
	lo, hi := descriptor.Domain()
	u, exact := exactRectEdgeMix(t, lo, hi)
	if !exact {
		return Solid{}, h, geom.ErrUnrepresentable
	}
	point, e := descriptor.ExactPoint(u)
	if e != nil {
		return Solid{}, h, e
	}
	if point == s.vertices[edge.Start] || point == s.vertices[edge.End] {
		return Solid{}, h, geom.ErrUnrepresentable
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	h = EdgeSplit{First: edgeID, Second: len(edges), Vertex: len(v)}
	v = append(v, point)
	edges[edgeID] = parametric.Edge{Start: edge.Start, End: h.Vertex, Curve: edge.Curve,
		Range: parametric.Interval{Min: edge.Range.Min, Max: parameter}}
	edges = append(edges, parametric.Edge{Start: h.Vertex, End: edge.End, Curve: edge.Curve,
		Range: parametric.Interval{Min: parameter, Max: edge.Range.Max}})
	count := 0
	for i, f := range faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, EdgeSplit{}, e
		}
		apply := func(loop []Use) ([]Use, error) {
			out := make([]Use, 0, len(loop)+1)
			for _, use := range loop {
				if use.Edge != edgeID {
					out = append(out, use)
					continue
				}
				count++
				if !sameSplineTrim(edge, f.Surface, use) {
					q, e := compareSplineEdgeUse(ctx, edge, f.Surface, use, o)
					if e != nil {
						return nil, e
					}
					if !q {
						return nil, fmt.Errorf("trimmed: exact model edge agreement not proved: %w", geom.ErrConvergence)
					}
				}
				if use.Curve != nil || use.UVStart.X != use.UVEnd.X && use.UVStart.Y != use.UVEnd.Y {
					return nil, geom.ErrInvalid
				}
				t := new(big.Rat).Set(fraction)
				if use.Reversed {
					t.Sub(big.NewRat(1, 1), t)
				}
				x, xe := exactRectEdgeMix(t, use.UVStart.X, use.UVEnd.X)
				y, ye := exactRectEdgeMix(t, use.UVStart.Y, use.UVEnd.Y)
				p := geom.Vec2{X: x, Y: y}
				if !xe || !ye || p == use.UVStart || p == use.UVEnd {
					return nil, geom.ErrUnrepresentable
				}
				a, b := use, use
				a.UVEnd, b.UVStart = p, p
				if use.Reversed {
					a.Edge = h.Second
				} else {
					b.Edge = h.Second
				}
				out = append(out, a, b)
			}
			return out, nil
		}
		f.Outer, e = apply(f.Outer)
		if e != nil {
			return Solid{}, EdgeSplit{}, e
		}
		for j, loop := range f.Holes {
			f.Holes[j], e = apply(loop)
			if e != nil {
				return Solid{}, EdgeSplit{}, e
			}
		}
		faces[i] = f
	}
	if count != 2 {
		return Solid{}, EdgeSplit{}, geom.ErrInvalid
	}
	out, e := NewSolidContext(ctx, v, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, EdgeSplit{}, e
	}
	return out, h, nil
}
