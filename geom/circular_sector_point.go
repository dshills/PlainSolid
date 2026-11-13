package geom

import (
	"context"
	"math/big"
)

// CircularSector is a closed finite cylinder sector, including its axis. Axis
// must be a signed coordinate unit vector. Raw radial rays need not be unit;
// they must define exactly a quarter or half turn in the selected handedness.
// Axial bounds are distances from origin along axis. This is ideal represented
// geometry, independent of tolerance-based analytic topology.
type CircularSector struct {
	origin, axis, start, end Vec3
	radius, lo, hi           float64
	hand                     int
}

func NewCircularSector(origin, axis, start, end Vec3, radius, lo, hi float64, hand int) (CircularSector, error) {
	s := CircularSector{origin, axis, start, end, radius, lo, hi, hand}
	if !origin.IsFinite() || !start.IsFinite() || !end.IsFinite() || !finite(radius) || !finite(lo) || !finite(hi) || radius <= 0 || lo >= hi || (hand != 1 && hand != -1) {
		return CircularSector{}, ErrInvalid
	}
	if axis != (Vec3{X: 1}) && axis != (Vec3{X: -1}) && axis != (Vec3{Y: 1}) && axis != (Vec3{Y: -1}) && axis != (Vec3{Z: 1}) && axis != (Vec3{Z: -1}) {
		return CircularSector{}, ErrInvalid
	}
	if _, e := ClassifyCircularSectorPoint(context.Background(), s, origin, SurfacePatchEvaluationOptions{}); e != nil {
		return CircularSector{}, e
	}
	return s, nil
}

type CircularSectorLocation uint8

const (
	SectorOutside CircularSectorLocation = iota
	SectorInterior
	SectorBoundary
)

type CircularSectorBoundary uint8

const (
	SectorOuterWall CircularSectorBoundary = 1 << iota
	SectorStartWall
	SectorEndWall
	SectorLowerCap
	SectorUpperCap
	SectorAxis
)

type CircularSectorPoint struct {
	Complete        bool
	Location        CircularSectorLocation
	Boundary        CircularSectorBoundary
	QuarterTurn     bool
	ExactOperations int
}

// ClassifyCircularSectorPoint uses exact dyadic arithmetic, including for tiny
// offsets and squared radii. Boundary flags are supplied only for material
// points. Half sectors have two opposing radial halves of one planar wall;
// the axis belongs to both. No trigonometric angle rounding enters this proof.
func ClassifyCircularSectorPoint(ctx context.Context, s CircularSector, p Vec3, o SurfacePatchEvaluationOptions) (out CircularSectorPoint, err error) {
	if ctx == nil || !p.IsFinite() || s.radius <= 0 || s.lo >= s.hi || (s.hand != 1 && s.hand != -1) {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 10000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.ExactOperations = b.operations }()
	rat := func(v Vec3) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
	}
	a, c, n, x, org := rat(s.start), rat(s.end), rat(s.axis), rat(p), rat(s.origin)
	for _, v := range [][3]*big.Rat{a, c, n, x, org} {
		for _, q := range v {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	dot := func(a, c [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i := 0; i < 3; i++ {
			q.Add(q, new(big.Rat).Mul(a[i], c[i]))
		}
		return q
	}
	cross := func(a, c [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			q.Add(q, new(big.Rat).Mul(n[i], new(big.Rat).Sub(new(big.Rat).Mul(a[j], c[k]), new(big.Rat).Mul(a[k], c[j]))))
		}
		if s.hand < 0 {
			q.Neg(q)
		}
		return q
	}
	for i := 0; i < 3; i++ {
		x[i].Sub(x[i], org[i])
	}
	ac, turn := dot(a, c), cross(a, c)
	if dot(a, a).Sign() == 0 || dot(c, c).Sign() == 0 || dot(a, n).Sign() != 0 || dot(c, n).Sign() != 0 {
		return out, ErrInvalid
	}
	quarter := ac.Sign() == 0 && turn.Sign() > 0
	half := turn.Sign() == 0 && ac.Sign() < 0
	if !quarter && !half {
		return out, ErrDomain
	}
	out.QuarterTurn = quarter
	z := dot(x, n)
	radial := new(big.Rat).Sub(dot(x, x), new(big.Rat).Mul(z, z))
	r := new(big.Rat).SetFloat64(s.radius)
	r.Mul(r, r)
	left, right := cross(a, x), cross(x, c)
	for _, q := range []*big.Rat{ac, turn, z, radial, r, left, right} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	low, high := z.Cmp(new(big.Rat).SetFloat64(s.lo)), z.Cmp(new(big.Rat).SetFloat64(s.hi))
	out.Complete = true
	if low < 0 || high > 0 || radial.Cmp(r) > 0 || left.Sign() < 0 || (quarter && right.Sign() < 0) {
		return out, nil
	}
	if low == 0 {
		out.Boundary |= SectorLowerCap
	}
	if high == 0 {
		out.Boundary |= SectorUpperCap
	}
	if radial.Cmp(r) == 0 {
		out.Boundary |= SectorOuterWall
	}
	if radial.Sign() == 0 {
		out.Boundary |= SectorAxis | SectorStartWall | SectorEndWall
	} else if quarter {
		if left.Sign() == 0 {
			out.Boundary |= SectorStartWall
		}
		if right.Sign() == 0 {
			out.Boundary |= SectorEndWall
		}
	} else if left.Sign() == 0 {
		if dot(a, x).Sign() > 0 {
			out.Boundary |= SectorStartWall
		} else {
			out.Boundary |= SectorEndWall
		}
	}
	out.Location = SectorInterior
	if out.Boundary != 0 {
		out.Location = SectorBoundary
	}
	return out, nil
}
