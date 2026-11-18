//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedFrustumBoreDistance encloses the global minimum squared distance to
// the retained material boundary. Face is the witness's canonical original
// owner; native receivers translate it to their actual face index.
type WrappedFrustumBoreDistance struct {
	geom.FrustumDistanceBounds
	Face int
}

// BoundPointWrappedEccentricFrustumBoreDistance uses the actual walls and
// retained cap domains of an exactly contained circular bore. At an opening,
// perpendicular projection is accepted only on annular material; otherwise
// its closest point lies on the appropriate original circle. Removed disks
// contribute no distance witnesses. Blind bores, tubes and cavities qualify.
func BoundPointWrappedEccentricFrustumBoreDistance(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, p geom.Vec3,
	o geom.FrustumDistanceOptions) (WrappedFrustumBoreDistance, error) {
	if !p.IsFinite() {
		return WrappedFrustumBoreDistance{}, geom.ErrInvalid
	}
	return BoundPointWrappedEccentricFrustumBoreDistanceRational(ctx, outer, inner, [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}, o)
}

// BoundPointWrappedEccentricFrustumBoreDistanceRational retains exact model
// input, including the choice between exposed annulus and removed disk.
func BoundPointWrappedEccentricFrustumBoreDistanceRational(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, p [3]*big.Rat,
	o geom.FrustumDistanceOptions) (WrappedFrustumBoreDistance, error) {
	var empty WrappedFrustumBoreDistance
	if ctx == nil || !finite(o.SquaredTolerance) || o.SquaredTolerance <= 0 || o.MaxBisections < 0 || o.MaxBisections > 4096 || o.Surface.MaxCoefficientBits < 0 || o.Surface.MaxOperations < 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	bits := o.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	for _, v := range p {
		if v == nil {
			return empty, geom.ErrInvalid
		}
		if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
			return empty, geom.ErrConvergence
		}
	}
	if o.Surface.MaxOperations == 0 {
		o.Surface.MaxOperations = 1000000
	}
	work := 0
	remaining := func() (geom.FrustumDistanceOptions, error) {
		q := o
		q.Surface.MaxOperations -= work
		if q.Surface.MaxOperations < 1 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	a, e := geom.CertifyWrappedFrustumEmbedding(ctx, outer, o.Surface)
	if e != nil {
		return empty, e
	}
	work += a.ExactOperations
	limit, e := remaining()
	if e != nil {
		return empty, e
	}
	b, e := geom.CertifyWrappedFrustumEmbedding(ctx, inner, limit.Surface)
	if e != nil {
		return empty, e
	}
	if !strictWrappedParallelFrustumBore(a, b, false) {
		return empty, geom.ErrConvergence
	}
	work += b.ExactOperations
	var winner WrappedFrustumBoreDistance
	var lower *big.Rat
	keep := func(q geom.FrustumDistanceBounds, face int) {
		work += q.ExactOperations
		if lower == nil || q.Lower.Cmp(lower) < 0 {
			lower = q.Lower
		}
		if winner.Upper == nil || q.Upper.Cmp(winner.Upper) < 0 {
			winner = WrappedFrustumBoreDistance{FrustumDistanceBounds: q, Face: face}
		}
	}
	for i, w := range [2]*geom.WrappedSurfacePatch{outer, inner} {
		limit, e := remaining()
		if e != nil {
			return empty, e
		}
		q, e := geom.BoundPointWrappedCircularFrustumPartsDistanceRational(ctx, w, p, geom.CylinderLateral, limit)
		if e != nil {
			return empty, e
		}
		keep(q, 3*i)
	}
	pointRadiusSquared := func(axis [2]*big.Rat) *big.Rat {
		x := new(big.Rat).Sub(p[0], axis[0])
		y := new(big.Rat).Sub(p[1], axis[1])
		return new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
	}
	axisA, axisB := [2]*big.Rat{a.CenterX, a.CenterY}, [2]*big.Rat{b.CenterX, b.CenterY}
	pA, pB := pointRadiusSquared(axisA), pointRadiusSquared(axisB)
	zA, zB := [2]*big.Rat{a.LowerZ, a.UpperZ}, [2]*big.Rat{b.LowerZ, b.UpperZ}
	rA, rB := [2]*big.Rat{a.LowerRadius, a.UpperRadius}, [2]*big.Rat{b.LowerRadius, b.UpperRadius}
	innerFace := 4
	for i, part := range [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap} {
		axis, radius, disk := axisA, rA[i], true
		open := zA[i].Cmp(zB[i]) == 0
		if open && pB.Cmp(new(big.Rat).Mul(rB[i], rB[i])) < 0 {
			// The projected query is in the removed hole. The exact
			// nearest point on the inner circle is inside the outer disk.
			axis, radius, disk = axisB, rB[i], false
		} else if open && pA.Cmp(new(big.Rat).Mul(rA[i], rA[i])) > 0 {
			axis, radius, disk = axisA, rA[i], false
		}
		limit, e := remaining()
		if e != nil {
			return empty, e
		}
		q, e := geom.BoundCircularRadialBoundaryDistanceRational(ctx, axis, radius, zA[i], p, disk, part, limit)
		if e != nil {
			return empty, e
		}
		keep(q, i+1)
		if !open {
			limit, e = remaining()
			if e != nil {
				return empty, e
			}
			q, e = geom.BoundCircularRadialBoundaryDistanceRational(ctx, axisB, rB[i], zB[i], p, true, part, limit)
			if e != nil {
				return empty, e
			}
			keep(q, innerFace)
			innerFace++
		}
	}
	winner.Lower = new(big.Rat).Set(lower)
	winner.ExactOperations = work
	if new(big.Rat).Sub(winner.Upper, winner.Lower).Cmp(ratInset(o.SquaredTolerance)) > 0 {
		return empty, geom.ErrConvergence
	}
	return winner, nil
}

// BoundPointCanonicalWrappedFrustumBoreDistance re-proves a native circular
// bore (including reindexed/reloaded solids) and preserves its face ancestry.
func (s Solid) BoundPointCanonicalWrappedFrustumBoreDistance(ctx context.Context, p geom.Vec3,
	o geom.FrustumDistanceOptions) (WrappedFrustumBoreDistance, error) {
	if !p.IsFinite() {
		return WrappedFrustumBoreDistance{}, geom.ErrInvalid
	}
	return s.BoundPointCanonicalWrappedFrustumBoreDistanceRational(ctx, [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}, o)
}

// BoundPointCanonicalWrappedFrustumBoreDistanceRational preserves a rational
// query and the actual original boundary owner on a native receiver.
func (s Solid) BoundPointCanonicalWrappedFrustumBoreDistanceRational(ctx context.Context, p [3]*big.Rat,
	o geom.FrustumDistanceOptions) (WrappedFrustumBoreDistance, error) {
	proof, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Surface)
	if e != nil {
		return WrappedFrustumBoreDistance{}, e
	}
	q, e := BoundPointWrappedEccentricFrustumBoreDistanceRational(ctx, proof.Outer, proof.Inner, p, o)
	if e != nil {
		return WrappedFrustumBoreDistance{}, e
	}
	q.Face = canonicalFrustumBoreLineFaceIDs(proof)[q.Face]
	return q, nil
}
