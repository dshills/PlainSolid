package geom

import (
	"context"
	"math/big"
)

// OffAxisSphereFiniteCylinderUnion is the regular closed union of a sphere and
// a finite offset Z-cylinder. External tangencies are retained; this does not
// imply manifold boundary topology. Covered support patches are not reported.
// Constructors reject unresolved internal support tangencies rather than
// inferring local exposure from zero signs alone.
type OffAxisSphereFiniteCylinderUnion struct {
	model OffAxisSphereFiniteCylinderCommon
	mode  int // 0 ordinary union, 1 sphere contains cylinder, 2 cylinder contains sphere
}

func NewOffAxisSphereFiniteCylinderUnion(R, r float64, offset Vec2, lower, upper float64) (OffAxisSphereFiniteCylinderUnion, error) {
	m, e := NewOffAxisSphereFiniteCylinderCommon(R, r, offset, lower, upper)
	if e != nil {
		return OffAxisSphereFiniteCylinderUnion{}, e
	}
	out := OffAxisSphereFiniteCylinderUnion{model: m}
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	d2 := new(big.Rat).Add(sq(m.cx), sq(m.cy))
	negR := new(big.Rat).Neg(m.R)
	gap := new(big.Rat).Sub(m.r, m.R)
	if gap.Sign() >= 0 && d2.Cmp(sq(gap)) <= 0 && m.lower.Cmp(negR) <= 0 && m.upper.Cmp(m.R) >= 0 {
		out.mode = 2
		return out, nil
	}
	z2 := sq(m.lower)
	if z2.Cmp(sq(m.upper)) < 0 {
		z2 = sq(m.upper)
	}
	a := new(big.Rat).Sub(new(big.Rat).Sub(new(big.Rat).Sub(sq(m.R), z2), d2), sq(m.r))
	rhs := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(d2, sq(m.r)))
	if a.Sign() >= 0 && sq(a).Cmp(rhs) >= 0 {
		out.mode = 1
		return out, nil
	}
	// Disjoint interiors, including external radial or polar tangency, expose
	// both original boundaries and cannot create hidden support-only remnants.
	sum := new(big.Rat).Add(m.R, m.r)
	if d2.Cmp(sq(sum)) >= 0 || m.lower.Cmp(m.R) >= 0 || m.upper.Cmp(negR) <= 0 {
		return out, nil
	}
	if d2.Cmp(sq(gap)) == 0 || m.lower.Cmp(negR) == 0 || m.upper.Cmp(m.R) == 0 {
		return OffAxisSphereFiniteCylinderUnion{}, ErrDomain
	}
	for _, z := range []*big.Rat{m.lower, m.upper} {
		a := new(big.Rat).Sub(sq(m.R), sq(z))
		if a.Sign() <= 0 {
			continue
		}
		b := new(big.Rat).Sub(new(big.Rat).Sub(a, d2), sq(m.r))
		if sq(b).Cmp(rhs) == 0 {
			return OffAxisSphereFiniteCylinderUnion{}, ErrDomain
		}
	}
	return out, nil
}
func (s OffAxisSphereFiniteCylinderUnion) location(v []int) (BorePointLocation, BoreBoundaryMask) {
	sphere := v[0] <= 0
	cylinder := v[1] <= 0 && v[2] <= 0 && v[3] <= 0
	if s.mode == 1 {
		if v[0] > 0 {
			return BoreOutside, 0
		}
		if v[0] == 0 {
			return BoreBoundary, BoreSphere
		}
		return BoreInside, 0
	}
	if s.mode == 2 {
		sphere = false
	}
	if !sphere && !cylinder {
		return BoreOutside, 0
	}
	if sphere && v[0] < 0 || cylinder && v[1] < 0 && v[2] < 0 && v[3] < 0 {
		return BoreInside, 0
	}
	var mask BoreBoundaryMask
	if sphere && v[0] == 0 {
		mask |= BoreSphere
	}
	if cylinder {
		for i, m := range []BoreBoundaryMask{BoreCylinder, BoreLowerCap, BoreUpperCap} {
			if v[i+1] == 0 {
				mask |= m
			}
		}
	}
	return BoreBoundary, mask
}
func (s OffAxisSphereFiniteCylinderUnion) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	return s.model.classifyMaterial(p, s.location)
}

// IntersectLineBoundary returns every exposed isolated boundary event and
// continuous contact interval, with exact endpoint identities and support masks.
// A touching union can have several connected components meeting at a point.
func (s OffAxisSphereFiniteCylinderUnion) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	return s.model.intersectMaterial(ctx, origin, direction, minT, maxT, o, s.location)
}
func (s OffAxisSphereFiniteCylinderUnion) IntersectLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) ([]BoreLineHit, error) {
	r, e := s.IntersectLineBoundary(ctx, origin, direction, minT, maxT, o)
	if e != nil {
		return nil, e
	}
	if len(r.Intervals) > 0 {
		return nil, ErrConvergence
	}
	return r.Hits, nil
}
