package geom

import (
	"context"
	"math/big"
)

// SphereOverlapDistanceCandidate retains a full-sphere radial projection or
// the isolated nearest shared-circle point. Retained=false means a full sphere
// foot is buried by this Boolean; it is never used as an upper witness.
// Owners includes both source spheres at a shared-circle foot.
type SphereOverlapDistanceCandidate struct {
	Point             [3]*big.Rat
	SquaredDistance   *big.Rat
	Owners            BoreBoundaryMask
	Retained, Nearest bool
}

// SphereOverlapNearest is a complete exact nearest report within the documented
// rational-foot subset. Candidates 0/1 are the original radial feet; candidate
// 2 is the circle foot. Equal points may intentionally occur in multiple slots;
// the stable source order and Owners describe incidence, not multiplicity.
// MaterialNormalSigns are +1,+1 except A-minus-B, where the second is -1.
type SphereOverlapNearest struct {
	SquaredDistance     *big.Rat
	Owners              BoreBoundaryMask
	Candidates          [3]SphereOverlapDistanceCandidate
	MaterialNormalSigns [2]int
	Operations          int
}

// NearestOverlapBoundaryRational accepts transverse spheres whose centers differ
// in exactly one coordinate, and queries with rational positive radial distances
// to each center and to their common axis. The intersection-circle radius must
// also be rational. All are proved exactly; unsupported radicals, axis/center
// families, tangencies, containment and disjoint sources return ErrDomain.
//
// For each retained spherical cap, minimizing squared distance maximizes a
// linear functional on the sphere: its minimum is either the full radial foot
// (if retained) or the cap's circle boundary. The nearest circle foot is unique
// off-axis. Exact other-sphere signs exclude buried feet for Union, Intersection
// and Difference. Thus the three candidates exhaust minima in this subset.
// No native seam approximation is used. Failures are atomic with one budget.
func (s SpherePairMaterial) NearestOverlapBoundaryRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (out SphereOverlapNearest, err error) {
	defer func() {
		if err != nil {
			out = SphereOverlapNearest{}
		}
	}()
	if ctx == nil || s.radius2[0] == nil || s.radius2[1] == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return
	}
	if !s.transverse {
		return out, ErrDomain
	}
	limits, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	a := sphereOverlapArithmetic{ctx: ctx, b: &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}}
	for _, v := range p {
		if v == nil {
			return out, ErrInvalid
		}
		a.check(v)
	}
	axis := -1
	for j := 0; j < 3; j++ {
		for i := 0; i < 2; i++ {
			a.check(s.center[i][j])
		}
		if s.center[0][j].Cmp(s.center[1][j]) != 0 {
			if axis >= 0 {
				return out, ErrDomain
			}
			axis = j
		}
	}
	if axis < 0 {
		return out, ErrDomain
	}
	for _, v := range s.radius2 {
		a.check(v)
	}
	d := a.sub(s.center[1][axis], s.center[0][axis])
	// Signed axial distance from first center to the radical plane.
	t := a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), a.mul(d, d)), a.mul(big.NewRat(2, 1), d))
	h := a.sqrt(a.sub(s.radius2[0], a.mul(t, t)))
	center := [3]*big.Rat{}
	radial2 := new(big.Rat)
	for j := 0; j < 3; j++ {
		center[j] = new(big.Rat).Set(s.center[0][j])
		if j == axis {
			center[j] = a.add(center[j], t)
		} else {
			v := a.sub(p[j], center[j])
			radial2 = a.add(radial2, a.mul(v, v))
		}
	}
	rho := a.sqrt(radial2)
	out.MaterialNormalSigns = [2]int{1, 1}
	if s.op == SphereSetDifference {
		out.MaterialNormalSigns[1] = -1
	}
	for i := 0; i < 2; i++ {
		norm := a.sqrt(a.distance(p, s.center[i]))
		radius := a.sqrt(s.radius2[i])
		w := SphereOverlapDistanceCandidate{}
		for j := 0; j < 3; j++ {
			w.Point[j] = a.add(s.center[i][j], a.div(a.mul(radius, a.sub(p[j], s.center[i][j])), norm))
		}
		w.SquaredDistance = a.distance(p, w.Point)
		other := a.sub(a.distance(w.Point, s.center[1-i]), s.radius2[1-i])
		if a.err != nil {
			return out, a.err
		}
		w.Retained = other.Sign() >= 0
		if s.op == SphereSetIntersection || s.op == SphereSetDifference && i == 1 {
			w.Retained = other.Sign() <= 0
		}
		w.Owners = 1 << uint(i)
		if other.Sign() == 0 {
			w.Owners = SpherePairFirst | SpherePairSecond
		}
		out.Candidates[i] = w
	}
	rim := SphereOverlapDistanceCandidate{Retained: true, Owners: SpherePairFirst | SpherePairSecond}
	for j := 0; j < 3; j++ {
		rim.Point[j] = new(big.Rat).Set(center[j])
		if j != axis {
			rim.Point[j] = a.add(center[j], a.div(a.mul(h, a.sub(p[j], center[j])), rho))
		}
	}
	rim.SquaredDistance = a.distance(p, rim.Point)
	out.Candidates[2] = rim
	if a.err != nil {
		return out, a.err
	}
	// Independently authenticate the common-circle point against both originals.
	for i := 0; i < 2; i++ {
		q := a.distance(rim.Point, s.center[i])
		if a.err != nil {
			return out, a.err
		}
		if q.Cmp(s.radius2[i]) != 0 {
			return out, ErrDomain
		}
	}
	out.SquaredDistance = new(big.Rat).Set(rim.SquaredDistance)
	for _, w := range out.Candidates {
		if w.Retained && w.SquaredDistance.Cmp(out.SquaredDistance) < 0 {
			out.SquaredDistance.Set(w.SquaredDistance)
		}
	}
	for i := range out.Candidates {
		w := &out.Candidates[i]
		w.Nearest = w.Retained && w.SquaredDistance.Cmp(out.SquaredDistance) == 0
		if w.Nearest {
			out.Owners |= w.Owners
		}
	}
	if err = ctx.Err(); err != nil {
		return
	}
	out.Operations = a.b.operations
	return out, nil
}

type sphereOverlapArithmetic struct {
	ctx context.Context
	b   *polynomialBudget
	err error
}

func (a *sphereOverlapArithmetic) check(v *big.Rat) {
	if a.err == nil {
		a.err = a.ctx.Err()
	}
	if a.err == nil {
		a.err = a.b.check(v)
	}
}
func (a *sphereOverlapArithmetic) add(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	v := new(big.Rat).Add(x, y)
	a.check(v)
	return v
}
func (a *sphereOverlapArithmetic) sub(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	v := new(big.Rat).Sub(x, y)
	a.check(v)
	return v
}
func (a *sphereOverlapArithmetic) mul(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	v := new(big.Rat).Mul(x, y)
	a.check(v)
	return v
}
func (a *sphereOverlapArithmetic) div(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	if y.Sign() == 0 {
		a.err = ErrDomain
		return new(big.Rat)
	}
	v := new(big.Rat).Quo(x, y)
	a.check(v)
	return v
}
func (a *sphereOverlapArithmetic) sqrt(x *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	if x.Sign() <= 0 {
		a.err = ErrDomain
		return new(big.Rat)
	}
	lo, hi, e := rationalSqrtBounds(x, 8, a.b)
	if e != nil {
		a.err = e
		return new(big.Rat)
	}
	if lo.Cmp(hi) != 0 {
		a.err = ErrDomain
	}
	return lo
}
func (a *sphereOverlapArithmetic) distance(x, y [3]*big.Rat) *big.Rat {
	v := new(big.Rat)
	for j := 0; j < 3; j++ {
		d := a.sub(x[j], y[j])
		v = a.add(v, a.mul(d, d))
	}
	return v
}
