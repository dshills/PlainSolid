package geom

import (
	"context"
	"math/big"
)

// WrappedArcSeamJunction compares endpoints on opposite sides of one fundamental
// seam. DistinctParameters separates points in the quotient parameter chart,
// not necessarily model points on a self-intersecting surface. Matched alone
// proves identification, not unique continuation at a singular plane contact.
// TransverseContinuation additionally requires the stored C1 seam proof and a
// nonzero exact plane tangent component across the seam. TangentSeamSign is the
// sign of (f_v,-f_u)[axis], for the plane functional f and original UV units.
// LeftUpper records the side of Left. Returned rational endpoints are owned by
// the caller; their mutation cannot change the proof-carrying source arcs.
type WrappedArcSeamJunction struct {
	Matched, DistinctParameters, TransverseContinuation bool
	LeftUpper                                           bool
	TangentSeamSign                                     int
	Left, Right                                         SurfaceArcExactEndpoint
	ExactOperations                                     int
}

// CompareArcSeamEndpoints verifies the arc supports against the immutable
// fundamental patch, then compares recovered rational boundary roots exactly.
// Both arcs must use the identical oriented plane equation. Unrecovered
// algebraic roots remain ErrConvergence; overlap of rounded boxes never matches.
// This pairwise primitive does not prove global uniqueness of a stitching graph.
func (w *WrappedSurfacePatch) CompareArcSeamEndpoints(ctx context.Context, left SurfacePlaneArc, leftEnd int, right SurfacePlaneArc, rightEnd, axis int, o ArcExactEndpointOptions) (r WrappedArcSeamJunction, err error) {
	if ctx == nil || w == nil || w.patch == nil || axis < 0 || axis > 1 || left.source == nil || right.source == nil {
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
	lp, rp := exactPlaneCoefficients(left.plane), exactPlaneCoefficients(right.plane)
	for i := range lp {
		if lp[i].Cmp(rp[i]) != 0 {
			return r, ErrDomain
		}
	}
	arcs := []SurfacePlaneArc{left, right}
	indices := []int{leftEnd, rightEnd}
	ends := []*SurfaceArcExactEndpoint{&r.Left, &r.Right}
	for i, a := range arcs {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		p, e := trimPatchBudget(w.patch, a.source.Domain(), b)
		if e != nil {
			return r, e
		}
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		eq, e := CompareExactSurfaces(ctx, p, a.source, ExactSurfaceComparisonOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += eq.ExactOperations
		if e != nil {
			return r, e
		}
		if !eq.Complete || !eq.Equal {
			return r, ErrDomain
		}
		remaining = o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		ep, e := a.ExactEndpoint(indices[i], ArcExactEndpointOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += ep.ExactOperations
		if e != nil {
			return r, e
		}
		*ends[i] = ep
	}
	d := w.Domain()
	lo, hi := d.UMin, d.UMax
	if axis == 1 {
		lo, hi = d.VMin, d.VMax
	}
	lower, upper := new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)
	l, h := r.Left.Parameters[axis], r.Right.Parameters[axis]
	if l.Cmp(lower) == 0 && h.Cmp(upper) == 0 {
		r.LeftUpper = false
	} else if l.Cmp(upper) == 0 && h.Cmp(lower) == 0 {
		r.LeftUpper = true
	} else {
		return r, ErrDomain
	}
	if r.Left.Parameters[1-axis].Cmp(r.Right.Parameters[1-axis]) != 0 {
		r.DistinctParameters = true
		return r, nil
	}
	for i := 0; i < 3; i++ {
		if r.Left.Point[i].Cmp(r.Right.Point[i]) != 0 {
			return r, ErrConvergence
		}
	}
	if w.seam.FirstPartialsMatched[axis] {
		_, jet, e := exactSurfaceJet(left.source, r.Left.Parameters[0], r.Left.Parameters[1], b)
		if e != nil {
			return r, e
		}
		derivative := new(big.Rat)
		for i := 0; i < 3; i++ {
			derivative.Add(derivative, new(big.Rat).Mul(lp[i], jet[i][1-axis]))
			if e := b.check(derivative); e != nil {
				return r, e
			}
		}
		r.TangentSeamSign = derivative.Sign()
		if axis == 1 {
			r.TangentSeamSign = -r.TangentSeamSign
		}
		r.TransverseContinuation = r.TangentSeamSign != 0
	}
	r.Matched = true
	return r, nil
}
