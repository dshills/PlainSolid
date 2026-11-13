package geom

import (
	"context"
	"math/big"
)

// RoundedCylinderDistanceWitness represents an exact meridian point.
// Radial and Height are absolute meridian coordinates when ArcNormSquared
// is nil. Otherwise they are the rolling circle center coordinates and the
// point is their sum with ArcRadius*(ArcRadial, ArcHeight)/sqrt(ArcNormSquared).
// Model XY is obtained by multiplying the resulting radial coordinate by
// (AzimuthX, AzimuthY)/sqrt(AzimuthSquared) and adding Center.
type RoundedCylinderDistanceWitness struct {
	Center                             [2]*big.Rat
	Radial, Height                     *big.Rat
	AzimuthX, AzimuthY, AzimuthSquared *big.Rat
	ArcRadial, ArcHeight, ArcRadius    *big.Rat
	ArcNormSquared                     *big.Rat
	Part                               CylinderBoundaryPart
}

// RoundedCylinderDistanceBounds encloses the global minimum squared
// boundary distance. Upper also bounds distance to Witness. The witness
// is a valid exact boundary point, without asserting that it is a minimizer.
type RoundedCylinderDistanceBounds struct {
	Lower, Upper  *big.Rat
	Witness       RoundedCylinderDistanceWitness
	PrecisionBits int
}

// BoundPointCircularRoundedCylinderDistance reduces global distance to
// the union of cap/lateral meridian segments and quarter circles. Exact
// radial enclosures are transported using the distance function's
// Lipschitz constant of one. Options bound squared error and exact work;
// MaxBisections controls the maximum fractional bits of root enclosures.
func BoundPointCircularRoundedCylinderDistance(ctx context.Context, p CircularRoundedCylinderProfile, query Vec3,
	o FrustumDistanceOptions) (RoundedCylinderDistanceBounds, error) {
	var empty RoundedCylinderDistanceBounds
	if ctx == nil || !query.IsFinite() || !finite(o.SquaredTolerance) || o.SquaredTolerance <= 0 ||
		o.MaxBisections < 0 || o.MaxBisections > 4096 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxBisections == 0 {
		o.MaxBisections = 256
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	centers, e := validateCircularRoundedCylinderProfile(p, budget)
	if e != nil {
		return empty, e
	}
	x := new(big.Rat).Sub(new(big.Rat).SetFloat64(query.X), p.Center[0])
	y := new(big.Rat).Sub(new(big.Rat).SetFloat64(query.Y), p.Center[1])
	z := new(big.Rat).SetFloat64(query.Z)
	square := func(n *big.Rat) *big.Rat { return new(big.Rat).Mul(n, n) }
	q := new(big.Rat).Add(square(x), square(y))
	tol := new(big.Rat).SetFloat64(o.SquaredTolerance)
	for _, n := range []*big.Rat{x, y, z, q, tol} {
		if e := budget.check(n); e != nil {
			return empty, e
		}
	}
	major := [2]*big.Rat{new(big.Rat).Sub(p.Radius, p.Blend[0]), new(big.Rat).Sub(p.Radius, p.Blend[1])}
	zero := new(big.Rat)
	half := big.NewRat(1, 2)
	for bits := min(o.MaxBisections, 8); ; bits = min(o.MaxBisections, bits+8) {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		a, b, e := rationalSqrtBounds(q, bits, budget)
		if e != nil {
			return empty, e
		}
		rho := new(big.Rat).Mul(new(big.Rat).Add(a, b), half)
		width := new(big.Rat).Mul(new(big.Rat).Sub(b, a), half)
		var result RoundedCylinderDistanceBounds
		result.PrecisionBits = bits
		update := func(lo, hi *big.Rat, w RoundedCylinderDistanceWitness) error {
			lo = new(big.Rat).Sub(lo, width)
			if lo.Sign() < 0 {
				lo.SetInt64(0)
			}
			hi = new(big.Rat).Add(hi, width)
			l, h := square(lo), square(hi)
			for _, n := range []*big.Rat{l, h} {
				if e := budget.check(n); e != nil {
					return e
				}
			}
			if result.Lower == nil || l.Cmp(result.Lower) < 0 {
				result.Lower = l
			}
			if result.Upper == nil || h.Cmp(result.Upper) < 0 {
				result.Upper, result.Witness = h, w
			}
			return nil
		}
		addPoint := func(r, h *big.Rat, part CylinderBoundaryPart) error {
			n := meridianSquared(distanceMeridianPoint{rho, z}, distanceMeridianPoint{r, h})
			lo, hi, e := rationalSqrtBounds(n, bits, budget)
			if e != nil {
				return e
			}
			return update(lo, hi, RoundedCylinderDistanceWitness{Radial: r, Height: h, Part: part})
		}
		clamp := func(x, a, b *big.Rat) *big.Rat {
			if x.Cmp(a) < 0 {
				return a
			}
			if x.Cmp(b) > 0 {
				return b
			}
			return x
		}
		if e := addPoint(p.Radius, clamp(z, centers[0], centers[1]), CylinderLateral); e != nil {
			return empty, e
		}
		for i, cap := range [2]CylinderBoundaryPart{CylinderLowerCap, CylinderUpperCap} {
			if e := addPoint(clamp(rho, zero, major[i]), p.Height[i], cap); e != nil {
				return empty, e
			}
			if p.Blend[i].Sign() == 0 {
				continue
			}
			part := [2]CylinderBoundaryPart{CylinderLowerBlend, CylinderUpperBlend}[i]
			A, B := new(big.Rat).Sub(rho, major[i]), new(big.Rat).Sub(z, centers[i])
			outward := new(big.Rat).Set(B)
			if i == 0 {
				outward.Neg(outward)
			}
			if A.Sign() <= 0 || outward.Sign() <= 0 {
				// Maximize the dot product over the quarter circle.
				// Outside its outward quadrant the winner is an endpoint.
				if A.Cmp(outward) >= 0 {
					e = addPoint(p.Radius, centers[i], part)
				} else {
					e = addPoint(major[i], p.Height[i], part)
				}
				if e != nil {
					return empty, e
				}
				continue
			}
			n := new(big.Rat).Add(square(A), square(B))
			lo, hi, e := rationalSqrtBounds(n, bits, budget)
			if e != nil {
				return empty, e
			}
			lo.Sub(lo, p.Blend[i])
			hi.Sub(hi, p.Blend[i])
			if hi.Sign() <= 0 {
				lo, hi = hi.Neg(hi), lo.Neg(lo)
			} else if lo.Sign() < 0 {
				if neg := new(big.Rat).Neg(lo); neg.Cmp(hi) > 0 {
					hi = neg
				}
				lo.SetInt64(0)
			}
			if e := update(lo, hi, RoundedCylinderDistanceWitness{Radial: major[i], Height: centers[i], Part: part,
				ArcRadial: A, ArcHeight: B, ArcRadius: p.Blend[i], ArcNormSquared: n}); e != nil {
				return empty, e
			}
		}
		if new(big.Rat).Sub(result.Upper, result.Lower).Cmp(tol) <= 0 {
			w := &result.Witness
			for i := range p.Center {
				w.Center[i] = new(big.Rat).Set(p.Center[i])
			}
			w.Radial, w.Height = new(big.Rat).Set(w.Radial), new(big.Rat).Set(w.Height)
			w.AzimuthX, w.AzimuthY, w.AzimuthSquared = x, y, q
			if q.Sign() == 0 {
				w.AzimuthX, w.AzimuthSquared = big.NewRat(1, 1), big.NewRat(1, 1)
			}
			if w.ArcRadius != nil {
				w.ArcRadius = new(big.Rat).Set(w.ArcRadius)
			}
			return result, nil
		}
		if bits == o.MaxBisections {
			return empty, ErrConvergence
		}
	}
}

func rationalSqrtBounds(q *big.Rat, bits int, budget *polynomialBudget) (*big.Rat, *big.Rat, error) {
	if e := budget.check(q); e != nil {
		return nil, nil, e
	}
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(new(big.Int).Set(n), n).Cmp(q.Num()) == 0 &&
		new(big.Int).Mul(new(big.Int).Set(d), d).Cmp(q.Denom()) == 0 {
		v := new(big.Rat).SetFrac(n, d)
		return v, new(big.Rat).Set(v), nil
	}
	if q.Num().BitLen()+2*bits > budget.maxBits {
		return nil, nil, ErrConvergence
	}
	den := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	value := new(big.Int).Lsh(new(big.Int).Set(q.Num()), uint(2*bits))
	n.Sqrt(value.Quo(value, q.Denom()))
	return new(big.Rat).SetFrac(n, den), new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), den), nil
}
