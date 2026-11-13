package geom

import (
	"context"
	"errors"
	"math/big"
)

// TubeIsoEndpointJunction proves equality of selected model endpoints, not
// interior branch overlap. Public rational endpoint data are caller-owned.
// Separated requires an exhaustive empty iso-curve intersection; unsuccessful
// rational proposals alone do not prove separation. Neither flag is unresolved.
type TubeIsoEndpointJunction struct {
	// FixedChartCoordinates identifies the same-chart level/inverse proof route.
	FixedChartCoordinates bool
	Connected, Separated  bool
	Left, Right           TubeEndpointCertificate
	Intersection          ExactCurveSegmentIntersection
	ReversedCurves        bool
	ExactOperations       int
}

// CertifyTubeEndpointsByIsoIntersection extracts exact coordinate-level curves
// from immutable proved tubes. For opposite charts, at least one extracted
// curve must admit exact affine parameter interpolation. For distinct axes on
// the same chart, their fixed UV is evaluated and the opposite chart must be
// proved affine for exact inversion. No global affine relation between free coordinates is
// assumed. Rational intersection proposals are independently checked against
// BOTH full surface-pair endpoint residuals and strict dependent boxes.
// Unrecovered algebraic events, coincident curves and unsupported subdomains
// return ErrConvergence. Options bound all work cumulatively (default 4M operations).
func CertifyTubeEndpointsByIsoIntersection(ctx context.Context, left, right VerifiedSurfaceTube, leftUpper, rightUpper bool, o ExactCurveSegmentOptions) (r TubeIsoEndpointJunction, err error) {
	if ctx == nil || left.a == nil || left.b == nil || right.a == nil || right.b == nil || left.axis < 0 || left.axis > 3 || right.axis < 0 || right.axis > 3 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if left.axis == right.axis {
		return r, ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	remaining := func() (int, error) {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		n := o.MaxOperations - r.ExactOperations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	selected := func(t VerifiedSurfaceTube, upper bool) (*SurfacePatch, float64) {
		p := t.a
		if t.axis >= 2 {
			p = t.b
		}
		d := p.Domain()
		lo, hi := d.UMin, d.UMax
		if t.axis%2 == 1 {
			lo, hi = d.VMin, d.VMax
		}
		if upper {
			return p, hi
		}
		return p, lo
	}
	lp, lv := selected(left, leftUpper)
	rp, rv := selected(right, rightUpper)
	if left.axis/2 == right.axis/2 {
		return certifySameChartLevelEndpoints(ctx, left, right, leftUpper, rightUpper, lv, rv, o)
	}

	n, e := remaining()
	if e != nil {
		return r, e
	}
	lc, e := lp.IsoCurve(left.axis%2, lv, SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	r.ExactOperations += lc.operations
	n, e = remaining()
	if e != nil {
		return r, e
	}
	rc, e := rp.IsoCurve(right.axis%2, rv, SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	r.ExactOperations += rc.operations
	n, e = remaining()
	if e != nil {
		return r, e
	}
	local := o
	local.MaxOperations = n
	intersections, e := IntersectExactCurveAffineLine(ctx, lc, rc, local)
	r.ExactOperations += intersections.Operations
	if e != nil && (!errors.Is(e, ErrConvergence) && !errors.Is(e, ErrDegenerate)) {
		return r, e
	}
	if e != nil && !intersections.Complete && len(intersections.Hits) == 0 {
		n, e = remaining()
		if e != nil {
			return r, e
		}
		local.MaxOperations = n
		intersections, e = IntersectExactCurveAffineLine(ctx, rc, lc, local)
		r.ExactOperations += intersections.Operations
		r.ReversedCurves = true
		if e != nil && (!errors.Is(e, ErrConvergence) && !errors.Is(e, ErrDegenerate)) {
			return r, e
		}
	}
	r.Intersection = intersections
	if intersections.Complete && len(intersections.Hits) == 0 {
		r.Separated = true
		return r, nil
	}
	for _, hit := range intersections.Hits {
		if hit.OtherParameter == nil {
			continue
		}
		parameters := [4]*big.Rat{}
		parameters[left.axis] = new(big.Rat).SetFloat64(lv)
		parameters[right.axis] = new(big.Rat).SetFloat64(rv)
		lvar, rvar := (left.axis/2)*2+1-left.axis%2, (right.axis/2)*2+1-right.axis%2
		lparameter, rparameter := hit.Parameter, hit.OtherParameter
		if r.ReversedCurves {
			lparameter, rparameter = rparameter, lparameter
		}
		parameters[lvar] = new(big.Rat).Set(lparameter)
		parameters[rvar] = new(big.Rat).Set(rparameter)
		n, e = remaining()
		if e != nil {
			return r, e
		}
		comparison, e := CompareVerifiedTubeEndpoints(ctx, left, right, leftUpper, rightUpper, parameters, parameters, TubeEndpointOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
		r.ExactOperations += comparison.ExactOperations
		if e != nil {
			if errors.Is(e, ErrDomain) || errors.Is(e, ErrConvergence) {
				continue
			}
			return r, e
		}
		if comparison.Complete && comparison.Equal {
			r.Connected = true
			r.Left, r.Right = comparison.Left, comparison.Right
			return r, nil
		}
	}
	return r, ErrConvergence
}

func certifySameChartLevelEndpoints(ctx context.Context, left, right VerifiedSurfaceTube, leftUpper, rightUpper bool, lv, rv float64, o ExactCurveSegmentOptions) (r TubeIsoEndpointJunction, err error) {
	r.FixedChartCoordinates = true
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	source, opposite := left.a, left.b
	if left.axis >= 2 {
		source, opposite = left.b, left.a
	}
	uv := [2]float64{}
	uv[left.axis%2] = lv
	uv[right.axis%2] = rv
	d := source.Domain()
	if uv[0] < d.UMin || uv[0] > d.UMax || uv[1] < d.VMin || uv[1] > d.VMax {
		return r, ErrConvergence
	}
	point, _, e := exactSurfaceJet(source, new(big.Rat).SetFloat64(uv[0]), new(big.Rat).SetFloat64(uv[1]), budget)
	if e != nil {
		return r, e
	}
	remaining := func() (int, error) {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		n := o.MaxOperations - budget.operations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	n, e := remaining()
	if e != nil {
		return r, e
	}
	chart, e := NewAffineSurfaceChart(ctx, opposite, AffineSurfaceChartOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	budget.operations += chart.operations
	n, e = remaining()
	if e != nil {
		return r, e
	}
	inv, e := chart.InvertExactContext(ctx, point, AffineChartInverseOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += inv.ExactOperations
	if e != nil {
		return r, e
	}
	if !inv.OnPlane || !inv.Inside {
		return r, ErrConvergence
	}
	hint := [4]*big.Rat{new(big.Rat).SetFloat64(uv[0]), new(big.Rat).SetFloat64(uv[1]), inv.U, inv.V}
	if left.axis >= 2 {
		hint = [4]*big.Rat{inv.U, inv.V, new(big.Rat).SetFloat64(uv[0]), new(big.Rat).SetFloat64(uv[1])}
	}
	n, e = remaining()
	if e != nil {
		return r, e
	}
	comparison, e := CompareVerifiedTubeEndpoints(ctx, left, right, leftUpper, rightUpper, hint, hint, TubeEndpointOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += comparison.ExactOperations
	if e != nil {
		return r, e
	}
	if !comparison.Complete || !comparison.Equal {
		return r, ErrConvergence
	}
	r.Connected = true
	r.Left, r.Right = comparison.Left, comparison.Right
	return r, nil
}
