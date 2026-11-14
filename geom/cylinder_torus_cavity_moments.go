package geom

import (
	"context"
	"math/big"
)

// CylinderTorusCavityMoments retains exact coefficients a*pi+b*pi² for
// Volume, FirstZ, SecondXX=SecondYY, SecondZZ, in that order. Other raw first
// and mixed second moments vanish by axial symmetry. Enclosures include the
// centroidal quotient; coefficient rationals are independently owned.
type CylinderTorusCavityMoments struct {
	TorusCylinderMoments
	Coefficients [4][2]*big.Rat
}

// BoundMoments subtracts complete original torus integrals from the finite
// cylinder. One ledger covers coefficients and Machin pi intervals. Precision
// and requested width failures are atomic; no spatial refinement is needed.
func (c CylinderTorusCavityMaterial) BoundMoments(ctx context.Context, o TorusCylinderMomentOptions) (out CylinderTorusCavityMoments, err error) {
	s := c.source
	defer func() {
		if err != nil {
			out = CylinderTorusCavityMoments{}
		}
	}()
	if ctx == nil || s.R == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, t := range []*big.Rat{o.VolumeTolerance, o.SecondMomentTolerance, o.CentroidTolerance, o.InertiaTolerance} {
		if t == nil || t.Sign() <= 0 {
			return out, ErrInvalid
		}
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 64
	}
	if o.MaxCells < 1 || o.MaxCells > 65536 || o.PrecisionBits < 16 || o.PrecisionBits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, q := range []*big.Rat{s.R, s.r, s.c, s.lower, s.upper, o.VolumeTolerance, o.SecondMomentTolerance, o.CentroidTolerance, o.InertiaTolerance} {
		a.check(q)
	}
	if a.err != nil {
		return out, a.err
	}
	tol := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(o.PrecisionBits+16)))
	terms := 4*o.PrecisionBits + 64
	u, e := helicalAtanSmall(ctx, big.NewRat(1, 5), tol, terms, b)
	if e != nil {
		return out, e
	}
	v, e := helicalAtanSmall(ctx, big.NewRat(1, 239), tol, terms, b)
	if e != nil {
		return out, e
	}
	pi := [2]*big.Rat{a.sub(a.mul(big.NewRat(16, 1), u[0]), a.mul(big.NewRat(4, 1), v[1])), a.sub(a.mul(big.NewRat(16, 1), u[1]), a.mul(big.NewRat(4, 1), v[0]))}
	type interval = [2]*big.Rat
	add := func(x, y interval) interval { return interval{a.add(x[0], y[0]), a.add(x[1], y[1])} }
	sub := func(x, y interval) interval { return interval{a.sub(x[0], y[1]), a.sub(x[1], y[0])} }
	scale := func(x interval, c *big.Rat) interval {
		if c.Sign() < 0 {
			return interval{a.mul(x[1], c), a.mul(x[0], c)}
		}
		return interval{a.mul(x[0], c), a.mul(x[1], c)}
	}
	mulPos := func(x, y interval) interval { return interval{a.mul(x[0], y[0]), a.mul(x[1], y[1])} }
	divPos := func(x, y interval) interval {
		var z interval
		for _, xx := range x {
			for _, yy := range y {
				q := a.div(xx, yy)
				if z[0] == nil || q.Cmp(z[0]) < 0 {
					z[0] = q
				}
				if z[1] == nil || q.Cmp(z[1]) > 0 {
					z[1] = q
				}
			}
		}
		return z
	}
	clamp := func(x interval) interval {
		if x[0].Sign() < 0 {
			x[0] = new(big.Rat)
		}
		return x
	}

	r2, R2, c2 := a.mul(s.r, s.r), a.mul(s.R, s.R), a.mul(s.c, s.c)
	h := a.sub(s.upper, s.lower)
	cv := a.mul(c2, h)
	tv := a.mul(big.NewRat(2, 1), a.mul(s.R, r2))
	coefficients := [4][2]*big.Rat{
		{cv, a.sub(new(big.Rat), tv)},
		{a.mul(cv, a.div(a.add(s.upper, s.lower), big.NewRat(2, 1))), new(big.Rat)},
		{a.mul(cv, a.div(c2, big.NewRat(4, 1))), a.sub(new(big.Rat), a.mul(tv, a.add(a.div(R2, big.NewRat(2, 1)), a.mul(big.NewRat(3, 8), r2))))},
		{a.mul(cv, a.div(a.add(a.add(a.mul(s.upper, s.upper), a.mul(s.upper, s.lower)), a.mul(s.lower, s.lower)), big.NewRat(3, 1))), a.sub(new(big.Rat), a.mul(tv, a.div(r2, big.NewRat(4, 1))))},
	}
	out.Coefficients = coefficients
	eval := func(q [2]*big.Rat) interval { return add(scale(pi, q[0]), scale(mulPos(pi, pi), q[1])) }
	out.Volume = eval(coefficients[0])
	out.FirstZ = eval(coefficients[1])
	out.SecondXY = eval(coefficients[2])
	out.SecondZ = eval(coefficients[3])
	if a.err != nil {
		return out, a.err
	}
	if out.Volume[0].Sign() <= 0 {
		return out, ErrConvergence
	}
	out.CentroidZ = divPos(out.FirstZ, out.Volume)
	square := interval{a.mul(out.FirstZ[0], out.FirstZ[0]), a.mul(out.FirstZ[1], out.FirstZ[1])}
	if square[0].Cmp(square[1]) > 0 {
		square[0], square[1] = square[1], square[0]
	}
	if out.FirstZ[0].Sign() <= 0 && out.FirstZ[1].Sign() >= 0 {
		square[0] = new(big.Rat)
	}
	shift := divPos(square, out.Volume)
	out.InertiaOrigin[0] = add(out.SecondXY, out.SecondZ)
	out.InertiaOrigin[1] = add(out.SecondXY, out.SecondZ)
	out.InertiaOrigin[2] = scale(out.SecondXY, big.NewRat(2, 1))
	for i := 0; i < 2; i++ {
		out.InertiaCentroid[i] = clamp(sub(out.InertiaOrigin[i], shift))
	}
	out.InertiaCentroid[2] = scale(out.SecondXY, big.NewRat(2, 1))
	within := func(v interval, t *big.Rat) bool { return a.sub(v[1], v[0]).Cmp(t) <= 0 }
	ok := within(out.Volume, o.VolumeTolerance) && within(out.SecondXY, o.SecondMomentTolerance) && within(out.SecondZ, o.SecondMomentTolerance) && within(out.CentroidZ, o.CentroidTolerance)
	for i := 0; i < 3; i++ {
		ok = ok && within(out.InertiaOrigin[i], o.InertiaTolerance) && within(out.InertiaCentroid[i], o.InertiaTolerance)
	}
	if a.err != nil {
		return out, a.err
	}
	if !ok {
		return out, ErrConvergence
	}
	out.ExactOperations = b.operations
	return out, nil
}
