package geom

import (
	"context"
	"math/big"
)

// SphereLensRim is the exact source circle of a transverse sphere intersection.
// Its plane is Axis·(p−Center)=0 and its radius is sqrt(RadiusSquared). Axis is
// the original directed center displacement, not a rounded unit vector.
// CenterFraction is the fraction along that axis from the first center.
// NormalDot is the dot product of the original outward unit normals anywhere
// on the rim. All rational values belong to the caller.
type SphereLensRim struct {
	Center, Axis                               [3]*big.Rat
	RadiusSquared, AxisSquared, CenterFraction *big.Rat
	NormalDot                                  *big.Rat
}

// TransverseLensRim derives and verifies the original sphere-circle equations
// without square roots of the center distance or rim radius. The result is
// independent of any native/analytic support representation or tessellation.
func (s SpherePairMaterial) TransverseLensRim(ctx context.Context, o SurfacePatchOptions) (SphereLensRim, error) {
	if ctx == nil || s.radius2[0] == nil {
		return SphereLensRim{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return SphereLensRim{}, err
	}
	if !s.transverse || s.op != SphereSetIntersection {
		return SphereLensRim{}, ErrDomain
	}
	limits, err := patchOptions(o)
	if err != nil {
		return SphereLensRim{}, err
	}
	a := &sphereOverlapArithmetic{ctx: ctx, b: &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}}
	for i, center := range s.center {
		for _, coordinate := range center {
			a.check(coordinate)
		}
		a.check(s.radius2[i])
	}
	return s.deriveLensRim(a)
}

func (s SpherePairMaterial) deriveLensRim(a *sphereOverlapArithmetic) (SphereLensRim, error) {
	var r SphereLensRim
	for j := range r.Axis {
		r.Axis[j] = a.sub(s.center[1][j], s.center[0][j])
	}
	two := big.NewRat(2, 1)
	r.AxisSquared = a.distance(s.center[0], s.center[1])
	r.CenterFraction = a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), r.AxisSquared), a.mul(two, r.AxisSquared))
	r.RadiusSquared = a.sub(s.radius2[0], a.mul(a.mul(r.CenterFraction, r.CenterFraction), r.AxisSquared))
	for j := range r.Center {
		r.Center[j] = a.add(s.center[0][j], a.mul(r.CenterFraction, r.Axis[j]))
	}
	radiiProduct := a.mul(a.sqrt(s.radius2[0]), a.sqrt(s.radius2[1]))
	r.NormalDot = a.div(a.sub(a.add(s.radius2[0], s.radius2[1]), r.AxisSquared), a.mul(two, radiiProduct))
	if a.err != nil {
		return SphereLensRim{}, a.err
	}
	if r.RadiusSquared.Sign() <= 0 || new(big.Rat).Abs(r.NormalDot).Cmp(big.NewRat(1, 1)) >= 0 {
		return SphereLensRim{}, ErrConvergence
	}
	for i, c := range s.center {
		surface := a.add(a.distance(r.Center, c), r.RadiusSquared)
		if a.err != nil {
			return SphereLensRim{}, a.err
		}
		if surface.Cmp(s.radius2[i]) != 0 {
			return SphereLensRim{}, ErrConvergence
		}
	}
	return r, nil
}
