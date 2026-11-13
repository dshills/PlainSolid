package geom

import (
	"context"
	"math/big"
)

// SectorQuadraticParameter denotes Offset+Sign*sqrt(Radicand), exactly. Sign
// zero denotes a rational endpoint. Bounds are outward rational enclosures.
// All returned numbers belong to the caller.
type SectorQuadraticParameter struct {
	Offset, Radicand *big.Rat
	Sign             int
	Bounds           [2]*big.Rat
}
type CircularSectorHorizontalInterval struct {
	Complete, Empty, Singleton bool
	Endpoints                  [2]SectorQuadraticParameter
	EndpointBoundary           [2]CircularSectorBoundary
	InteriorBoundary           CircularSectorBoundary
	ExactOperations            int
}

// IntersectCircularSectorHorizontalLine intersects origin+t*direction with a
// closed quarter/half sector and a finite parameter range. Direction must have
// exactly one nonzero coordinate and be perpendicular to the rotation axis.
// Disk endpoints retain exact quadratic expressions; angular and segment clips
// are rational and ordered exactly. Enclosures use 96 fractional binary bits.
// InteriorBoundary names supporting boundary features; a half-sector line
// contained in the diametral plane names both radial halves, while individual
// endpoint masks distinguish those halves exactly.
func IntersectCircularSectorHorizontalLine(ctx context.Context, s CircularSector, origin, direction Vec3, tr [2]float64, o SurfacePatchEvaluationOptions) (out CircularSectorHorizontalInterval, err error) {
	if ctx == nil || !origin.IsFinite() || !direction.IsFinite() || !finite(tr[0]) || !finite(tr[1]) || tr[0] > tr[1] {
		return out, ErrInvalid
	}
	count := 0
	for i := 0; i < 3; i++ {
		if coordinate(direction, i) != 0 {
			count++
			if coordinate(s.axis, i) != 0 {
				return out, ErrDomain
			}
		}
	}
	if count != 1 {
		return out, ErrInvalid
	}
	// Validate the immutable sector and shared budget without relying on a
	// rounded local coordinate transform.
	q, e := ClassifyCircularSectorPoint(ctx, s, s.origin, o)
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: q.ExactOperations}
	defer func() { out.ExactOperations = b.operations }()
	r := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	x, d, n, a, c := [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}
	for i := 0; i < 3; i++ {
		x[i] = new(big.Rat).Sub(r(coordinate(origin, i)), r(coordinate(s.origin, i)))
		d[i] = r(coordinate(direction, i))
		n[i] = r(coordinate(s.axis, i))
		a[i] = r(coordinate(s.start, i))
		c[i] = r(coordinate(s.end, i))
	}
	dot := func(v, w [3]*big.Rat) *big.Rat {
		z := new(big.Rat)
		for i := 0; i < 3; i++ {
			z.Add(z, new(big.Rat).Mul(v[i], w[i]))
		}
		return z
	}
	cross := func(v, w [3]*big.Rat) *big.Rat {
		z := new(big.Rat)
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			z.Add(z, new(big.Rat).Mul(n[i], new(big.Rat).Sub(new(big.Rat).Mul(v[j], w[k]), new(big.Rat).Mul(v[k], w[j]))))
		}
		if s.hand < 0 {
			z.Neg(z)
		}
		return z
	}
	z := dot(x, n)
	if z.Cmp(r(s.lo)) < 0 || z.Cmp(r(s.hi)) > 0 {
		out.Complete = true
		out.Empty = true
		return out, nil
	}
	if z.Cmp(r(s.lo)) == 0 {
		out.InteriorBoundary |= SectorLowerCap
	}
	if z.Cmp(r(s.hi)) == 0 {
		out.InteriorBoundary |= SectorUpperCap
	}
	lo, hi := r(tr[0]), r(tr[1])
	type wall struct {
		constant, slope *big.Rat
		flag            CircularSectorBoundary
	}
	walls := []wall{{cross(a, x), cross(a, d), SectorStartWall}}
	if q.QuarterTurn {
		walls = append(walls, wall{cross(x, c), cross(d, c), SectorEndWall})
	}
	for _, w := range walls {
		if w.slope.Sign() == 0 {
			if w.constant.Sign() < 0 {
				out.Complete = true
				out.Empty = true
				return out, nil
			}
			if w.constant.Sign() == 0 {
				out.InteriorBoundary |= w.flag
				if !q.QuarterTurn {
					out.InteriorBoundary |= SectorEndWall
				}
			}
			continue
		}
		cut := new(big.Rat).Quo(new(big.Rat).Neg(w.constant), w.slope)
		if w.slope.Sign() > 0 && cut.Cmp(lo) > 0 {
			lo = cut
		}
		if w.slope.Sign() < 0 && cut.Cmp(hi) < 0 {
			hi = cut
		}
	}
	A := dot(d, d)
	center := new(big.Rat).Quo(new(big.Rat).Neg(dot(x, d)), A)
	radial := new(big.Rat).Sub(dot(x, x), new(big.Rat).Mul(z, z))
	rad := new(big.Rat).Add(new(big.Rat).Mul(center, center), new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(r(s.radius), r(s.radius)), radial), A))
	for _, v := range []*big.Rat{lo, hi, A, center, rad} {
		if e = b.check(v); e != nil {
			return out, e
		}
	}
	if lo.Cmp(hi) > 0 || rad.Sign() < 0 {
		out.Complete = true
		out.Empty = true
		return out, nil
	}
	// Compare center+sign*sqrt(rad) with a rational without approximation.
	cmp := func(sign int, v *big.Rat) int {
		delta := new(big.Rat).Sub(v, center)
		if sign < 0 {
			delta.Neg(delta)
		}
		var k int
		if delta.Sign() < 0 {
			k = 1
		} else {
			k = rad.Cmp(new(big.Rat).Mul(delta, delta))
		}
		if sign < 0 {
			k = -k
		}
		return k
	}
	if cmp(1, lo) < 0 || cmp(-1, hi) > 0 {
		out.Complete = true
		out.Empty = true
		return out, nil
	}
	sl, sh, e := rationalSqrtBounds(rad, 96, b)
	if e != nil {
		return out, e
	}
	for i, sign := range []int{-1, 1} {
		v := lo
		if i == 1 {
			v = hi
		}
		useRational := (i == 0 && cmp(sign, v) <= 0) || (i == 1 && cmp(sign, v) >= 0)
		ep := SectorQuadraticParameter{Offset: new(big.Rat).Set(center), Radicand: new(big.Rat).Set(rad), Sign: sign}
		if useRational {
			ep.Offset = new(big.Rat).Set(v)
			ep.Radicand = new(big.Rat)
			ep.Sign = 0
			ep.Bounds = [2]*big.Rat{new(big.Rat).Set(v), new(big.Rat).Set(v)}
		} else if sign < 0 {
			ep.Bounds = [2]*big.Rat{new(big.Rat).Sub(center, sh), new(big.Rat).Sub(center, sl)}
		} else {
			ep.Bounds = [2]*big.Rat{new(big.Rat).Add(center, sl), new(big.Rat).Add(center, sh)}
		}
		out.Endpoints[i] = ep
		out.EndpointBoundary[i] = out.InteriorBoundary
		if !useRational || cmp(sign, v) == 0 {
			out.EndpointBoundary[i] |= SectorOuterWall
		}
		if ep.Sign == 0 {
			for _, w := range walls {
				if new(big.Rat).Add(w.constant, new(big.Rat).Mul(w.slope, ep.Offset)).Sign() == 0 {
					out.EndpointBoundary[i] |= w.flag
				}
			}
		}

		if !q.QuarterTurn && out.InteriorBoundary&(SectorStartWall|SectorEndWall) != 0 {
			base, slope := dot(a, x), dot(a, d)
			side := base.Sign()
			if slope.Sign() != 0 {
				cut := new(big.Rat).Quo(new(big.Rat).Neg(base), slope)
				if ep.Sign == 0 {
					side = ep.Offset.Cmp(cut)
				} else {
					side = cmp(ep.Sign, cut)
				}
				side *= slope.Sign()
			}
			out.EndpointBoundary[i] &^= SectorStartWall | SectorEndWall
			if side > 0 {
				out.EndpointBoundary[i] |= SectorStartWall
			} else if side < 0 {
				out.EndpointBoundary[i] |= SectorEndWall
			} else {
				out.EndpointBoundary[i] |= SectorAxis | SectorStartWall | SectorEndWall
			}
		}
		if ep.Sign == 0 {
			point := [3]*big.Rat{}
			for k := 0; k < 3; k++ {
				point[k] = new(big.Rat).Add(x[k], new(big.Rat).Mul(d[k], ep.Offset))
			}
			rr := new(big.Rat).Sub(dot(point, point), new(big.Rat).Mul(z, z))
			if rr.Sign() == 0 {
				out.EndpointBoundary[i] |= SectorAxis | SectorStartWall | SectorEndWall
			} else if !q.QuarterTurn && cross(a, point).Sign() == 0 {
				out.EndpointBoundary[i] &^= SectorStartWall | SectorEndWall
				if dot(a, point).Sign() > 0 {
					out.EndpointBoundary[i] |= SectorStartWall
				} else {
					out.EndpointBoundary[i] |= SectorEndWall
				}
			}
		}
	}
	out.Singleton = rad.Sign() == 0 || lo.Cmp(hi) == 0 || cmp(1, lo) == 0 || cmp(-1, hi) == 0
	out.Complete = true
	return out, nil
}
