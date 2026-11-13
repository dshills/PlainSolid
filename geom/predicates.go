package geom

import (
	"math"
	"math/big"
)

// predicateInterval encloses the exact value of floating-point expressions.
// Every arithmetic result is rounded outward by one representable value. This
// deliberately conservative filter avoids assumed determinant error constants.
type predicateInterval struct{ lo, hi float64 }

func piPoint(v float64) predicateInterval { return predicateInterval{v, v} }
func piAdd(a, b predicateInterval) predicateInterval {
	return predicateInterval{math.Nextafter(a.lo+b.lo, math.Inf(-1)), math.Nextafter(a.hi+b.hi, math.Inf(1))}
}
func piNeg(a predicateInterval) predicateInterval    { return predicateInterval{-a.hi, -a.lo} }
func piSub(a, b predicateInterval) predicateInterval { return piAdd(a, piNeg(b)) }
func piMul(a, b predicateInterval) predicateInterval {
	v := [4]float64{a.lo * b.lo, a.lo * b.hi, a.hi * b.lo, a.hi * b.hi}
	lo, hi := v[0], v[0]
	for _, x := range v[1:] {
		lo = math.Min(lo, x)
		hi = math.Max(hi, x)
	}
	return predicateInterval{math.Nextafter(lo, math.Inf(-1)), math.Nextafter(hi, math.Inf(1))}
}
func piDet(a [][]predicateInterval) predicateInterval {
	if len(a) == 1 {
		return a[0][0]
	}
	result := piPoint(0)
	for j := range a[0] {
		minor := make([][]predicateInterval, len(a)-1)
		for i := 1; i < len(a); i++ {
			row := make([]predicateInterval, 0, len(a)-1)
			row = append(row, a[i][:j]...)
			row = append(row, a[i][j+1:]...)
			minor[i-1] = row
		}
		term := piMul(a[0][j], piDet(minor))
		if j%2 == 1 {
			term = piNeg(term)
		}
		result = piAdd(result, term)
	}
	return result
}
func exactDet(a [][]*big.Rat) *big.Rat {
	if len(a) == 1 {
		return new(big.Rat).Set(a[0][0])
	}
	result := new(big.Rat)
	for j := range a[0] {
		minor := make([][]*big.Rat, len(a)-1)
		for i := 1; i < len(a); i++ {
			row := make([]*big.Rat, 0, len(a)-1)
			row = append(row, a[i][:j]...)
			row = append(row, a[i][j+1:]...)
			minor[i-1] = row
		}
		term := new(big.Rat).Mul(a[0][j], exactDet(minor))
		if j%2 == 1 {
			term.Neg(term)
		}
		result.Add(result, term)
	}
	return result
}
func coordinate(p Vec3, i int) float64 {
	switch i {
	case 0:
		return p.X
	case 1:
		return p.Y
	default:
		return p.Z
	}
}

// scaledPredicatePoints returns an exact common power-of-two rescaling. Cases
// where that rescaling would make a nonzero coordinate subnormal use exact
// rationals instead, retaining the entire finite float64 coordinate envelope.
func scaledPredicatePoints(points []Vec3) ([]Vec3, bool) {
	max := 0.0
	for _, p := range points {
		max = math.Max(max, math.Max(math.Abs(p.X), math.Max(math.Abs(p.Y), math.Abs(p.Z))))
	}
	_, exponent := math.Frexp(max)
	scaled := make([]Vec3, len(points))
	for i, p := range points {
		var out [3]float64
		for j := 0; j < 3; j++ {
			v := coordinate(p, j)
			out[j] = math.Ldexp(v, -exponent)
			if v != 0 && math.Abs(out[j]) < 0x1p-1022 {
				return nil, false
			}
		}
		scaled[i] = Vec3{out[0], out[1], out[2]}
	}
	return scaled, true
}
func spatialPredicate(points []Vec3, lifted bool) (int, error) {
	for _, p := range points {
		if !p.IsFinite() {
			return 0, ErrInvalid
		}
	}
	n := len(points) - 1
	if scaled, ok := scaledPredicatePoints(points); ok {
		origin := scaled[n]
		a := make([][]predicateInterval, n)
		for i := 0; i < n; i++ {
			a[i] = make([]predicateInterval, n)
			lift := piPoint(0)
			for j := 0; j < 3; j++ {
				d := piSub(piPoint(coordinate(scaled[i], j)), piPoint(coordinate(origin, j)))
				a[i][j] = d
				if lifted {
					lift = piAdd(lift, piMul(d, d))
				}
			}
			if lifted {
				a[i][3] = lift
			}
		}
		det := piDet(a)
		if det.lo > 0 {
			return 1, nil
		}
		if det.hi < 0 {
			return -1, nil
		}
	}
	origin := points[n]
	a := make([][]*big.Rat, n)
	for i := 0; i < n; i++ {
		a[i] = make([]*big.Rat, n)
		lift := new(big.Rat)
		for j := 0; j < 3; j++ {
			d := new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(points[i], j)), new(big.Rat).SetFloat64(coordinate(origin, j)))
			a[i][j] = d
			if lifted {
				lift.Add(lift, new(big.Rat).Mul(d, d))
			}
		}
		if lifted {
			a[i][3] = lift
		}
	}
	return exactDet(a).Sign(), nil
}

// Orient3D returns the exact sign of (a-d) dot ((b-d) cross (c-d)) for finite
// float64 coordinates. Zero denotes coplanarity. A scaled outward-rounded
// interval filter is followed by exact rational arithmetic when inconclusive.
func Orient3D(a, b, c, d Vec3) (int, error) { return spatialPredicate([]Vec3{a, b, c, d}, false) }

// InSphere returns the exact oriented in-sphere determinant sign. For a
// noncoplanar tetrahedron (a,b,c,d), a nonzero sign equal to Orient3D(a,b,c,d)
// means e is inside its circumsphere; the opposite sign means outside. Zero
// means cospherical (or a degenerate determinant). If the tetrahedron is
// coplanar this determinant does not define an inside/outside classification.
func InSphere(a, b, c, d, e Vec3) (int, error) { return spatialPredicate([]Vec3{a, b, c, d, e}, true) }
