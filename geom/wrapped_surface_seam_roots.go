package geom

import (
	"context"
	"math/big"
)

// WrappedArcSeamRootJunction retains an exact isolating interval for the common
// varying coordinate, which may be algebraic. No rounded point is promoted to a
// root. DistinctParameters concerns the quotient chart, not spatial separation.
type WrappedArcSeamRootJunction struct {
	Matched, DistinctParameters, TransverseContinuation, LeftUpper bool
	C0Continuation                                                 bool
	TangentSeamSign                                                int
	Lower, Upper                                                   *big.Rat
	ExactOperations                                                int
}

// CompareArcSeamRoots matches opposite boundary root identities by exact common
// polynomial factors and root count. Continuation requires stored C1 seam proof
// and a strict homogeneous plane-numerator derivative bound on both arc cells.
// Its scope excludes corner roots and entire coincident seam edges.
func (w *WrappedSurfacePatch) CompareArcSeamRoots(ctx context.Context, left SurfacePlaneArc, leftEnd int, right SurfacePlaneArc, rightEnd, axis int, o ArcExactEndpointOptions) (r WrappedArcSeamRootJunction, err error) {
	return w.compareArcSeamRoots(ctx, left, leftEnd, right, rightEnd, axis, o, false)
}

func (w *WrappedSurfacePatch) compareArcSeamRoots(ctx context.Context, left SurfacePlaneArc, leftEnd int, right SurfacePlaneArc, rightEnd, axis int, o ArcExactEndpointOptions, allowC0 bool) (r WrappedArcSeamRootJunction, err error) {
	if ctx == nil || w == nil || w.patch == nil || axis < 0 || axis > 1 || leftEnd < 0 || leftEnd > 1 || rightEnd < 0 || rightEnd > 1 || left.source == nil || right.source == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if w.period[axis] == nil {
		return r, ErrDomain
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
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	a, c := left.Endpoints[leftEnd].identity, right.Endpoints[rightEnd].identity
	canonical := func(id *surfaceRootIdentity) *surfaceRootIdentity {
		if id != nil && id.axis != axis && id.lo.Cmp(id.hi) == 0 {
			copy := *id
			copy.axis = axis
			copy.fixed = id.lo
			copy.lo = id.fixed
			copy.hi = id.fixed
			return &copy
		}
		return id
	}
	a, c = canonical(a), canonical(c)
	if a == nil || c == nil || a.axis != axis || c.axis != axis {
		return r, ErrConvergence
	}
	d := w.Domain()
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	lo, hi := new(big.Rat).SetFloat64(limits[axis][0]), new(big.Rat).SetFloat64(limits[axis][1])
	if a.fixed.Cmp(lo) == 0 && c.fixed.Cmp(hi) == 0 {
	} else if a.fixed.Cmp(hi) == 0 && c.fixed.Cmp(lo) == 0 {
		r.LeftUpper = true
	} else {
		return r, ErrDomain
	}
	for _, id := range []*surfaceRootIdentity{a, c} {
		if id.lo.Cmp(new(big.Rat).SetFloat64(limits[1-axis][0])) <= 0 || id.hi.Cmp(new(big.Rat).SetFloat64(limits[1-axis][1])) >= 0 {
			return r, ErrConvergence
		}
	}
	lp, rp := exactPlaneCoefficients(left.plane), exactPlaneCoefficients(right.plane)
	for i := range lp {
		if lp[i].Cmp(rp[i]) != 0 {
			return r, ErrDomain
		}
	}
	signs := [2]int{}
	for i, arc := range []SurfacePlaneArc{left, right} {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		patch, e := trimPatchBudget(w.patch, arc.source.Domain(), b)
		if e != nil {
			return r, e
		}
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		eq, e := CompareExactSurfaces(ctx, patch, arc.source, ExactSurfaceComparisonOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += eq.ExactOperations
		if e != nil {
			return r, e
		}
		if !eq.Complete || !eq.Equal {
			return r, ErrDomain
		}
		data, e := arc.source.arcData(arc.plane, b)
		if e != nil {
			return r, e
		}
		signs[i] = data.dv[4].sign()
		if axis == 1 {
			signs[i] = -data.du[4].sign()
		}
	}
	// Shift only the fixed seam coordinate: physical varying-coordinate
	// polynomials and their certified isolating intervals are unchanged.
	shifted := *c
	shifted.fixed = a.fixed
	matched, e := exactSurfaceRootMatch(a, &shifted, b)
	if e != nil {
		return r, e
	}
	if !matched {
		r.DistinctParameters = true
		return r, nil
	}
	r.Lower = new(big.Rat).Set(a.lo)
	if c.lo.Cmp(r.Lower) > 0 {
		r.Lower.Set(c.lo)
	}
	r.Upper = new(big.Rat).Set(a.hi)
	if c.hi.Cmp(r.Upper) < 0 {
		r.Upper.Set(c.hi)
	}
	r.Matched = true
	if signs[0] != 0 && signs[0] == signs[1] && (w.seam.FirstPartialsMatched[axis] || allowC0 && w.seam.PositionMatched[axis]) {
		r.TransverseContinuation = true
		r.TangentSeamSign = signs[0]
		r.C0Continuation = !w.seam.FirstPartialsMatched[axis]
	}
	return r, nil
}
