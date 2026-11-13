package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplitExactBezierTrimEdge splits both owning cubic or quartic trims and the
// shared normalized model edge at the same exact rational event. It preserves
// source surfaces and proves agreement and trim topology before publication.
// The new UV and model vertex coordinates must be exactly representable.
func (s Solid) SplitExactBezierTrimEdge(ctx context.Context, edge int, t *big.Rat, options edgecheck.Options) (Solid, EdgeSplit, error) {
	var zero EdgeSplit
	if ctx == nil || edge < 0 || edge >= len(s.edges) || t == nil {
		return Solid{}, zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, zero, e
	}
	m := s.edges[edge]
	model, ok := m.Curve.(surfaceTrimCurve)
	q, exact := model.curve.(ExactBezierUV)
	if !ok || !exact || m.Range != (parametric.Interval{Max: 1}) {
		return Solid{}, zero, geom.ErrInvalid
	}
	left, right, e := q.Split(t)
	if e != nil {
		return Solid{}, zero, e
	}
	patch, e := exactFaceSupportPatch(Face{Surface: model.surface}, geom.SurfacePatchOptions{
		MaxOperations: options.MaxWork, MaxCoefficientBits: options.MaxBits,
	})
	if e != nil {
		return Solid{}, zero, e
	}
	uv := left.ends[1]
	jet, e := patch.EvaluateRational(ctx, ratInset(uv.X), ratInset(uv.Y), geom.SurfacePatchEvaluationOptions{
		MaxOperations: options.MaxWork, MaxCoefficientBits: options.MaxBits,
	})
	if e != nil {
		return Solid{}, zero, e
	}
	point, e := geom.RepresentedExactPoint(jet.Point)
	if e != nil {
		return Solid{}, zero, e
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	h := EdgeSplit{First: edge, Second: len(edges), Vertex: len(vertices)}
	vertices = append(vertices, point)
	makeEdge := func(start, end int, q ExactBezierUV) parametric.Edge {
		return parametric.Edge{Start: start, End: end, Range: m.Range, Curve: surfaceTrimCurve{model.surface, q.ends[0], q.ends[1], q}}
	}
	edges[edge] = makeEdge(m.Start, h.Vertex, left)
	edges = append(edges, makeEdge(h.Vertex, m.End, right))
	owners := 0
	for i := range faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, zero, e
		}
		f := &faces[i]
		rings := []*[]Use{&f.Outer}
		for j := range f.Holes {
			rings = append(rings, &f.Holes[j])
		}
		for _, ring := range rings {
			position := -1
			for j, u := range *ring {
				if u.Edge != edge {
					continue
				}
				if position >= 0 {
					return Solid{}, zero, geom.ErrConvergence
				}
				position = j
			}
			if position >= 0 {
				j, u := position, (*ring)[position]
				owners++
				c, ok := u.Curve.(ExactBezierUV)
				if !ok {
					return Solid{}, zero, geom.ErrConvergence
				}
				proven, e := comparePolynomialTrimUse(ctx, m, f.Surface, u, options)
				if e != nil {
					return Solid{}, zero, e
				}
				if !proven {
					return Solid{}, zero, geom.ErrConvergence
				}
				local, first, second := t, h.First, h.Second
				if u.Reversed {
					local, first, second = new(big.Rat).Sub(big.NewRat(1, 1), t), second, first
				}
				a, b, e := c.Split(local)
				if e != nil {
					return Solid{}, zero, e
				}
				// The source chart's normalized t is verified above, so the
				// split represents the same exact model point on this owner.
				before, after := (*ring)[:j], (*ring)[j+1:]
				next := make([]Use, 0, len(*ring)+1)
				next = append(next, before...)
				next = append(next, Use{Edge: first, Reversed: u.Reversed, UVStart: a.ends[0], UVEnd: a.ends[1], Curve: a},
					Use{Edge: second, Reversed: u.Reversed, UVStart: b.ends[0], UVEnd: b.ends[1], Curve: b})
				*ring = append(next, after...)
			}
		}
	}
	if owners != 2 {
		return Solid{}, zero, geom.ErrConvergence
	}
	result, e := NewSolidContext(ctx, vertices, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, zero, e
	}
	if e := validateNativeExactQuadratics(ctx, result); e != nil {
		return Solid{}, zero, e
	}
	return result, h, nil
}
