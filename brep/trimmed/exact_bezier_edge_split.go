package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplitExactBezierTrimEdge splits both owning cubic or quartic trims and the
// shared normalized model edge at the same exact rational event. It preserves
// source surfaces and proves agreement and trim topology before publication.
// Original degree-three/four SplineUV owners with a proved single Bezier trace
// retain their authored curve and restricted parameter ranges (including reversal
// and redundant knots). Their exact split parameter must also be representable.
// MappedSplineUV owners retain their exact original source and derive positive
// child speed maps, with independent full-curve comparison on both children.
// The new UV and model vertex coordinates must be exactly representable;
// existing native endpoint evaluation checks also remain in force.
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
				c, e := splitStoredBezierDescriptor(ctx, u.Curve, options)
				if e != nil {
					return Solid{}, zero, e
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
				sourceA, sourceB, e := splitStoredBezierSources(ctx, u.Curve, local, a, b, options)
				if e != nil {
					return Solid{}, zero, e
				}
				// The source chart's normalized t is verified above, so the
				// split represents the same exact model point on this owner.
				before, after := (*ring)[:j], (*ring)[j+1:]
				next := make([]Use, 0, len(*ring)+1)
				next = append(next, before...)
				next = append(next, Use{Edge: first, Reversed: u.Reversed, UVStart: a.ends[0], UVEnd: a.ends[1], Curve: sourceA},
					Use{Edge: second, Reversed: u.Reversed, UVStart: b.ends[0], UVEnd: b.ends[1], Curve: sourceB})
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

// MergeExactBezierTrimEdges rejoins a coherent cubic or quartic split. It
// reconstructs the whole homogeneous curve from the first subcurve, then
// independently verifies both subcurves and their owning face loops. The
// intermediate vertex must not belong to any other edge. Original SplineUV
// children additionally require unchanged authored curves, exactly touching raw
// ranges, and the exact original split ratio; both normalized restrictions are
// independently reproved before the authored full range is restored. Exact
// MappedSplineUV children require exact adjacent source intervals and the
// independently proved child Möbius coefficient identities. Ordinary spline
// endpoint modes are normalized to legacy only when both whole-source endpoint
// evaluations equal their independently proved exact values.
func (s Solid) MergeExactBezierTrimEdges(ctx context.Context, first, second int, split *big.Rat) (Solid, EdgeMergeHistory, error) {
	return s.mergeExactTrimEdges(ctx, first, second, split, func(a, b PCurve, t *big.Rat) (PCurve, [2]geom.Vec2, error) {
		if _, ok := a.(MultiSpanMappedSplineUV); ok {
			return joinPiecewiseMappedSources(ctx, a, b, t)
		}
		if _, ok := a.(PiecewiseMappedSplineUV); ok {
			return joinPiecewiseMappedSources(ctx, a, b, t)
		}
		if x, ok := a.(MappedSplineUV); ok {
			if y, ok := b.(MappedSplineUV); ok {
				q, ends, e := joinMappedSplineSources(ctx, x, y, t)
				if e == nil || !errors.Is(e, geom.ErrConvergence) {
					return q, ends, e
				}
			}
			return joinPiecewiseMappedSources(ctx, a, b, t)
		}
		if x, ok := a.(SplineUV); ok {
			y, ok := b.(SplineUV)
			if !ok {
				return nil, [2]geom.Vec2{}, geom.ErrInvalid
			}
			return joinStoredBezierSources(ctx, x, y, t)
		}
		x, xok := a.(ExactBezierUV)
		y, yok := b.(ExactBezierUV)
		if !xok || !yok {
			return nil, [2]geom.Vec2{}, geom.ErrInvalid
		}
		q, e := joinExactBezierControls(x, y, t)
		return q, q.ends, e
	})
}

func joinExactBezierControls(a, b ExactBezierUV, t *big.Rat) (ExactBezierUV, error) {
	var zero ExactBezierUV
	n := len(a.control)
	if n < 4 || n > 5 || n != len(b.control) || a.ends[1] != b.ends[0] ||
		t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return zero, geom.ErrConvergence
	}
	// The endpoint of each de Casteljau row is a triangular change of
	// basis: L_i = sum choose(i,j)*(1-t)^(i-j)*t^j*C_j. Recover C_i
	// in order, then certify that BOTH original nets are the actual split.
	u := new(big.Rat).Sub(big.NewRat(1, 1), t)
	powT, powU := [5]*big.Rat{big.NewRat(1, 1)}, [5]*big.Rat{big.NewRat(1, 1)}
	for i := 1; i < n; i++ {
		powT[i] = new(big.Rat).Mul(t, powT[i-1])
		powU[i] = new(big.Rat).Mul(u, powU[i-1])
	}
	h := make([][3]*big.Rat, n)
	for i := range h {
		for axis, control := range a.control[i] {
			x := new(big.Rat).Set(control)
			for j := 0; j < i; j++ {
				w := new(big.Rat).Mul(powT[j], powU[i-j])
				w.Mul(w, big.NewRat(polynomialChoose(i, j), 1))
				x.Sub(x, w.Mul(w, h[j][axis]))
			}
			h[i][axis] = x.Quo(x, powT[i])
		}
	}
	q, e := NewExactBezierUV(h)
	if e != nil {
		return zero, e
	}
	l, r, e := q.Split(t)
	if e != nil {
		return zero, e
	}
	for i := range l.control {
		for j := range l.control[i] {
			if l.control[i][j].Cmp(a.control[i][j]) != 0 || r.control[i][j].Cmp(b.control[i][j]) != 0 {
				return zero, geom.ErrConvergence
			}
		}
	}
	return q, nil
}
