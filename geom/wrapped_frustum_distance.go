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
// Radius and Height are exact rationals, including cap-interior witnesses.
type FrustumDistanceWitness struct {
	CenterX, CenterY, Radius, Height, AzimuthX, AzimuthY, AzimuthSquared *big.Rat
	Part                                                                 CylinderBoundaryPart
	RotationallyNonunique                                                bool
}

// FrustumDistanceBounds encloses the true minimum squared boundary distance.
// Upper bounds the distance to Witness; Lower is global over all three boundary
// pieces. Uniqueness is not certified: equal-distance caps or sides may tie.
type FrustumDistanceBounds struct {
	Lower, Upper *big.Rat
	Witness      FrustumDistanceWitness
	Bisections   int
}

// BoundPointWrappedCircularFrustumDistance uses the exact meridian trapezoid
// boundary for a certified circular-profile cylinder/frustum. It bounds the
// algebraic radial square root and solves rational segment distances. No mesh,
// sampled surface, or rounded witness is used. Failure returns no partial bound.
func BoundPointWrappedCircularFrustumDistance(ctx context.Context, w *WrappedSurfacePatch, query Vec3, o FrustumDistanceOptions) (FrustumDistanceBounds, error) {
	empty := FrustumDistanceBounds{}
	if !query.IsFinite() || !finite(o.SquaredTolerance) || o.SquaredTolerance <= 0 {
		return empty, ErrInvalid
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
	opts, _ := patchOptions(o.Surface)
	budget := &polynomialBudget{operations: p.ExactOperations, maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	x, y, z := new(big.Rat).Sub(new(big.Rat).SetFloat64(query.X), p.CenterX), new(big.Rat).Sub(new(big.Rat).SetFloat64(query.Y), p.CenterY), new(big.Rat).SetFloat64(query.Z)
	q := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	sqrtN, sqrtD := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	lo, hi := new(big.Rat), new(big.Rat)
	if new(big.Int).Mul(new(big.Int).Set(sqrtN), sqrtN).Cmp(q.Num()) == 0 && new(big.Int).Mul(new(big.Int).Set(sqrtD), sqrtD).Cmp(q.Denom()) == 0 {
		lo.SetFrac(sqrtN, sqrtD)
		hi.Set(lo)
	} else {
		floor := new(big.Int).Sqrt(new(big.Int).Quo(q.Num(), q.Denom()))
		lo.SetInt(floor)
		hi.Add(lo, big.NewRat(1, 1))
	}
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
			return FrustumDistanceBounds{Lower: lower, Upper: upper, Bisections: iteration, Witness: FrustumDistanceWitness{new(big.Rat).Set(p.CenterX), new(big.Rat).Set(p.CenterY), new(big.Rat).Set(witness[0]), new(big.Rat).Set(witness[1]), ax, ay, aq, part, axis && witness[0].Sign() > 0}}, nil
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
