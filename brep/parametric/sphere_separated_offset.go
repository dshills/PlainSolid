package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// OffsetSeparatedSpheres offsets the union of two strictly separated balls.
// Dilation distributes over union and can join the expanding balls through
// a transverse intersection. Erosion acts independently on the original
// separated components and discards a component when its radius reaches
// zero. An exact tangent join remains outside the native solid domain.
// Retained sphere radii must be exactly representable.
func (s SpherePairSet) OffsetSeparatedSpheres(ctx context.Context, distance, tolerance float64) (Solid, SpherePairOffsetHistory, error) {
	var empty Solid
	noHistory := emptySpherePairOffsetHistory()
	if ctx == nil || !s.valid || !finite(distance) || !finite(tolerance) || tolerance <= 0 {
		return empty, noHistory, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, noHistory, err
	}
	balls, source, err := s.fullSphereOffsetOwners(false)
	if err != nil {
		return empty, noHistory, err
	}
	a, b := new(big.Rat).SetFloat64(balls[0].Radius), new(big.Rat).SetFloat64(balls[1].Radius)
	distanceSquared := sphereOffsetCenterDistanceSquared(balls[0].Center, balls[1].Center)
	sum := new(big.Rat).Add(a, b)
	if distanceSquared.Cmp(new(big.Rat).Mul(sum, sum)) <= 0 {
		return empty, noHistory, geom.ErrDomain
	}
	d := new(big.Rat).SetFloat64(distance)
	radius := [2]*big.Rat{new(big.Rat).Add(a, d), new(big.Rat).Add(b, d)}
	history := SpherePairOffsetHistory{SourceFaces: source, ResultFaces: [2]int{-1, -1}, Radii: radius}
	if d.Sign() == 0 {
		history.ResultFaces = source
		return s.solid, history, nil
	}
	retained, sole := 0, -1
	for i, r := range radius {
		if r.Sign() > 0 {
			retained++
			sole = i
		}
	}
	if retained == 0 {
		return empty, history, nil
	}
	if retained == 2 {
		sum.Add(radius[0], radius[1])
		if distanceSquared.Cmp(new(big.Rat).Mul(sum, sum)) == 0 {
			return empty, noHistory, geom.ErrDomain
		}
	}
	for i, r := range radius {
		if r.Sign() <= 0 {
			continue
		}
		balls[i].Radius, err = geom.RepresentedExactScalar(r)
		if err != nil {
			return empty, noHistory, err
		}
	}
	if retained == 1 {
		solid, err := Sphere(balls[sole].Center, balls[sole].Radius)
		if err != nil {
			return empty, noHistory, err
		}
		if err = ctx.Err(); err != nil {
			return empty, noHistory, err
		}
		history.ResultFaces[sole] = 0
		return solid, history, nil
	}
	offset, err := NewSpherePairSet(ctx, balls[0], balls[1], Union, tolerance)
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
