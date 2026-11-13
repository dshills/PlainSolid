package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SteinmetzNegativeCap retains the finite negative-X tail of the equal-radius
// cylinder common. Its exact ideal volume enclosure and angular seam bound
// have the same semantics as SteinmetzPositiveCap; Cap is strictly negative.
type SteinmetzNegativeCap SteinmetzPositiveCap

// PerpendicularCylinderNegativeCap constructs X≤center.X+cap, -radius<cap<0.
// The exact Pythagorean ordinate and validated angular split are reused from
// the positive-cap proof. Four analytic faces enclose the tail; no cap is
// replaced by a sampled plane or projected approximate seam.
func PerpendicularCylinderNegativeCap(ctx context.Context, center geom.Vec3, radius, cap, xOtherHalfLength, zHalfLength, tolerance float64) (SteinmetzNegativeCap, error) {
	fail := func(e error) (SteinmetzNegativeCap, error) { return SteinmetzNegativeCap{}, e }
	if !finite(cap) || cap >= 0 || cap <= -radius {
		return fail(geom.ErrInvalid)
	}
	// This positive counterpart supplies an independent complete angular and
	// source-volume proof. Its topology is not used as the tail boundary.
	proof, e := PerpendicularCylinderPositiveCap(ctx, center, radius, -cap, xOtherHalfLength, zHalfLength, tolerance)
	if e != nil {
		return fail(e)
	}
	return buildNegativeSteinmetzCap(ctx, center, radius, cap, tolerance, proof)
}
func buildNegativeSteinmetzCap(ctx context.Context, center geom.Vec3, radius, cap, tolerance float64, proof SteinmetzPositiveCap) (SteinmetzNegativeCap, error) {
	fail := func(e error) (SteinmetzNegativeCap, error) { return SteinmetzNegativeCap{}, e }
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	capX, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(center.X), q(cap)))
	if e != nil {
		return fail(e)
	}
	u := Interval{-proof.SplitAngle, proof.SplitAngle}
	faces := []Face{{Surface: steinmetzChart{center, radius, 0, -1}, U: u, V: Interval{-1, 1}, Reversed: true}}
	for _, sign := range []int{1, -1} {
		faces = append(faces, Face{Surface: steinmetzClippedCylinder{center, radius, cap, sign}, U: u, V: Interval{0, 1}, Reversed: sign > 0})
	}
	faces = append(faces, Face{Surface: steinmetzDisk{geom.Vec3{X: capX, Y: center.Y, Z: center.Z}, radius}, U: u, V: Interval{-1, 1}})
	solid, e := joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	full := new(big.Rat).Mul(big.NewRat(16, 3), new(big.Rat).Mul(new(big.Rat).Mul(q(radius), q(radius)), q(radius)))
	bounds := [2]*big.Rat{new(big.Rat).Sub(full, proof.IdealVolumeBounds[1]), new(big.Rat).Sub(full, proof.IdealVolumeBounds[0])}
	return SteinmetzNegativeCap{Solid: solid, Cap: cap, SeamOrdinate: proof.SeamOrdinate, SplitAngle: proof.SplitAngle, XAxisFaces: []int{1, 2}, ZAxisFaces: []int{0}, CapFace: 3, SeamSquaredBound: new(big.Rat).Set(proof.SeamSquaredBound), IdealVolumeBounds: bounds}, nil
}
