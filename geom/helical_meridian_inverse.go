package geom

import (
	"context"
	"math/big"
)

// HelicalMeridianPoint is an exact query at one prescribed turn, not a global
// surface/solid classification. ExcludedAtTurn does not exclude another turn.
// OnMeridian includes the infinite profile line; OnSurface additionally requires
// 0<=ProfileParameter<=1. Parameter and Point exist only for OnMeridian.
// Boundary is -1 at profile start, +1 at end, otherwise zero. Rationals are owned.
type HelicalMeridianPoint struct {
	OnMeridian, OnSurface, ExcludedAtTurn bool
	Turn, ProfileParameter                *big.Rat
	Point                                 [3]*big.Rat
	Boundary                              int
	ExactOperations                       int
}

// InvertQuarterTurnPoint proves membership of an exact rational model point in
// the selected meridian, only when 4*turn is an integer. At those phases the
// exact analytic sine/cosine equal 0 or ±1. Arbitrary phases are unsupported
// (ErrConvergence), never inferred from numerical trigonometric residuals.
// All arithmetic uses the represented frame directly, so its floating-point
// normalization is not assumed to give an exactly orthogonal matrix.
func (s HelicalProfileSurface) InvertQuarterTurnPoint(ctx context.Context, turn *big.Rat, point [3]*big.Rat, o SurfacePatchEvaluationOptions) (out HelicalMeridianPoint, err error) {
	if ctx == nil || turn == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if e := s.check(s.domain.UMin, 0); e != nil {
		return out, e
	}
	for _, q := range point {
		if q == nil {
			return out, ErrInvalid
		}
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.ExactOperations = b.operations }()
	for _, q := range append([]*big.Rat{turn}, point[:]...) {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	if turn.Cmp(new(big.Rat).SetFloat64(s.domain.UMin)) < 0 || turn.Cmp(new(big.Rat).SetFloat64(s.domain.UMax)) > 0 {
		return out, ErrDomain
	}
	phase := new(big.Rat).Mul(turn, big.NewRat(4*int64(s.handedness), 1))
	if e := b.check(phase); e != nil {
		return out, e
	}
	if !phase.IsInt() {
		return out, ErrConvergence
	}
	quadrant := new(big.Int).Mod(phase.Num(), big.NewInt(4)).Int64()
	cosines := [4]int64{1, 0, -1, 0}
	sines := [4]int64{0, 1, 0, -1}
	cs, sn := big.NewRat(cosines[quadrant], 1), big.NewRat(sines[quadrant], 1)
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	origin := [3]float64{s.frame.origin.X, s.frame.origin.Y, s.frame.origin.Z}
	x := [3]float64{s.frame.x.X, s.frame.x.Y, s.frame.x.Z}
	y := [3]float64{s.frame.y.X, s.frame.y.Y, s.frame.y.Z}
	z := [3]float64{s.frame.z.X, s.frame.z.Y, s.frame.z.Z}
	dr, dz := new(big.Rat).Sub(rat(s.end.X), rat(s.start.X)), new(big.Rat).Sub(rat(s.end.Y), rat(s.start.Y))
	height := new(big.Rat).Add(new(big.Rat).Mul(rat(s.lead), turn), rat(s.start.Y))
	var start, direction [3]*big.Rat
	selected := -1
	for i := 0; i < 3; i++ {
		radial := new(big.Rat).Add(new(big.Rat).Mul(rat(x[i]), cs), new(big.Rat).Mul(rat(y[i]), sn))
		start[i] = new(big.Rat).Add(rat(origin[i]), new(big.Rat).Add(new(big.Rat).Mul(radial, rat(s.start.X)), new(big.Rat).Mul(rat(z[i]), height)))
		direction[i] = new(big.Rat).Add(new(big.Rat).Mul(radial, dr), new(big.Rat).Mul(rat(z[i]), dz))
		for _, q := range []*big.Rat{start[i], direction[i]} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		if selected < 0 && direction[i].Sign() != 0 {
			selected = i
		}
	}
	if selected < 0 {
		return out, ErrDegenerate
	}
	parameter := new(big.Rat).Quo(new(big.Rat).Sub(point[selected], start[selected]), direction[selected])
	if e := b.check(parameter); e != nil {
		return out, e
	}
	out.Turn = new(big.Rat).Set(turn)
	for i := 0; i < 3; i++ {
		predicted := new(big.Rat).Add(start[i], new(big.Rat).Mul(direction[i], parameter))
		if e := b.check(predicted); e != nil {
			return out, e
		}
		if predicted.Cmp(point[i]) != 0 {
			out.ExcludedAtTurn = true
			return out, nil
		}
	}
	out.OnMeridian = true
	out.ProfileParameter = parameter
	for i, q := range point {
		out.Point[i] = new(big.Rat).Set(q)
	}
	out.OnSurface = parameter.Sign() >= 0 && parameter.Cmp(big.NewRat(1, 1)) <= 0
	out.ExcludedAtTurn = !out.OnSurface
	if parameter.Sign() == 0 {
		out.Boundary = -1
	} else if parameter.Cmp(big.NewRat(1, 1)) == 0 {
		out.Boundary = 1
	}
	return out, nil
}
