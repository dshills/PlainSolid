package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// This additive gate admits only the same complete exact Bezier chart and
// domain. No affine map or curve-only agreement substitutes for support identity.
func sameExactBezierMergeSupport(ctx context.Context, a, b geom.Surface) (bool, error) {
	x, xok := a.(*geom.ExactBezierSurface)
	y, yok := b.(*geom.ExactBezierSurface)
	if !xok || !yok || x == nil || y == nil {
		return false, nil
	}
	r, e := geom.CompareExactSurfaces(ctx, x.ExactPatch(), y.ExactPatch(), geom.ExactSurfaceComparisonOptions{MaxCells: 1, MaxOperations: 1000000, MaxCoefficientBits: 16384})
	return e == nil && r.Complete && r.Equal, e
}

func sameAuthoredBezierSpline(a, b *geom.BSpline) bool {
	if a == nil || b == nil || a.Degree() != b.Degree() || a.Degree() < 3 || a.Degree() > 4 {
		return false
	}
	ap, bp := a.Poles(), b.Poles()
	ak, bk := a.Knots(), b.Knots()
	aw, bw := a.Weights(), b.Weights()
	if len(ap) != len(bp) || len(ak) != len(bk) || len(aw) != len(bw) || len(ap) > 64 || len(ak) > 64 {
		return false
	}
	for i := range ap {
		if ap[i] != bp[i] {
			return false
		}
	}
	for i := range ak {
		if ak[i] != bk[i] {
			return false
		}
	}
	for i := range aw {
		if aw[i] != bw[i] {
			return false
		}
	}
	return true
}

// At most eight active spans/64 authored controls are admitted by the shared
// descriptor proof. Each of the whole/two restriction descriptors and two full
// comparisons has a separate bounded 1M-operation/16384-bit proof, as the native
// merge API has no caller work options. Opt-in mode normalization adds at most
// eight exact endpoint evaluations, each bounded as WithExactEndpoints. Failure
// never publishes partial topology.
func joinStoredBezierSources(ctx context.Context, a, b SplineUV, t *big.Rat) (PCurve, [2]geom.Vec2, error) {
	var zero [2]geom.Vec2
	if !sameAuthoredBezierSpline(a.Curve, b.Curve) || a.Reversed != b.Reversed || !finite(a.Range.Min) || !finite(a.Range.Max) || !finite(b.Range.Min) || !finite(b.Range.Max) || a.Range.Min >= a.Range.Max || b.Range.Min >= b.Range.Max {
		return nil, zero, geom.ErrConvergence
	}
	q := a
	if a.Reversed {
		if a.Range.Min != b.Range.Max {
			return nil, zero, geom.ErrConvergence
		}
		q.Range.Min = b.Range.Min
	} else {
		if a.Range.Max != b.Range.Min {
			return nil, zero, geom.ErrConvergence
		}
		q.Range.Max = b.Range.Max
	}
	width := new(big.Rat).Sub(ratInset(q.Range.Max), ratInset(q.Range.Min))
	first := new(big.Rat).Sub(ratInset(a.Range.Max), ratInset(a.Range.Min))
	if new(big.Rat).Quo(first, width).Cmp(t) != 0 {
		return nil, zero, geom.ErrConvergence
	}
	limits := geom.SurfacePatchOptions{MaxOperations: 1000000, MaxCoefficientBits: 16384}
	curve, e := mixedStoredHigherSplineUV(ctx, q, limits)
	if e != nil {
		return nil, zero, e
	}
	whole := curve.(ExactBezierUV)
	left, right, e := whole.Split(t)
	if e != nil {
		return nil, zero, e
	}
	for i, source := range []SplineUV{a, b} {
		actual, e := polynomialTrimBezierDescriptor(ctx, source, limits)
		if e != nil {
			return nil, zero, e
		}
		expected, e := []ExactBezierUV{left, right}[i].descriptor(ctx, limits)
		if e != nil {
			return nil, zero, e
		}
		proof, e := geom.CompareExactCurvesContext(ctx, actual, expected, geom.ExactCurveComparisonOptions{MaxSpans: 8, MaxOperations: 1000000, MaxCoefficientBits: 16384})
		if e != nil {
			return nil, zero, e
		}
		if !proof.Complete || !proof.Equal {
			return nil, zero, geom.ErrConvergence
		}
	}
	if a.ExactEndpoints || b.ExactEndpoints {
		// Evaluation mode is not source identity. Reprove the actual endpoint
		// behavior on each child before normalizing the restored whole range.
		// This does not repair an invalid legacy child whose endpoint rounded.
		for i, source := range []SplineUV{a, b} {
			ends := []ExactBezierUV{left, right}[i].ends
			for j := 0; j < 2; j++ {
				p, e := source.EvalUV(float64(j))
				if e != nil {
					return nil, zero, e
				}
				if p != ends[j] {
					return nil, zero, geom.ErrConvergence
				}
			}
		}
		q.ExactEndpoints = false
		legacy := true
		for j := 0; j < 2; j++ {
			p, e := q.EvalUV(float64(j))
			if e != nil || p != whole.ends[j] {
				legacy = false
			}
		}
		if !legacy {
			q, e = q.WithExactEndpoints()
			if e != nil {
				return nil, zero, e
			}
			for j := 0; j < 2; j++ {
				p, e := q.EvalUV(float64(j))
				if e != nil || p != whole.ends[j] {
					return nil, zero, geom.ErrConvergence
				}
			}
		}
	}
	return q, whole.ends, nil
}
