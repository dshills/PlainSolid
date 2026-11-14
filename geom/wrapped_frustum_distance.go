package geom

import (
	"context"
	"math/big"
)

// FrustumDistanceOptions requests a gap in squared model distance, not distance.
// MaxBisections defaults to256 (maximum4096); Surface covers embedding proof
// and cumulative exact arithmetic. SquaredTolerance must be positive.
type FrustumDistanceOptions struct {
	SquaredTolerance float64
	MaxBisections    int
	Surface          SurfacePatchOptions
}

// FrustumDistanceWitness is an exact boundary point in algebraic cylindrical
// form. Its model XY is Center+(Radius*AzimuthX,Radius*AzimuthY)/sqrt(AzimuthSquared).
// Axis queries use Azimuth=(1,0), squared1. Height is absolute model Z.
// Radius and Height are exact rationals. An exact cap projection may set
// Center to the projected XY and Radius to zero.
type FrustumDistanceWitness struct {
	CenterX, CenterY, Radius, Height, AzimuthX, AzimuthY, AzimuthSquared *big.Rat
	Part                                                                 CylinderBoundaryPart
	RotationallyNonunique                                                bool
}

// FrustumDistanceBounds encloses the true minimum squared boundary distance.
// Upper bounds the distance to Witness; Lower is global over the requested
// boundary pieces. Uniqueness is not certified: equal distances may tie.
type FrustumDistanceBounds struct {
	Lower, Upper    *big.Rat
	Witness         FrustumDistanceWitness
	Bisections      int
	ExactOperations int
}

// BoundPointWrappedCircularFrustumDistance uses the exact meridian trapezoid
// boundary for a certified circular-profile cylinder/frustum. It bounds the
// algebraic radial square root and solves rational segment distances. No mesh,
// sampled surface, or rounded witness is used. Failure returns no partial bound.
func BoundPointWrappedCircularFrustumDistance(ctx context.Context, w *WrappedSurfacePatch, query Vec3, o FrustumDistanceOptions) (FrustumDistanceBounds, error) {
	return BoundPointWrappedCircularFrustumPartsDistance(ctx, w, query, CylinderLateral|CylinderLowerCap|CylinderUpperCap, o)
}

// BoundPointWrappedCircularFrustumPartsDistance bounds distance to any chosen
// union of the original circular wall and the two original cap disks. A wall
// includes both rims. It keeps the same certified squared error contract as
// the complete closed-frustum query.
func BoundPointWrappedCircularFrustumPartsDistance(ctx context.Context, w *WrappedSurfacePatch, query Vec3,
	parts CylinderBoundaryPart, o FrustumDistanceOptions) (FrustumDistanceBounds, error) {
	if !query.IsFinite() {
		return FrustumDistanceBounds{}, ErrInvalid
	}
	return BoundPointWrappedCircularFrustumPartsDistanceRational(ctx, w, rationalLineVec3(query), parts, o)
}

// BoundPointWrappedCircularFrustumPartsDistanceRational preserves exact
// model query coordinates in the meridian and source-owned witness.
func BoundPointWrappedCircularFrustumPartsDistanceRational(ctx context.Context, w *WrappedSurfacePatch, query [3]*big.Rat,
	parts CylinderBoundaryPart, o FrustumDistanceOptions) (FrustumDistanceBounds, error) {
	empty := FrustumDistanceBounds{}
	if parts == 0 || parts & ^(CylinderLateral|CylinderLowerCap|CylinderUpperCap) != 0 {
		return empty, ErrInvalid
	}
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
	p, e := CertifyWrappedFrustumEmbedding(ctx, w, o.Surface)
	if e != nil {
		return empty, e
	}
	if !p.Circular {
		return empty, ErrConvergence
	}
	budget := &polynomialBudget{operations: p.ExactOperations, maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	x, y, z := new(big.Rat).Sub(query[0], p.CenterX), new(big.Rat).Sub(query[1], p.CenterY), query[2]
	q := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	lo, hi := frustumSqrtBounds(q)
	for _, v := range []*big.Rat{x, y, z, q, lo, hi} {
		if e := budget.check(v); e != nil {
			return empty, e
		}
	}
	type segment struct {
		a, b distanceMeridianPoint
		part CylinderBoundaryPart
	}
	segments := []segment{{distanceMeridianPoint{new(big.Rat), p.LowerZ}, distanceMeridianPoint{p.LowerRadius, p.LowerZ}, CylinderLowerCap}, {distanceMeridianPoint{p.LowerRadius, p.LowerZ}, distanceMeridianPoint{p.UpperRadius, p.UpperZ}, CylinderLateral}, {distanceMeridianPoint{new(big.Rat), p.UpperZ}, distanceMeridianPoint{p.UpperRadius, p.UpperZ}, CylinderUpperCap}}
	tol := new(big.Rat).SetFloat64(o.SquaredTolerance)
	for iteration := 0; ; iteration++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
		queryMid := distanceMeridianPoint{mid, z}
		left, right := distanceMeridianPoint{lo, z}, distanceMeridianPoint{hi, z}
		var lower, upper *big.Rat
		var witness distanceMeridianPoint
		var part CylinderBoundaryPart
		for _, s := range segments {
			if s.part&parts == 0 {
				continue
			}
			low := meridianSegmentDistance(left, right, s.a, s.b)
			candidate, _ := meridianProjection(queryMid, s.a, s.b)
			bound := meridianSquared(left, candidate)
			other := meridianSquared(right, candidate)
			if other.Cmp(bound) > 0 {
				bound = other
			}
			for _, v := range []*big.Rat{low, bound, candidate[0], candidate[1]} {
				if e := budget.check(v); e != nil {
					return empty, e
				}
			}
			if lower == nil || low.Cmp(lower) < 0 {
				lower = low
			}
			if upper == nil || bound.Cmp(upper) < 0 {
				upper = bound
				witness = candidate
				part = s.part
			}
		}
		if new(big.Rat).Sub(upper, lower).Cmp(tol) <= 0 {
			ax, ay, aq := new(big.Rat).Set(x), new(big.Rat).Set(y), new(big.Rat).Set(q)
			axis := q.Sign() == 0
			if axis {
				ax.SetInt64(1)
				ay.SetInt64(0)
				aq.SetInt64(1)
			}
			return FrustumDistanceBounds{Lower: lower, Upper: upper, Bisections: iteration, ExactOperations: budget.operations, Witness: FrustumDistanceWitness{new(big.Rat).Set(p.CenterX), new(big.Rat).Set(p.CenterY), new(big.Rat).Set(witness[0]), new(big.Rat).Set(witness[1]), ax, ay, aq, part, axis && witness[0].Sign() > 0}}, nil
		}
		if iteration >= o.MaxBisections {
			return empty, ErrConvergence
		}
		square := new(big.Rat).Mul(mid, mid)
		if e := budget.check(square); e != nil {
			return empty, e
		}
		switch square.Cmp(q) {
		case -1:
			lo.Set(mid)
		case 1:
			hi.Set(mid)
		default:
			lo.Set(mid)
			hi.Set(mid)
		}
	}
}

// BoundCircularRadialBoundaryDistance bounds distance to a horizontal disk
// or circle of the exact supplied radius and center. axisInterior is true
// for a filled disk, false for its circular rim. The witness is analytic
// and lies on the selected original horizontal part.
func BoundCircularRadialBoundaryDistance(ctx context.Context, center [2]*big.Rat, radius, height *big.Rat,
	query Vec3, axisInterior bool, part CylinderBoundaryPart, o FrustumDistanceOptions) (FrustumDistanceBounds, error) {
	if !query.IsFinite() {
		return FrustumDistanceBounds{}, ErrInvalid
	}
	return BoundCircularRadialBoundaryDistanceRational(ctx, center, radius, height, rationalLineVec3(query), axisInterior, part, o)
}

// BoundCircularRadialBoundaryDistanceRational preserves the exact XYZ query
// and clips its projection to the specified original disk or circular rim.
func BoundCircularRadialBoundaryDistanceRational(ctx context.Context, center [2]*big.Rat, radius, height *big.Rat,
	query [3]*big.Rat, axisInterior bool, part CylinderBoundaryPart, o FrustumDistanceOptions) (FrustumDistanceBounds, error) {
	var empty FrustumDistanceBounds
	if center[0] == nil || center[1] == nil || radius == nil || radius.Sign() <= 0 || height == nil ||
		(part != CylinderLowerCap && part != CylinderUpperCap) {
		return empty, ErrInvalid
	}
	opt, e := validateFrustumRationalDistance(ctx, query, o)
	if e != nil {
		return empty, e
	}
	if o.MaxBisections == 0 {
		o.MaxBisections = 256
	}
	if o.MaxBisections < 1 || o.MaxBisections > 4096 {
		return empty, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: opt.MaxOperations, maxBits: opt.MaxCoefficientBits}
	for _, q := range []*big.Rat{center[0], center[1], radius, height} {
		if e := b.check(q); e != nil {
			return empty, e
		}
	}
	x := new(big.Rat).Sub(query[0], center[0])
	y := new(big.Rat).Sub(query[1], center[1])
	q := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	r2 := new(big.Rat).Mul(radius, radius)
	dz := new(big.Rat).Sub(query[2], height)
	dz.Mul(dz, dz)
	for _, c := range []*big.Rat{q, r2, dz} {
		if e := b.check(c); e != nil {
			return empty, e
		}
	}
	inside := axisInterior && q.Cmp(r2) <= 0
	axis := q.Sign() == 0
	w := FrustumDistanceWitness{CenterX: new(big.Rat).Set(center[0]), CenterY: new(big.Rat).Set(center[1]),
		Height: new(big.Rat).Set(height), Radius: new(big.Rat).Set(radius), Part: part,
		AzimuthX: x, AzimuthY: y, AzimuthSquared: new(big.Rat).Set(q), RotationallyNonunique: axis && !inside}
	if axis {
		w.AzimuthX.SetInt64(1)
		w.AzimuthSquared.SetInt64(1)
	}
	if inside {
		// A centered exact witness expresses the represented query XY
		// without introducing its irrational radial coordinate.
		w.CenterX.Set(query[0])
		w.CenterY.Set(query[1])
		w.Radius.SetInt64(0)
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		return FrustumDistanceBounds{Lower: new(big.Rat).Set(dz), Upper: dz, Witness: w, ExactOperations: b.operations}, nil
	}
	lo, hi := frustumSqrtBounds(q)
	tol := new(big.Rat).SetFloat64(o.SquaredTolerance)
	for i := 0; i <= o.MaxBisections; i++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		left, right := new(big.Rat).Sub(lo, radius), new(big.Rat).Sub(hi, radius)
		cross := left.Sign() <= 0 && right.Sign() >= 0
		left.Mul(left, left)
		right.Mul(right, right)
		if left.Cmp(right) > 0 {
			left, right = right, left
		}
		lower := new(big.Rat).Set(dz)
		if !cross {
			lower.Add(lower, left)
		}
		upper := new(big.Rat).Add(dz, right)
		if e := b.check(lower); e != nil {
			return empty, e
		}
		if e := b.check(upper); e != nil {
			return empty, e
		}
		if new(big.Rat).Sub(upper, lower).Cmp(tol) <= 0 {
			return FrustumDistanceBounds{Lower: lower, Upper: upper, Witness: w, Bisections: i, ExactOperations: b.operations}, nil
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
		square := new(big.Rat).Mul(mid, mid)
		if e := b.check(square); e != nil {
			return empty, e
		}
		switch square.Cmp(q) {
		case -1:
			lo = mid
		case 1:
			hi = mid
		default:
			lo, hi = mid, mid
		}
	}
	return empty, ErrConvergence
}

func validateFrustumRationalDistance(ctx context.Context, query [3]*big.Rat, o FrustumDistanceOptions) (SurfacePatchOptions, error) {
	if ctx == nil || !finite(o.SquaredTolerance) || o.SquaredTolerance <= 0 || o.MaxBisections < 0 || o.MaxBisections > 4096 {
		return o.Surface, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return o.Surface, e
	}
	opt, e := patchOptions(o.Surface)
	if e != nil {
		return opt, e
	}
	for _, v := range query {
		if v == nil {
			return opt, ErrInvalid
		}
		if v.Num().BitLen() > opt.MaxCoefficientBits || v.Denom().BitLen() > opt.MaxCoefficientBits {
			return opt, ErrConvergence
		}
	}
	return opt, nil
}

func frustumSqrtBounds(q *big.Rat) (lo, hi *big.Rat) {
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(new(big.Int).Set(n), n).Cmp(q.Num()) == 0 && new(big.Int).Mul(new(big.Int).Set(d), d).Cmp(q.Denom()) == 0 {
		lo = new(big.Rat).SetFrac(n, d)
		return lo, new(big.Rat).Set(lo)
	}
	lo = new(big.Rat).SetInt(new(big.Int).Sqrt(new(big.Int).Quo(q.Num(), q.Denom())))
	return lo, new(big.Rat).Add(lo, big.NewRat(1, 1))
}

type distanceMeridianPoint [2]*big.Rat

func meridianSquared(a, b distanceMeridianPoint) *big.Rat {
	r := new(big.Rat)
	for i := 0; i < 2; i++ {
		d := new(big.Rat).Sub(a[i], b[i])
		r.Add(r, new(big.Rat).Mul(d, d))
	}
	return r
}
func meridianProjection(p, a, b distanceMeridianPoint) (distanceMeridianPoint, *big.Rat) {
	dx, dz := new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(b[1], a[1])
	norm := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dz, dz))
	t := new(big.Rat)
	if norm.Sign() > 0 {
		t.Add(new(big.Rat).Mul(new(big.Rat).Sub(p[0], a[0]), dx), new(big.Rat).Mul(new(big.Rat).Sub(p[1], a[1]), dz))
		t.Quo(t, norm)
		if t.Sign() < 0 {
			t.SetInt64(0)
		}
		if t.Cmp(big.NewRat(1, 1)) > 0 {
			t.SetInt64(1)
		}
	}
	q := distanceMeridianPoint{new(big.Rat).Add(a[0], new(big.Rat).Mul(t, dx)), new(big.Rat).Add(a[1], new(big.Rat).Mul(t, dz))}
	return q, meridianSquared(p, q)
}

// The first segment is horizontal. Test crossing exactly before minimizing
// the four endpoint-to-segment distances, which covers disjoint planar segments.
func meridianSegmentDistance(l, r, a, b distanceMeridianPoint) *big.Rat {
	dz := new(big.Rat).Sub(b[1], a[1])
	if dz.Sign() != 0 {
		t := new(big.Rat).Quo(new(big.Rat).Sub(l[1], a[1]), dz)
		if t.Sign() >= 0 && t.Cmp(big.NewRat(1, 1)) <= 0 {
			x := new(big.Rat).Add(a[0], new(big.Rat).Mul(t, new(big.Rat).Sub(b[0], a[0])))
			if x.Cmp(l[0]) >= 0 && x.Cmp(r[0]) <= 0 {
				return new(big.Rat)
			}
		}
	} else if l[1].Cmp(a[1]) == 0 {
		low, high := a[0], b[0]
		if low.Cmp(high) > 0 {
			low, high = high, low
		}
		if low.Cmp(r[0]) <= 0 && high.Cmp(l[0]) >= 0 {
			return new(big.Rat)
		}
	}
	_, best := meridianProjection(l, a, b)
	for _, s := range [][3]distanceMeridianPoint{{r, a, b}, {a, l, r}, {b, l, r}} {
		_, v := meridianProjection(s[0], s[1], s[2])
		if v.Cmp(best) < 0 {
			best = v
		}
	}
	return best
}
