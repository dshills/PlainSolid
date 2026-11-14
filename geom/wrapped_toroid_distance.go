package geom

import (
	"context"
	"math/big"
)

// ToroidDistanceBounds encloses minimum squared distance to the entire
// circular toroid boundary. The witness is algebraic: first rotate into the
// meridian plane using Azimuth/sqrt(AzimuthSquared); in that plane the boundary
// point is (R+r*A/sqrt(MeridianSquared), Cz+r*B/sqrt(MeridianSquared)), where
// A=sqrt(AzimuthSquared)−R and B=query.Z−Cz. For centerline queries a fixed
// exterior-equator witness is selected. Axis queries choose azimuth +X.
// Nonunique flags identify exact rotational/meridian families; other possible
// uniqueness properties are not asserted.
type ToroidDistanceBounds struct {
	Lower, Upper                                     *big.Rat
	Center                                           [3]*big.Rat
	MajorRadius, MinorRadius                         *big.Rat
	AzimuthX, AzimuthY, AzimuthSquared, HeightOffset *big.Rat
	AxisNonunique, CenterlineNonunique               bool
	Bisections                                       int
}

// BoundPointWrappedCircularToroidDistance proves squared distance bounds for
// certified circular U/V profiles with equal radial and axial minor radius.
// It brackets rho=sqrt(x²+y²), then d=sqrt((rho−R)²+z²); the exact distance is
// |d−r|. Returned witness data describes the exact radial projection rather
// than a sampled/rounded point. Options and failure atomicity match the
// frustum distance query. Elliptical meridians fail explicitly.
func BoundPointWrappedCircularToroidDistance(ctx context.Context, w *WrappedSurfacePatch, query Vec3, o FrustumDistanceOptions) (ToroidDistanceBounds, error) {
	if !query.IsFinite() {
		return ToroidDistanceBounds{}, ErrInvalid
	}
	return BoundPointWrappedCircularToroidDistanceRational(ctx, w, rationalLineVec3(query), o)
}

// BoundPointWrappedCircularToroidDistanceRational does not round the input
// model point and retains its exact component values in the local witness.
func BoundPointWrappedCircularToroidDistanceRational(ctx context.Context, w *WrappedSurfacePatch, query [3]*big.Rat, o FrustumDistanceOptions) (ToroidDistanceBounds, error) {
	var empty ToroidDistanceBounds
	opts, e := validateFrustumRationalDistance(ctx, query, o)
	if e != nil {
		return empty, e
	}
	if o.MaxBisections == 0 {
		o.MaxBisections = 256
	}
	if o.MaxBisections < 1 || o.MaxBisections > 4096 {
		return empty, ErrInvalid
	}
	p, e := CertifyWrappedToroidEmbedding(ctx, w, o.Surface)
	if e != nil {
		return empty, e
	}
	if p.RadialMinorRadius.Cmp(p.AxialMinorRadius) != 0 {
		return empty, ErrConvergence
	}
	budget := &polynomialBudget{operations: p.ExactOperations, maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, stride := range []int{1, 9} {
		for j := 0; j < 8; j += 2 {
			a, b, c := w.patch.net[j*stride][3], w.patch.net[(j+1)*stride][3], w.patch.net[(j+2)*stride][3]
			left := new(big.Rat).Mul(a, c)
			right := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(b, b))
			for _, v := range []*big.Rat{left, right} {
				if e := budget.check(v); e != nil {
					return empty, e
				}
			}
			if left.Cmp(right) != 0 {
				return empty, ErrConvergence
			}
		}
	}
	x, y, z := new(big.Rat).Sub(query[0], p.Center[0]), new(big.Rat).Sub(query[1], p.Center[1]), new(big.Rat).Sub(query[2], p.Center[2])
	q := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	z2 := new(big.Rat).Mul(z, z)
	for _, v := range []*big.Rat{x, y, z, q, z2} {
		if e := budget.check(v); e != nil {
			return empty, e
		}
	}
	rhoLo, rhoHi := frustumSqrtBounds(q)
	tol := new(big.Rat).SetFloat64(o.SquaredTolerance)
	square := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	// Maintain enclosing sqrt intervals for the smallest and largest meridian
	// squared distances. Their targets tighten as the radial bracket contracts.
	var distLo, distHi *big.Rat
	for iteration := 0; ; iteration++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		a, b := new(big.Rat).Sub(rhoLo, p.MajorRadius), new(big.Rat).Sub(rhoHi, p.MajorRadius)
		sa, sb := square(a), square(b)
		min2, max2 := sa, sb
		if sa.Cmp(sb) > 0 {
			min2, max2 = sb, sa
		}
		if a.Sign() <= 0 && b.Sign() >= 0 {
			min2 = new(big.Rat)
		}
		min2 = new(big.Rat).Add(min2, z2)
		max2 = new(big.Rat).Add(max2, z2)
		if distLo == nil {
			distLo, _ = frustumSqrtBounds(min2)
			_, distHi = frustumSqrtBounds(max2)
		}
		// Bisection endpoints are certified against the current targets, so each
		// update remains valid even after rho's interval has contracted.
		if square(distLo).Cmp(min2) > 0 {
			return empty, ErrConvergence
		}
		if square(distHi).Cmp(max2) < 0 {
			return empty, ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(distLo, distHi), big.NewRat(2, 1))
		m2 := square(mid)
		if m2.Cmp(min2) <= 0 {
			distLo.Set(mid)
		} else if m2.Cmp(max2) >= 0 {
			distHi.Set(mid)
		}
		da, db := new(big.Rat).Sub(distLo, p.RadialMinorRadius), new(big.Rat).Sub(distHi, p.RadialMinorRadius)
		lower, upper := square(da), square(db)
		if lower.Cmp(upper) > 0 {
			lower, upper = upper, lower
		}
		if da.Sign() <= 0 && db.Sign() >= 0 {
			lower = new(big.Rat)
		}
		for _, v := range []*big.Rat{q, z2, rhoLo, rhoHi, min2, max2, distLo, distHi, lower, upper} {
			if e := budget.check(v); e != nil {
				return empty, e
			}
		}
		if new(big.Rat).Sub(upper, lower).Cmp(tol) <= 0 {
			centerline := z.Sign() == 0 && q.Cmp(square(p.MajorRadius)) == 0
			r := ToroidDistanceBounds{Lower: lower, Upper: upper, MajorRadius: new(big.Rat).Set(p.MajorRadius), MinorRadius: new(big.Rat).Set(p.RadialMinorRadius), AzimuthX: new(big.Rat).Set(x), AzimuthY: new(big.Rat).Set(y), AzimuthSquared: new(big.Rat).Set(q), HeightOffset: new(big.Rat).Set(z), AxisNonunique: q.Sign() == 0, CenterlineNonunique: centerline, Bisections: iteration}
			for i := range r.Center {
				r.Center[i] = new(big.Rat).Set(p.Center[i])
			}
			return r, nil
		}
		if iteration >= o.MaxBisections {
			return empty, ErrConvergence
		}
		mid = new(big.Rat).Quo(new(big.Rat).Add(rhoLo, rhoHi), big.NewRat(2, 1))
		switch square(mid).Cmp(q) {
		case -1:
			rhoLo.Set(mid)
		case 1:
			rhoHi.Set(mid)
		default:
			rhoLo.Set(mid)
			rhoHi.Set(mid)
		}
	}
}
