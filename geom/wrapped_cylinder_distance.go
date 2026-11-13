package geom

import (
	"context"
	"math/big"
)

// CylinderDistanceBounds encloses global squared distance to all three boundary
// pieces. Witness is an exact rational boundary point, including cap-interior
// projections. Uniqueness is not asserted. Each returned value is caller-owned.
type CylinderDistanceBounds struct {
	Lower, Upper *big.Rat
	Witness      [3]*big.Rat
	Part         CylinderBoundaryPart
	VisitedCells int
}

// BoundPointWrappedCylinderDistance supports every canonical positive-weight
// rational cylinder profile, without a circular approximation. Extrusion
// separates XY profile distance from exact Z clamping. One certified exact
// curve search supplies the lateral and (when needed) cap-rim distances;
// exact conic membership permits cap-interior orthogonal projections.
// Options bound the curve search; proof, extraction and witness reconstruction
// have the same bounded per-stage arithmetic limits. No partial result escapes
// on failure. The lower bound includes outward conversion of the curve search;
// the upper bound is recomputed from the exact returned witness.
func BoundPointWrappedCylinderDistance(ctx context.Context, w *WrappedSurfacePatch, q Vec3, o CurveDistanceOptions) (CylinderDistanceBounds, error) {
	empty := CylinderDistanceBounds{}
	if !q.IsFinite() {
		return empty, ErrInvalid
	}
	po := SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	proof, e := CertifyWrappedCylinderEmbedding(ctx, w, po)
	if e != nil {
		return empty, e
	}
	options, e := patchOptions(po)
	if e != nil {
		return empty, e
	}
	b := &polynomialBudget{operations: proof.ExactOperations, maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	// Exact membership of query XY in the convex quadrant-profile disk.
	u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(q.X), proof.CenterX), proof.Radius)
	v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(q.Y), proof.CenterY), proof.Radius)
	quadrant := 0
	if u.Sign() < 0 {
		if v.Sign() >= 0 {
			quadrant = 1
			u, v = new(big.Rat).Set(v), new(big.Rat).Neg(u)
		} else {
			quadrant = 2
			u.Neg(u)
			v.Neg(v)
		}
	} else if v.Sign() < 0 {
		quadrant = 3
		u, v = new(big.Rat).Neg(v), new(big.Rat).Set(u)
	}
	one := big.NewRat(1, 1)
	inside := u.Cmp(one) <= 0 && v.Cmp(one) <= 0
	sum := new(big.Rat).Sub(new(big.Rat).Add(u, v), one)
	if inside && sum.Sign() >= 0 {
		j := 4 * quadrant
		wa, wb, wc := w.patch.net[j][3], w.patch.net[j+2][3], w.patch.net[j+4][3]
		left := new(big.Rat).Mul(new(big.Rat).Mul(wa, wc), new(big.Rat).Mul(sum, sum))
		right := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(wb, wb))
		right.Mul(right, new(big.Rat).Sub(one, u))
		right.Mul(right, new(big.Rat).Sub(one, v))
		for _, r := range []*big.Rat{left, right} {
			if e := b.check(r); e != nil {
				return empty, e
			}
		}
		inside = left.Cmp(right) <= 0
	}
	qz := new(big.Rat).SetFloat64(q.Z)
	if inside && (qz.Cmp(proof.LowerZ) <= 0 || qz.Cmp(proof.UpperZ) >= 0) {
		// The axial distance to the solid's entire height interval is
		// a lower bound. An orthogonal projection into the cap attains
		// it exactly, so no lateral subdivision can improve the result.
		z, part := proof.UpperZ, CylinderUpperCap
		if qz.Cmp(proof.LowerZ) <= 0 {
			z, part = proof.LowerZ, CylinderLowerCap
		}
		d := new(big.Rat).Sub(qz, z)
		squared := d.Mul(d, d)
		if e := b.check(squared); e != nil {
			return empty, e
		}
		return CylinderDistanceBounds{Lower: squared, Upper: new(big.Rat).Set(squared),
			Witness: [3]*big.Rat{new(big.Rat).SetFloat64(q.X), new(big.Rat).SetFloat64(q.Y), new(big.Rat).Set(z)}, Part: part}, nil
	}
	curve, e := w.patch.IsoCurve(1, w.patch.Domain().VMin, po)
	if e != nil {
		return empty, e
	}
	// Preserve exact XY coefficients; flatten only the query-independent height.
	flat := *curve.patch
	flat.net = make([]exactHomogeneous, len(curve.patch.net))
	for i, h := range curve.patch.net {
		for k := 0; k < 4; k++ {
			flat.net[i][k] = new(big.Rat).Set(h[k])
		}
		flat.net[i][2].SetInt64(0)
	}
	if e := flat.computeBounds(b); e != nil {
		return empty, e
	}
	flat.operations = b.operations
	planar := &SurfaceIsoCurve{patch: &flat, fixedAxis: -1, operations: b.operations}
	distance, e := BoundPointExactCurveDistance(ctx, Vec3{X: q.X, Y: q.Y}, planar, o)
	if e != nil {
		return empty, e
	}
	if !distance.Complete || !distance.HasWitness || !finite(distance.SquaredDistanceMin) {
		return empty, ErrConvergence
	}
	sample, e := flat.EvaluateRational(ctx, new(big.Rat).SetFloat64(distance.Parameter), big.NewRat(0, 1), SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return empty, e
	}
	lowerXY := new(big.Rat).SetFloat64(distance.SquaredDistanceMin)
	query := [3]*big.Rat{new(big.Rat).SetFloat64(q.X), new(big.Rat).SetFloat64(q.Y), qz}
	upperXY := new(big.Rat)
	for k := 0; k < 2; k++ {
		d := new(big.Rat).Sub(sample.Point[k], query[k])
		upperXY.Add(upperXY, new(big.Rat).Mul(d, d))
	}
	clamp := new(big.Rat).Set(query[2])
	if clamp.Cmp(proof.LowerZ) < 0 {
		clamp.Set(proof.LowerZ)
	}
	if clamp.Cmp(proof.UpperZ) > 0 {
		clamp.Set(proof.UpperZ)
	}
	result := CylinderDistanceBounds{VisitedCells: distance.VisitedCells}
	for i, z := range []*big.Rat{clamp, proof.LowerZ, proof.UpperZ} {
		d := new(big.Rat).Sub(query[2], z)
		dz2 := new(big.Rat).Mul(d, d)
		low, high := new(big.Rat).Add(lowerXY, dz2), new(big.Rat).Add(upperXY, dz2)
		xy := [2]*big.Rat{sample.Point[0], sample.Point[1]}
		part := CylinderLateral
		if i > 0 {
			part = CylinderLowerCap
			if i == 2 {
				part = CylinderUpperCap
			}
			if inside {
				low.Set(dz2)
				high.Set(dz2)
				xy = [2]*big.Rat{query[0], query[1]}
			}
		}
		if result.Lower == nil || low.Cmp(result.Lower) < 0 {
			result.Lower = new(big.Rat).Set(low)
		}
		if result.Upper == nil || high.Cmp(result.Upper) < 0 {
			result.Upper = new(big.Rat).Set(high)
			result.Part = part
			result.Witness = [3]*big.Rat{new(big.Rat).Set(xy[0]), new(big.Rat).Set(xy[1]), new(big.Rat).Set(z)}
		}
	}
	return result, nil
}
