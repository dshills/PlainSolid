package geom

import (
	"context"
	"math/big"
	"runtime"
)

// SphereAxisMinimumKind distinguishes exact minimizer sets, never representative
// points inferred to stand for a family.
type SphereAxisMinimumKind uint8

const (
	SphereAxisPole SphereAxisMinimumKind = iota + 1
	SphereAxisRetainedCap
	SphereAxisSharedCircle
)

// SphereAxisMinimumFamily describes the entire nearest set on one retained cap
// (slots 0/1), or on the common circle (slot 2). Nearest means it also attains the
// global minimum. SourceOwner identifies the original cap in slots 0/1; zero in
// slot 2. A circle family can occur in several slots without extra multiplicity.
//
// Pole uses Point only. Circle is the set |x-Center|²=RadiusSquared,
// x[Axis]=Center[Axis]. RetainedCap is |x-Center|²=RadiusSquared and
// CapSide*(x[Axis]-CapCoordinate)>=0, where CapSide is exactly +1 or -1.
// Owners describes every relative-interior point of the family. For a cap,
// RimOwners additionally describes its boundary circle (both original owners).
// No normal or isolated azimuth is assigned to a non-singleton family.
type SphereAxisMinimumFamily struct {
	Kind                           SphereAxisMinimumKind
	SourceOwner, Owners, RimOwners BoreBoundaryMask
	Point, Center                  [3]*big.Rat
	RadiusSquared, CapCoordinate   *big.Rat
	Axis, CapSide                  int
	SquaredDistance                *big.Rat
	Nearest                        bool
}

// SphereOverlapAxisNearest gives complete exact global minimizer families in
// stable original-cap A, original-cap B, common-circle order. Owners includes
// every source face attaining the minimum, including boundaries of a cap family.
type SphereOverlapAxisNearest struct {
	SquaredDistance     *big.Rat
	Owners              BoreBoundaryMask
	Families            [3]SphereAxisMinimumFamily
	MaterialNormalSigns [2]int
	Operations          int
}

// NearestOverlapAxisBoundaryRational accepts transverse spheres with center
// displacement along one coordinate and a rational query exactly on that axis.
// The circle radius need not be rational: its exact squared radius is retained.
// Each cap minimizes a linear axial functional, so the complete minimum is its
// admitted pole, its entire rim, or (at its center) the entire retained cap.
// Exact rational comparisons preserve ties across those families. Tangency,
// containment, disjoint sources and off-axis queries are rejected atomically.
func (s SpherePairMaterial) NearestOverlapAxisBoundaryRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (out SphereOverlapAxisNearest, err error) {
	defer func() {
		if err != nil {
			out = SphereOverlapAxisNearest{}
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
		} else if p[j].Cmp(s.center[0][j]) != 0 {
			return out, ErrDomain
		}
	}
	if axis < 0 {
		return out, ErrDomain
	}
	for _, v := range s.radius2 {
		a.check(v)
	}
	d := a.sub(s.center[1][axis], s.center[0][axis])
	t := a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), a.mul(d, d)), a.mul(big.NewRat(2, 1), d))
	h2 := a.sub(s.radius2[0], a.mul(t, t))
	circleCenter := [3]*big.Rat{}
	for j := 0; j < 3; j++ {
		circleCenter[j] = new(big.Rat).Set(s.center[0][j])
		if j == axis {
			circleCenter[j] = a.add(circleCenter[j], t)
		}
	}
	if a.err != nil {
		return out, a.err
	}
	if h2.Sign() <= 0 {
		return out, ErrDomain
	}
	for i := 0; i < 2; i++ {
		v := a.add(a.distance(circleCenter, s.center[i]), h2)
		if a.err != nil {
			return out, a.err
		}
		if v.Cmp(s.radius2[i]) != 0 {
			return out, ErrDomain
		}
	}
	axial := a.sub(p[axis], circleCenter[axis])
	rimDistance := a.add(a.mul(axial, axial), h2)
	circle := func(source BoreBoundaryMask) SphereAxisMinimumFamily {
		f := SphereAxisMinimumFamily{Kind: SphereAxisSharedCircle, SourceOwner: source, Owners: 3, Axis: axis, RadiusSquared: new(big.Rat).Set(h2), SquaredDistance: new(big.Rat).Set(rimDistance)}
		for j := 0; j < 3; j++ {
			f.Center[j] = new(big.Rat).Set(circleCenter[j])
		}
		return f
	}
	out.MaterialNormalSigns = [2]int{1, 1}
	if s.op == SphereSetDifference {
		out.MaterialNormalSigns[1] = -1
	}
	for i := 0; i < 2; i++ {
		R := a.sqrt(s.radius2[i])
		delta := a.sub(p[axis], s.center[i][axis])
		owner := BoreBoundaryMask(1 << uint(i))
		direction := d.Sign()
		if i == 1 {
			direction = -direction
		}
		outside := s.op == SphereSetUnion || s.op == SphereSetDifference && i == 0
		side := direction
		if outside {
			side = -side
		}
		if a.err != nil {
			return out, a.err
		}
		if delta.Sign() == 0 {
			f := SphereAxisMinimumFamily{Kind: SphereAxisRetainedCap, SourceOwner: owner, Owners: owner, RimOwners: 3, RadiusSquared: new(big.Rat).Set(s.radius2[i]), Axis: axis, CapCoordinate: new(big.Rat).Set(circleCenter[axis]), CapSide: side, SquaredDistance: new(big.Rat).Set(s.radius2[i])}
			for j := 0; j < 3; j++ {
				f.Center[j] = new(big.Rat).Set(s.center[i][j])
			}
			out.Families[i] = f
		} else {
			f := SphereAxisMinimumFamily{Kind: SphereAxisPole, SourceOwner: owner, Owners: owner, Axis: axis}
			for j := 0; j < 3; j++ {
				f.Point[j] = new(big.Rat).Set(s.center[i][j])
				if j == axis {
					f.Point[j] = a.add(f.Point[j], a.mul(big.NewRat(int64(delta.Sign()), 1), R))
				}
			}
			residual := a.sub(a.distance(f.Point, s.center[1-i]), s.radius2[1-i])
			if a.err != nil {
				return out, a.err
			}
			retained := residual.Sign() >= 0
			if !outside {
				retained = residual.Sign() <= 0
			}
			if retained {
				f.SquaredDistance = a.distance(p, f.Point)
				if residual.Sign() == 0 {
					f.Owners = 3
				}
				out.Families[i] = f
			} else {
				out.Families[i] = circle(owner)
			}
		}
	}
	out.Families[2] = circle(0)
	runtime.KeepAlive(&circleCenter) // golang/go#82026: keeps circleCenter in the stack map. Remove once the Go fix is in use.
	if a.err != nil {
		return out, a.err
	}
	out.SquaredDistance = new(big.Rat).Set(rimDistance)
	for _, f := range out.Families {
		if f.SquaredDistance.Cmp(out.SquaredDistance) < 0 {
			out.SquaredDistance.Set(f.SquaredDistance)
		}
	}
	for i := range out.Families {
		f := &out.Families[i]
		f.Nearest = f.SquaredDistance.Cmp(out.SquaredDistance) == 0
		if f.Nearest {
			out.Owners |= f.Owners | f.RimOwners
		}
	}
	if err = ctx.Err(); err != nil {
		return
	}
	out.Operations = a.b.operations
	return out, nil
}
