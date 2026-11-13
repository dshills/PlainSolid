package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ObliqueSkewCylinderCommonResult retains the source support faces and
// exact placement evidence. Face group zero belongs to the first cylinder.
type ObliqueSkewCylinderCommonResult struct {
	Solid         Solid
	CylinderFaces [2][]int
	Placement     geom.ObliqueSkewCylinderMap
}

// ObliqueSkewCylinderCommon maps an orthogonal section solid to two skew
// circular cylinders. The source map and signed axis separation must be
// exactly representable. Conservative exact cap clearance is required;
// tangent and unhandled overlap transitions fail without returning a shell.
func ObliqueSkewCylinderCommon(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (ObliqueSkewCylinderCommonResult, error) {
	var empty ObliqueSkewCylinderCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if _, e := geom.NewCylinderPairCommon(a, b); e != nil {
		return empty, e
	}
	m, e := geom.NewObliqueSkewCylinderMap(ctx, a.Origin, a.Axis, a.Radius, b.Origin, b.Axis, b.Radius, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	if m.ProjectionRelation < 0 {
		return ObliqueSkewCylinderCommonResult{Placement: m}, nil
	}
	if m.ProjectionRelation == 0 {
		return empty, geom.ErrConvergence
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for i, s := range [2]geom.RawCylinder{a, b} {
		t := m.ClosestRawParameters[i]
		h := m.RawAxialHalfExtents[i].Bounds[1]
		if new(big.Rat).Sub(t, q(s.Min)).Cmp(h) <= 0 || new(big.Rat).Sub(q(s.Max), t).Cmp(h) <= 0 {
			return empty, geom.ErrConvergence
		}
	}
	xform, e := m.RepresentedAffine()
	if e != nil {
		return empty, e
	}
	// The orthogonal charts use (a,y,b), while the exact map uses (a,b,y).
	xform = xform.Mul(geom.Affine{{1, 0, 0, 0}, {0, 0, 1, 0}, {0, 1, 0, 0}})
	// Recover the signed separation using any nonzero exact component
	// of the shared perpendicular unit vector.
	shift := new(big.Rat)
	permutation := [3]float64{xform[0][1], xform[1][1], xform[2][1]}
	for i, x := range permutation {
		if x != 0 {
			shift.Quo(new(big.Rat).Sub(m.ClosestFeet[1][i], m.ClosestFeet[0][i]), q(x))
			break
		}
	}
	h, e := geom.RepresentedExactScalar(shift)
	if e != nil {
		return empty, e
	}
	// These local caps are artificial; the proof above covers the original
	// raw cap windows. Values are placed beyond both local extents.
	extent := 4 * max(a.Radius, b.Radius)
	if !finite(extent) {
		return empty, geom.ErrUnrepresentable
	}
	gap := new(big.Rat).Abs(new(big.Rat).Sub(q(a.Radius), q(b.Radius)))
	var local SteinmetzCommon
	if new(big.Rat).Abs(shift).Cmp(gap) <= 0 || shift.Sign() == 0 {
		local, e = SkewOrthogonalCylinderCommon(ctx, geom.Vec3{}, a.Radius, b.Radius, h, extent, extent, tolerance)
	} else {
		local, e = SkewOrthogonalOverlapCommon(ctx, geom.Vec3{}, a.Radius, b.Radius, h, extent, extent, tolerance)
	}
	if e != nil {
		return empty, e
	}
	out, e := local.Solid.Transformed(xform)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return ObliqueSkewCylinderCommonResult{Solid: out, CylinderFaces: [2][]int{local.XAxisFaces, local.ZAxisFaces}, Placement: m}, nil
}
