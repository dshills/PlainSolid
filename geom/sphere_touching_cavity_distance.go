package geom

import (
	"context"
	"math/big"
)

// SphereTouchingCavityNearest preserves the complete two-full-support nearest
// report for a strictly smaller inner ball internally tangent to the outer.
// Contact is their exact rational singular boundary point, with both original
// owners. ContactNearest is proved by exact distance comparison. When true and
// both radial minima are singletons, CoincidentSingletons proves those two
// original feet are exactly Contact. Otherwise a whole-sphere center family
// may include Contact; no isolated azimuth is inferred for that family.
// When ContactNearest is false, Boundary still retains all global nearest
// owners, including possible tied but physically distinct feet.
// This is an analytic material query, NOT a manifold native shell certificate.
type SphereTouchingCavityNearest struct {
	Boundary                             SphereCavityNearest
	Contact                              [3]*big.Rat
	ContactOwners                        BoreBoundaryMask
	ContactNearest, CoincidentSingletons bool
	Operations                           int
}

// NearestTouchingCavityBoundaryRational accepts exactly internal tangency for
// A-minus-B with R>r. Both entire original spheres belong to the boundary of
// closure(interior(A\B)), including their singular contact: for small transverse
// radius rho, the inward offsets R-sqrt(R²-rho²) and r-sqrt(r²-rho²)
// bound a nonempty material interval because R>r. Strict cavities,
// overlap, empty and other operations are rejected by this explicit method.
// The existing strict cavity API and native tangent-shell gate are unchanged.
func (s SpherePairMaterial) NearestTouchingCavityBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions) (SphereTouchingCavityNearest, error) {
	return s.nearestTouchingCavityBoundaryRational(ctx, p, o, nil)
}
func (s SpherePairMaterial) nearestTouchingCavityBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions, shared *polynomialBudget) (out SphereTouchingCavityNearest, err error) {
	defer func() {
		if err != nil {
			out = SphereTouchingCavityNearest{}
		}
	}()
	if ctx == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return
	}
	if o.SqrtBits == 0 {
		o.SqrtBits = 64
	}
	if o.SqrtBits < 8 || o.SqrtBits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	budget := shared
	if budget == nil {
		budget = &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	}
	q, e := s.nearestFullSphereBoundaryRational(ctx, p, o, true, true, budget)
	if e != nil {
		return out, e
	}
	a := sphereOverlapArithmetic{ctx: ctx, b: budget}
	R, r := q.Candidates[0].Radius, q.Candidates[1].Radius
	scale := a.div(R, a.sub(R, r))
	for j := 0; j < 3; j++ {
		out.Contact[j] = a.add(s.center[0][j], a.mul(scale, a.sub(s.center[1][j], s.center[0][j])))
	}
	for i := 0; i < 2; i++ {
		v := a.distance(out.Contact, s.center[i])
		if a.err != nil {
			return out, a.err
		}
		if v.Cmp(s.radius2[i]) != 0 {
			return out, ErrDomain
		}
	}
	selected := 0
	if q.Owners == SpherePairSecond {
		selected = 1
	}
	w := q.Candidates[selected]
	contactDistance := a.distance(p, out.Contact)
	constant := a.add(w.Radicand, a.mul(w.Radius, w.Radius))
	coefficient := a.mul(big.NewRat(2, 1), w.Radius)
	difference := a.sub(contactDistance, constant)
	if a.err != nil {
		return out, a.err
	}
	sign, e := sphereCavityRadicalSign(difference, new(big.Rat).Neg(coefficient), w.Radicand, budget)
	if e != nil {
		return out, e
	}
	if sign < 0 {
		return out, ErrDomain
	}
	out.ContactNearest = sign == 0
	out.CoincidentSingletons = out.ContactNearest && !q.Candidates[0].WholeSphere && !q.Candidates[1].WholeSphere
	if out.ContactNearest && q.Owners != 3 {
		return out, ErrDomain
	}
	out.ContactOwners = 3
	if err = ctx.Err(); err != nil {
		return
	}
	out.Operations = budget.operations
	q.Operations = out.Operations
	out.Boundary = q
	return out, nil
}
