package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SpherePairOffsetHistory tracks the two original support faces in input
// order. The outer sphere precedes the inner sphere for a cavity. An absent
// result face has ID -1. Radii are the exact signed radius expressions before
// regularization.
type SpherePairOffsetHistory struct {
	SourceFaces, ResultFaces [2]int
	Radii                    [2]*big.Rat
}

func emptySpherePairOffsetHistory() SpherePairOffsetHistory {
	return SpherePairOffsetHistory{SourceFaces: [2]int{-1, -1}, ResultFaces: [2]int{-1, -1}}
}

// OffsetSphericalCavity offsets a checked ball difference with a strictly
// contained inner ball. A positive offset dilates the exterior and shrinks
// the cavity, which disappears at radius zero. A negative offset erodes
// the exterior and expands the removed ball. Its regularized result can
// have a transverse opening or be empty. An isolated pinch at a tangent
// inner wall is outside the native solid domain. Retained sphere radii
// must be exactly representable. The tolerance applies to native sewing.
func (s SpherePairSet) OffsetSphericalCavity(ctx context.Context, distance, tolerance float64) (Solid, SpherePairOffsetHistory, error) {
	var empty Solid
	noHistory := emptySpherePairOffsetHistory()
	if ctx == nil || !s.valid || !finite(distance) || !finite(tolerance) || tolerance <= 0 {
		return empty, noHistory, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, noHistory, err
	}
	balls, source, err := s.sphereOffsetOwners(Difference, true)
	if err != nil {
		return empty, noHistory, err
	}
	outer, inner := balls[0], balls[1]
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	centerDistance2 := sphereOffsetCenterDistanceSquared(outer.Center, inner.Center)
	a, b := q(outer.Radius), q(inner.Radius)
	slack := new(big.Rat).Sub(a, b)
	squaredSlack := new(big.Rat).Mul(slack, slack)
	if slack.Sign() <= 0 || centerDistance2.Cmp(squaredSlack) >= 0 {
		return empty, noHistory, geom.ErrDomain
	}
	d := q(distance)
	// Dilation uses the two exposed full supports of the strictly enclosed
	// cavity. Erosion distributes over the intersection of the outer ball
	// with the complement of the cavity, including after a wall is opened.
	radius := [2]*big.Rat{new(big.Rat).Add(a, d), new(big.Rat).Sub(b, d)}
	history := SpherePairOffsetHistory{SourceFaces: source, ResultFaces: [2]int{-1, -1}, Radii: radius}
	if d.Sign() == 0 {
		history.ResultFaces = source
		return s.solid, history, nil
	}
	if radius[0].Sign() <= 0 {
		return empty, history, nil
	}
	// Exact containment decides collapse and tangency before binary64
	// conversion or any tolerance-dependent native boundary construction.
	slack.Sub(radius[0], radius[1])
	squaredSlack.Mul(slack, slack)
	comparison := centerDistance2.Cmp(squaredSlack)
	if slack.Sign() <= 0 && comparison <= 0 {
		return empty, history, nil
	}
	if radius[1].Sign() > 0 && comparison == 0 {
		return empty, noHistory, geom.ErrDomain
	}
	outer.Radius, err = geom.RepresentedExactScalar(radius[0])
	if err != nil {
		return empty, noHistory, err
	}
	if radius[1].Sign() <= 0 {
		ball, err := Sphere(outer.Center, outer.Radius)
		if err != nil {
			return empty, noHistory, err
		}
		if err = ctx.Err(); err != nil {
			return empty, noHistory, err
		}
		history.ResultFaces[0] = 0
		return ball, history, nil
	}
	inner.Radius, err = geom.RepresentedExactScalar(radius[1])
	if err != nil {
		return empty, noHistory, err
	}
	offset, err := NewSpherePairSet(ctx, outer, inner, Difference, tolerance)
	if err != nil {
		return empty, noHistory, err
	}
	for i, owners := range offset.faces {
		if len(owners) != 1 {
			return empty, noHistory, geom.ErrDomain
		}
		history.ResultFaces[i] = owners[0]
	}
	return offset.solid, history, nil
}

func (s SpherePairSet) sphereOffsetOwners(operation BooleanOp, fullSurface bool) (balls [2]Ball, source [2]int, err error) {
	if s.operation != operation {
		return balls, source, geom.ErrDomain
	}
	for i, owners := range s.faces {
		if len(owners) != 1 {
			return balls, source, geom.ErrDomain
		}
		face := s.solid.faces[owners[0]]
		surface, ok := face.Surface.(geom.SphereSurface)
		if !ok || face.Reversed != (i == 1 && operation == Difference) || fullSurface && (face.V.Min != -math.Pi/2 || face.V.Max != math.Pi/2) {
			return balls, source, geom.ErrDomain
		}
		frame, radius := surface.Parameters()
		balls[i] = Ball{Center: frame.Origin(), Radius: radius}
		source[i] = owners[0]
	}
	return balls, source, nil
}

func sphereOffsetCenterDistanceSquared(a, b geom.Vec3) *big.Rat {
	squared := new(big.Rat)
	for _, coordinate := range [3][2]float64{{a.X, b.X}, {a.Y, b.Y}, {a.Z, b.Z}} {
		d := new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate[0]), new(big.Rat).SetFloat64(coordinate[1]))
		squared.Add(squared, new(big.Rat).Mul(d, d))
	}
	return squared
}
