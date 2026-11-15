package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ErodeSphericalLens erodes an originally transverse intersection of two
// balls by a ball of the requested nonnegative depth. Both support radii
// shrink by the exact depth. External tangency and complete collapse
// regularize to empty material. Surviving radii must be exactly representable.
func (s SpherePairSet) ErodeSphericalLens(ctx context.Context, depth, tolerance float64) (Solid, SpherePairOffsetHistory, error) {
	var empty Solid
	noHistory := emptySpherePairOffsetHistory()
	if ctx == nil || !s.valid || !finite(depth) || depth < 0 || !finite(tolerance) || tolerance <= 0 {
		return empty, noHistory, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, noHistory, err
	}
	balls, source, err := s.sphereOffsetOwners(Intersection, false)
	if err != nil {
		return empty, noHistory, err
	}
	transverse, err := s.query.HasTransverseBoundaryIntersection()
	if err != nil {
		return empty, noHistory, err
	}
	if !transverse {
		return empty, noHistory, geom.ErrDomain
	}
	d := new(big.Rat).SetFloat64(depth)
	var history = SpherePairOffsetHistory{SourceFaces: source, ResultFaces: [2]int{-1, -1}}
	for i, ball := range balls {
		history.Radii[i] = new(big.Rat).Sub(new(big.Rat).SetFloat64(ball.Radius), d)
	}
	if d.Sign() == 0 {
		history.ResultFaces = source
		return s.solid, history, nil
	}
	if history.Radii[0].Sign() <= 0 || history.Radii[1].Sign() <= 0 {
		return empty, history, nil
	}
	// Erosion commutes with intersection. The radius difference is fixed,
	// so the two eroded balls can remain transverse or lose volume entirely.
	sum := new(big.Rat).Add(history.Radii[0], history.Radii[1])
	if sphereOffsetCenterDistanceSquared(balls[0].Center, balls[1].Center).Cmp(new(big.Rat).Mul(sum, sum)) >= 0 {
		return empty, history, nil
	}
	for i, r := range history.Radii {
		balls[i].Radius, err = geom.RepresentedExactScalar(r)
		if err != nil {
			return empty, noHistory, err
		}
	}
	result, err := NewSpherePairSet(ctx, balls[0], balls[1], Intersection, tolerance)
	if err != nil {
		return empty, noHistory, err
	}
	for i, owners := range result.faces {
		if len(owners) != 1 {
			return empty, noHistory, geom.ErrDomain
		}
		history.ResultFaces[i] = owners[0]
	}
	return result.solid, history, nil
}
