package geom

import (
	"context"
	"math/big"
)

// OffAxisSphereFiniteCylinderDifference queries the regularized closure of
// sphere interior minus the closed finite cylinder. Private exact predicates
// retain arbitrary cap placements, including partial cap disks. Tangencies
// that cannot be reduced to a proven unchanged sphere or empty result are
// conservatively rejected by the constructor; lower-dimensional remnants are
// never presented as material. Support masks identify inward-facing tool faces.
type OffAxisSphereFiniteCylinderDifference struct {
	model OffAxisSphereFiniteCylinderCommon
	mode  int // 0 transverse, 1 unchanged sphere, 2 empty
}

func NewOffAxisSphereFiniteCylinderDifference(R, r float64, offset Vec2, lower, upper float64) (OffAxisSphereFiniteCylinderDifference, error) {
	m, e := NewOffAxisSphereFiniteCylinderCommon(R, r, offset, lower, upper)
	if e != nil {
		return OffAxisSphereFiniteCylinderDifference{}, e
	}
	out := OffAxisSphereFiniteCylinderDifference{model: m}
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	d2 := new(big.Rat).Add(sq(m.cx), sq(m.cy))
	sum := new(big.Rat).Add(m.R, m.r)
	negR := new(big.Rat).Neg(m.R)
	// A tool whose open volume misses the sphere cannot create a tool boundary,
	// including external radial tangency and pole-only cap contact.
	if d2.Cmp(sq(sum)) >= 0 || m.lower.Cmp(m.R) >= 0 || m.upper.Cmp(negR) <= 0 {
		out.mode = 1
		return out, nil
	}
	gap := new(big.Rat).Sub(m.r, m.R)
	if gap.Sign() >= 0 && d2.Cmp(sq(gap)) <= 0 && m.lower.Cmp(negR) <= 0 && m.upper.Cmp(m.R) >= 0 {
		out.mode = 2
		return out, nil
	}
	// Internal cylinder/sphere tangency at the equator, or cap pole tangency,
	// needs local regularization beyond the transverse support-sign proof.
	if d2.Cmp(sq(gap)) == 0 || m.lower.Cmp(negR) == 0 || m.upper.Cmp(m.R) == 0 {
		return OffAxisSphereFiniteCylinderDifference{}, ErrDomain
	}
	// Reject a tangent/coincident cap-circle and cylinder rim. This includes
	// coaxial coincident rims; squared elimination is conservative and exact.
	for _, z := range []*big.Rat{m.lower, m.upper} {
		a := new(big.Rat).Sub(sq(m.R), sq(z))
		if a.Sign() <= 0 {
			continue
		}
		b := new(big.Rat).Sub(new(big.Rat).Sub(a, d2), sq(m.r))
		rhs := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(d2, sq(m.r)))
		if sq(b).Cmp(rhs) == 0 {
			return OffAxisSphereFiniteCylinderDifference{}, ErrDomain
		}
	}
	return out, nil
}
func (s OffAxisSphereFiniteCylinderDifference) location(v []int) (BorePointLocation, BoreBoundaryMask) {
	if s.mode == 2 || v[0] > 0 {
		return BoreOutside, 0
	}
	if s.mode == 1 {
		if v[0] == 0 {
			return BoreBoundary, BoreSphere
		}
		return BoreInside, 0
	}
	// Outside any tool constraint leaves unmodified sphere material.
	if v[1] > 0 || v[2] > 0 || v[3] > 0 {
		if v[0] == 0 {
			return BoreBoundary, BoreSphere
		}
		return BoreInside, 0
	}
	// Strict tool interior is absent even on the original sphere boundary.
	if v[1] < 0 && v[2] < 0 && v[3] < 0 {
		return BoreOutside, 0
	}
	var mask BoreBoundaryMask
	for i, m := range []BoreBoundaryMask{BoreSphere, BoreCylinder, BoreLowerCap, BoreUpperCap} {
		if v[i] == 0 {
			mask |= m
		}
	}
	return BoreBoundary, mask
}
func (s OffAxisSphereFiniteCylinderDifference) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	return s.model.classifyMaterial(p, s.location)
}

// IntersectLineBoundary returns isolated hits and closed boundary contact
// intervals with exact algebraic endpoints. An unchanged sphere has no tool
// masks even where a tool support touches it. Empty regularized results have
// no hits. Arithmetic and cancellation follow the finite-common query options.
func (s OffAxisSphereFiniteCylinderDifference) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	return s.model.intersectMaterial(ctx, origin, direction, minT, maxT, o, s.location)
}
func (s OffAxisSphereFiniteCylinderDifference) IntersectLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) ([]BoreLineHit, error) {
	r, e := s.IntersectLineBoundary(ctx, origin, direction, minT, maxT, o)
	if e != nil {
		return nil, e
	}
	if len(r.Intervals) > 0 {
		return nil, ErrConvergence
	}
	return r.Hits, nil
}
