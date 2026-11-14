package geom

import (
	"context"
	"math/big"
)

// ObliqueCylinderTorusCavityDistance reports complete original support bounds
// in world distance units. Cylinder witnesses use world XY azimuth/Z height.
// Torus witnesses instead use TorusQuery's local azimuth, then the independently
// owned proper TorusFrame maps them to world coordinates. WholeCircle retains
// an entire attained circle; witnesses need not be exact stationary minimizers.
// Only exact collapsed values certify ExactOwners; interval overlaps do not.
type ObliqueCylinderTorusCavityDistance struct {
	TorusCylinderDistance
	ExactOwners BoreBoundaryMask
	TorusFrame  [3][4]*big.Rat
	TorusQuery  [3]*big.Rat
}

// BoundBoundaryDistance evaluates the original inward torus in its own proper
// chart, and the original wall/caps in the stock chart. One shared arithmetic
// and cell ledger includes the exact inverse; no coaxial geometry is assumed.
// Atomic unresolved/budget behavior and strict owner separation are inherited
// from the full-boundary cavity solver.
func (s ObliqueCylinderTorusCavityMaterial) BoundBoundaryDistance(ctx context.Context, p [3]*big.Rat, o TorusCylinderDistanceOptions) (ObliqueCylinderTorusCavityDistance, error) {
	var out ObliqueCylinderTorusCavityDistance
	if s.source.R == nil {
		return out, ErrInvalid
	}
	adapter := EccentricCylinderTorusCavityMaterial{s.source, [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	var local [3]*big.Rat
	base, e := adapter.boundBoundaryDistance(ctx, p, o, &s.pose, &local)
	if e != nil {
		return out, e
	}
	frame, e := s.TorusFrame()
	if e != nil {
		return out, e
	}
	out.TorusCylinderDistance = base.TorusCylinderDistance
	out.ExactOwners = base.ExactOwners
	out.TorusFrame = frame
	out.TorusQuery = local
	return out, nil
}
