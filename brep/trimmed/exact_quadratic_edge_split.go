package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplitExactQuadraticTrimEdge splits a graph model edge and both exact
// rational quadratic uses at one rational normalized parameter. The
// new UV and model vertices must both be exactly representable. The
// new model edges each keep a normalized exact trimmed evaluator.
func (s Solid) SplitExactQuadraticTrimEdge(ctx context.Context, edge int, t *big.Rat, o edgecheck.Options) (Solid, EdgeSplit, error) {
	return s.splitExactQuadraticTrimEdge(ctx, edge, t, o, false)
}

// SplitRationalEndpointQuadraticTrimEdge splits a shared exact quadratic
// edge at a rational parameter. Both owning charts retain the exact new
// UV endpoint; the model vertex stores a display point and has a separate
// exact rational identity available from RationalVertices.
func (s Solid) SplitRationalEndpointQuadraticTrimEdge(ctx context.Context, edge int, t *big.Rat, o edgecheck.Options) (Solid, EdgeSplit, error) {
	return s.splitExactQuadraticTrimEdge(ctx, edge, t, o, true)
}

func (s Solid) splitExactQuadraticTrimEdge(ctx context.Context, edge int, t *big.Rat, o edgecheck.Options, rationalEndpoints bool) (Solid, EdgeSplit, error) {
	var bad EdgeSplit
	if ctx == nil || edge < 0 || edge >= len(s.edges) || t == nil || t.Sign() <= 0 || t.Cmp(big.NewRat(1, 1)) >= 0 {
		return Solid{}, bad, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, bad, e
	}
	m := s.edges[edge]
	c, ok := m.Curve.(surfaceTrimCurve)
	if !ok || m.Range != (parametric.Interval{Max: 1}) {
		return Solid{}, bad, geom.ErrInvalid
	}
	q, ok := c.curve.(ExactQuadraticUV)
	base, _ := parametric.SurfaceMapping(c.surface)
	_, okSurface := base.(*geom.BSplineSurface)
	if !ok || !okSurface {
		return Solid{}, bad, geom.ErrInvalid
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	a, e := q.restrict(zero, t, rationalEndpoints)
	if e != nil {
		return Solid{}, bad, e
	}
	d, e := q.restrict(t, one, rationalEndpoints)
	if e != nil {
		return Solid{}, bad, e
	}
	j, e := exactQuadraticModelEndpoint(ctx, c, a, 1, geom.SurfacePatchOptions{
		MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits,
	})
	if e != nil {
		return Solid{}, bad, e
	}
	point, e := geom.RepresentedExactPoint(j)
	if rationalEndpoints {
		point, e = rationalDisplayPoint(j)
	}
	if e != nil {
		return Solid{}, bad, e
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	h := EdgeSplit{First: edge, Second: len(edges), Vertex: len(v)}
	v = append(v, point)
	curve := func(q ExactQuadraticUV) surfaceTrimCurve { return surfaceTrimCurve{c.surface, q.ends[0], q.ends[1], q} }
	edges[edge] = parametric.Edge{Start: m.Start, End: h.Vertex, Range: m.Range, Curve: curve(a)}
	edges = append(edges, parametric.Edge{Start: h.Vertex, End: m.End, Range: m.Range, Curve: curve(d)})
	owners := 0
	for i := range faces {
		f := &faces[i]
		rings := []*[]Use{&f.Outer}
		for j := range f.Holes {
			rings = append(rings, &f.Holes[j])
		}
		for _, ring := range rings {
			out := make([]Use, 0, len(*ring)+1)
			for _, u := range *ring {
				if u.Edge != edge {
					out = append(out, u)
					continue
				}
				owners++
				proven, e := compareSplineRationalTrimUse(ctx, m, f.Surface, u, o)
				if e != nil || !proven {
					if e == nil {
						e = geom.ErrConvergence
					}
					return Solid{}, bad, e
				}
				c, ok := u.Curve.(ExactQuadraticUV)
				if !ok {
					return Solid{}, bad, geom.ErrInvalid
				}
				k := t
				first, second := h.First, h.Second
				if u.Reversed {
					k = new(big.Rat).Sub(one, t)
					first, second = second, first
				}
				a, e := c.restrict(zero, k, rationalEndpoints)
				if e != nil {
					return Solid{}, bad, e
				}
				b, e := c.restrict(k, one, rationalEndpoints)
				if e != nil {
					return Solid{}, bad, e
				}
				out = append(out, Use{Edge: first, Reversed: u.Reversed, UVStart: a.ends[0], UVEnd: a.ends[1], Curve: a},
					Use{Edge: second, Reversed: u.Reversed, UVStart: b.ends[0], UVEnd: b.ends[1], Curve: b})
			}
			*ring = out
		}
	}
	if owners != 2 {
		return Solid{}, bad, geom.ErrConvergence
	}
	out, e := NewSolidContext(ctx, v, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, bad, e
	}
	if e := validateNativeExactQuadratics(ctx, out); e != nil {
		return Solid{}, bad, e
	}
	return out, h, nil
}

// MergeExactQuadraticTrimEdges joins two consecutively oriented split trims
// using the exact original normalized split parameter. Both homogeneous
// subcurve identities are reproved; geometric proximity is insufficient.
func (s Solid) MergeExactQuadraticTrimEdges(ctx context.Context, first, second int, split *big.Rat) (Solid, EdgeMergeHistory, error) {
	return s.mergeExactTrimEdges(ctx, first, second, split, func(a, b PCurve, t *big.Rat) (PCurve, [2]geom.Vec2, error) {
		x, xok := a.(ExactQuadraticUV)
		y, yok := b.(ExactQuadraticUV)
		if !xok || !yok {
			return nil, [2]geom.Vec2{}, geom.ErrInvalid
		}
		q, e := joinExactQuadraticControls(x, y, t)
		return q, q.ends, e
	})
}

func (s Solid) mergeExactTrimEdges(ctx context.Context, first, second int, split *big.Rat,
	join func(PCurve, PCurve, *big.Rat) (PCurve, [2]geom.Vec2, error)) (Solid, EdgeMergeHistory, error) {
	var bad EdgeMergeHistory
	if ctx == nil || first < 0 || second < 0 || first == second || first >= len(s.edges) || second >= len(s.edges) ||
		split == nil || split.Sign() <= 0 || split.Cmp(big.NewRat(1, 1)) >= 0 {
		return Solid{}, bad, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, bad, e
	}
	a, b := s.edges[first], s.edges[second]
	x, xok := a.Curve.(surfaceTrimCurve)
	y, yok := b.Curve.(surfaceTrimCurve)
	if a.End != b.Start || !xok || !yok || a.Range != (parametric.Interval{Max: 1}) || a.Range != b.Range || x.b != y.a {
		return Solid{}, bad, geom.ErrConvergence
	}
	as, aok := x.surface.(*geom.BSplineSurface)
	bs, bok := y.surface.(*geom.BSplineSurface)
	if !aok || !bok || !geom.SameSplineSurface(as, bs) {
		return Solid{}, bad, geom.ErrConvergence
	}
	whole, endpoints, e := join(x.curve, y.curve, split)
	if e != nil {
		return Solid{}, bad, e
	}
	middle := a.End
	for i, e := range s.edges {
		if i != first && i != second && (e.Start == middle || e.End == middle) {
			return Solid{}, bad, geom.ErrConvergence
		}
	}
	var owners int
	faces := s.Faces()
	for i := range faces {
		f := &faces[i]
		rings := []*[]Use{&f.Outer}
		for j := range f.Holes {
			rings = append(rings, &f.Holes[j])
		}
		for _, ring := range rings {
			if e := ctx.Err(); e != nil {
				return Solid{}, bad, e
			}
			u := *ring
			start := -1
			for j, x := range u {
				if x.Edge == first && !x.Reversed || x.Edge == second && x.Reversed {
					start = j
					break
				}
			}
			if start < 0 {
				for _, x := range u {
					if x.Edge == first || x.Edge == second {
						return Solid{}, bad, geom.ErrConvergence
					}
				}
				continue
			}
			if len(u) < 4 {
				return Solid{}, bad, geom.ErrConvergence
			}
			x, y := u[start], u[(start+1)%len(u)]
			want := second
			fraction := split
			if x.Reversed {
				want = first
				fraction = new(big.Rat).Sub(big.NewRat(1, 1), split)
			}
			if y.Edge != want || x.Reversed != y.Reversed || x.UVEnd != y.UVStart {
				return Solid{}, bad, geom.ErrConvergence
			}
			uv, ends, e := join(x.Curve, y.Curve, fraction)
			if e != nil {
				return Solid{}, bad, e
			}
			out := []Use{{Edge: first, Reversed: x.Reversed, UVStart: ends[0], UVEnd: ends[1], Curve: uv}}
			for j := 2; j < len(u); j++ {
				z := u[(start+j)%len(u)]
				if z.Edge == first || z.Edge == second {
					return Solid{}, bad, geom.ErrConvergence
				}
				out = append(out, z)
			}
			*ring = out
			owners++
		}
	}
	if owners != 2 {
		return Solid{}, bad, geom.ErrConvergence
	}
	h := EdgeMergeHistory{RemovedVertex: middle, Vertices: deletedEntityMap(len(s.vertices)), Edges: deletedEntityMap(len(s.edges))}
	var v []geom.Vec3
	for i, q := range s.vertices {
		if i != middle {
			h.Vertices[i] = len(v)
			v = append(v, q)
		}
	}
	var edges []parametric.Edge
	for i, e := range s.edges {
		if i == second {
			continue
		}
		h.Edges[i] = len(edges)
		if i == first {
			e.End, e.Curve = b.End, surfaceTrimCurve{x.surface, endpoints[0], endpoints[1], whole}
		}
		e.Start, e.End = h.Vertices[e.Start], h.Vertices[e.End]
		edges = append(edges, e)
	}
	h.Edge = h.Edges[first]
	h.Edges[second] = h.Edge
	for i := range faces {
		f := &faces[i]
		for _, ring := range append([][]Use{f.Outer}, f.Holes...) {
			for j := range ring {
				ring[j].Edge = h.Edges[ring[j].Edge]
			}
		}
	}
	out, e := NewSolidContext(ctx, v, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, bad, e
	}
	if e := validateNativeExactQuadratics(ctx, out); e != nil {
		return Solid{}, bad, e
	}
	return out, h, nil
}

func joinExactQuadraticControls(a, b ExactQuadraticUV, t *big.Rat) (ExactQuadraticUV, error) {
	if a.control[0][0] == nil || b.control[0][0] == nil || a.ends[1] != b.ends[0] ||
		t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return ExactQuadraticUV{}, geom.ErrConvergence
	}
	h := [3][3]*big.Rat{a.control[0], {}, b.control[2]}
	one := big.NewRat(1, 1)
	u := new(big.Rat).Sub(one, t)
	for i := range h[1] {
		h[1][i] = new(big.Rat).Quo(new(big.Rat).Sub(a.control[1][i], new(big.Rat).Mul(u, h[0][i])), t)
	}
	q, e := NewExactQuadraticUV(h)
	if a.rationalEndpoints || b.rationalEndpoints {
		q, e = NewRationalEndpointQuadraticUV(h)
	}
	if e != nil {
		return ExactQuadraticUV{}, e
	}
	check := func(actual, want ExactQuadraticUV) bool {
		for i, p := range actual.control {
			for j, x := range p {
				if x.Cmp(want.control[i][j]) != 0 {
					return false
				}
			}
		}
		return true
	}
	x, e := q.Restrict(new(big.Rat), t)
	if e != nil || !check(x, a) {
		return ExactQuadraticUV{}, geom.ErrConvergence
	}
	y, e := q.Restrict(t, one)
	if e != nil || !check(y, b) {
		return ExactQuadraticUV{}, geom.ErrConvergence
	}
	return q, nil
}
