package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SphereCylinderCommonRegime identifies which native construction owns the
// result of a finite off-axis intersection.
type SphereCylinderCommonRegime uint8

const (
	SphereCylinderCommonEmpty SphereCylinderCommonRegime = iota
	SphereCylinderCommonFullEnds
	SphereCylinderCommonOneLens
	SphereCylinderCommonOppositeLenses
	SphereCylinderCommonSameSideLenses
)

// SphereFiniteOffsetCommon includes native face ancestry and exact source cap
// material, indexed by lower and upper planes in the caller's frame.
type SphereFiniteOffsetCommon struct {
	Solid                             Solid
	Regime                            SphereCylinderCommonRegime
	WallFaces                         []int
	PlanarEndFaces, SphericalEndFaces [2][]int
	CapMaterial                       [2]geom.SphereCylinderCapMaterial
}

// SphereFiniteOffsetCylinderCommon selects a native analytic intersection
// for full ends, one lens with a full planar or spherical opposite end,
// or two strict lenses.
// Strictly disjoint and externally tangent inputs return empty regularized
// material. Mixed regimes outside the implemented charts fail atomically.
func SphereFiniteOffsetCylinderCommon(ctx context.Context, frame geom.Frame, R, r, d, lower, upper, tolerance float64) (SphereFiniteOffsetCommon, error) {
	var empty SphereFiniteOffsetCommon
	if ctx == nil || !finite(R) || !finite(r) || !finite(d) || !finite(lower) || !finite(upper) || !finite(tolerance) ||
		R <= 0 || r <= 0 || d < 0 || lower >= upper || tolerance <= 0 || frame.ZAxis().Length() == 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(a *big.Rat) *big.Rat { return new(big.Rat).Mul(a, a) }
	gap := new(big.Rat).Sub(q(d), q(r))
	if gap.Sign() < 0 {
		gap.SetInt64(0)
	}
	axial := 0.
	if lower > 0 {
		axial = lower
	} else if upper < 0 {
		axial = upper
	}
	if new(big.Rat).Add(sq(gap), sq(q(axial))).Cmp(sq(q(R))) >= 0 {
		return empty, nil
	}
	var out SphereFiniteOffsetCommon
	lenses := [2]bool{}
	for i, h := range [2]float64{lower, upper} {
		c, e := geom.ClassifySphereCylinderCapMaterial(ctx, geom.Vec3{}, R, geom.Vec3{X: d}, geom.Vec3{Z: 1}, r, q(h), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			return empty, e
		}
		out.CapMaterial[i] = c
		lenses[i] = c.Kind == geom.SphereCylinderCapLens && !c.Tangent
	}
	switch {
	case lenses[0] && lenses[1]:
		if lower < 0 && upper > 0 {
			c, e := SphereOffAxisCylinderDoubleCommon(ctx, frame, R, r, d, lower, upper, tolerance)
			if e != nil {
				return empty, e
			}
			out.Solid, out.Regime, out.WallFaces = c.Solid, SphereCylinderCommonOppositeLenses, c.CylindricalFaces
			out.PlanarEndFaces, out.SphericalEndFaces = c.PlanarEndFaces, c.SphericalEndFaces
		} else {
			c, e := SphereOffAxisCylinderSameSideCommon(ctx, frame, R, r, d, lower, upper, tolerance)
			if e != nil {
				return empty, e
			}
			out.Solid, out.Regime, out.WallFaces = c.Solid, SphereCylinderCommonSameSideLenses, c.CylindricalFaces
			out.PlanarEndFaces = c.PlanarEndFaces
			j := 0
			if lower > 0 {
				j = 1
			}
			out.SphericalEndFaces[j] = c.SphericalFaces
		}
	case lenses[1]:
		c, e := SphereOffAxisCylinderChordCommon(ctx, frame, R, r, d, lower, upper, tolerance)
		if e != nil {
			return empty, e
		}
		out.Solid, out.Regime, out.WallFaces = c.Solid, SphereCylinderCommonOneLens, c.CylindricalFaces
		out.PlanarEndFaces = [2][]int{c.LowerCap, c.UpperCap}
		out.SphericalEndFaces = [2][]int{c.LowerSphericalFaces, c.SphericalFaces}
	case lenses[0]:
		// Flip both transverse Y and axial Z, preserving the frame's
		// handedness and the offset along its X axis.
		flip, e := geom.NewFrame(frame.Origin(), frame.XAxis(), frame.ZAxis().Scale(-1))
		if e != nil {
			return empty, e
		}
		c, e := SphereOffAxisCylinderChordCommon(ctx, flip, R, r, d, -upper, -lower, tolerance)
		if e != nil {
			return empty, e
		}
		out.Solid, out.Regime, out.WallFaces = c.Solid, SphereCylinderCommonOneLens, c.CylindricalFaces
		out.PlanarEndFaces = [2][]int{c.UpperCap, c.LowerCap}
		out.SphericalEndFaces = [2][]int{c.SphericalFaces, c.LowerSphericalFaces}
	default:
		c, e := SphereOffAxisCylinderCommon(ctx, frame, R, r, d, lower, upper, tolerance)
		if e != nil {
			return empty, e
		}
		out.Solid, out.Regime, out.WallFaces = c.Solid, SphereCylinderCommonFullEnds, c.CylindricalFaces
		faces := c.Solid.Faces()
		for i, a := range [2][]int{c.LowerEndFaces, c.UpperEndFaces} {
			s := faces[a[0]].Surface.(offsetCommonChart)
			if i == 0 && s.lowerSpherical || i == 1 && s.upperSpherical {
				out.SphericalEndFaces[i] = a
			} else {
				out.PlanarEndFaces[i] = a
			}
		}
	}
	return out, nil
}
