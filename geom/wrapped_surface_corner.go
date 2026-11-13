package geom

import (
	"context"
	"math/big"
)

// WrappedArcCornerJunction proves a transverse crossing through a corner of a
// doubly periodic chart. Direction is +1 when canonical plane tangent (f_v,-f_u)
// points into the fundamental rectangle, -1 when it points out. Both components
// must be strictly directed into/out of the corresponding corner quadrant.
type WrappedArcCornerJunction struct {
	Matched, TransverseContinuation bool
	Direction                       [2]int
	TangentSigns                    [2]int
	Left, Right                     SurfaceArcExactEndpoint
	ExactOperations                 int
}

// CompareArcCornerEndpoints requires exact corner parameters, exact support
// identity, and C1 closure in both axes. Nonentering quadrants or a tangent
// parallel to either seam remain unresolved. Pairwise success does not establish
// uniqueness against any additional arcs; the wrapped trace assembler checks it.
func (w *WrappedSurfacePatch) CompareArcCornerEndpoints(ctx context.Context, left SurfacePlaneArc, leftEnd int, right SurfacePlaneArc, rightEnd int, o ArcExactEndpointOptions) (r WrappedArcCornerJunction, err error) {
	if ctx == nil || w == nil || w.patch == nil || left.source == nil || right.source == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if w.period[0] == nil || w.period[1] == nil {
		return r, ErrDomain
	}
	if !w.seam.FirstPartialsMatched[0] || !w.seam.FirstPartialsMatched[1] {
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
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	lp, rp := exactPlaneCoefficients(left.plane), exactPlaneCoefficients(right.plane)
	for i := range lp {
		if lp[i].Cmp(rp[i]) != 0 {
			return r, ErrDomain
		}
	}
	d := w.Domain()
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	var signs [2][2]int
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
		remaining = o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		index := leftEnd
		if i == 1 {
			index = rightEnd
		}
		ep, e := arc.ExactEndpoint(index, ArcExactEndpointOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += ep.ExactOperations
		if e != nil {
			return r, e
		}
		inward := [2]int{}
		for axis := 0; axis < 2; axis++ {
			if ep.Parameters[axis].Cmp(new(big.Rat).SetFloat64(limits[axis][0])) == 0 {
				inward[axis] = 1
			} else if ep.Parameters[axis].Cmp(new(big.Rat).SetFloat64(limits[axis][1])) == 0 {
				inward[axis] = -1
			} else {
				return r, ErrDomain
			}
		}
		_, jet, e := exactSurfaceJet(arc.source, ep.Parameters[0], ep.Parameters[1], b)
		if e != nil {
			return r, e
		}
		for axis := 0; axis < 2; axis++ {
			v := new(big.Rat)
			for k := 0; k < 3; k++ {
				v.Add(v, new(big.Rat).Mul(lp[k], jet[k][1-axis]))
				if e := b.check(v); e != nil {
					return r, e
				}
			}
			signs[i][axis] = v.Sign()
			if axis == 1 {
				signs[i][axis] = -signs[i][axis]
			}
		}
		direction := signs[i][0] * inward[0]
		if direction == 0 || signs[i][1]*inward[1] != direction {
			return r, ErrConvergence
		}
		r.Direction[i] = direction
		if i == 0 {
			r.Left = ep
		} else {
			r.Right = ep
		}
	}
	for k := 0; k < 3; k++ {
		if r.Left.Point[k].Cmp(r.Right.Point[k]) != 0 {
			return r, ErrConvergence
		}
	}
	if signs[0] != signs[1] || r.Direction[0] == r.Direction[1] {
		return r, ErrConvergence
	}
	r.TangentSigns = signs[0]
	r.Matched = true
	r.TransverseContinuation = true
	return r, nil
}
