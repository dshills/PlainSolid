package geom

import (
	"context"
	"math/big"
)

// EccentricCylinderTorusCavityMoments gives exact a*pi+b*pi² coefficients
// for every raw unit-density moment. Central inertia/centroid contain quotients
// and are reported as outward intervals, not polynomial pi coefficients.
type EccentricCylinderTorusCavityMoments struct {
	Volume                                 [2]*big.Rat
	First, Centroid                        [3][2]*big.Rat
	Second, InertiaOrigin, InertiaCentroid [3][3][2]*big.Rat
	VolumeCoefficients                     [2]*big.Rat
	FirstCoefficients                      [3][2]*big.Rat
	SecondCoefficients                     [3][3][2]*big.Rat
	ExactOperations                        int
}

// BoundMoments translates only the removed original torus, using one bounded
// ledger for exact coefficients and rigorous pi intervals. Failures are atomic.
func (c EccentricCylinderTorusCavityMaterial) BoundMoments(ctx context.Context, o TorusCylinderMomentOptions) (out EccentricCylinderTorusCavityMoments, err error) {
	s := c.source
	defer func() {
		if err != nil {
			out = EccentricCylinderTorusCavityMoments{}
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

	for _, v := range c.offset {
		a.check(v)
	}
	clone := func(v [2]*big.Rat) [2]*big.Rat { return [2]*big.Rat{new(big.Rat).Set(v[0]), new(big.Rat).Set(v[1])} }
	zero := func() [2]*big.Rat { return [2]*big.Rat{new(big.Rat), new(big.Rat)} }
	out.VolumeCoefficients = clone(coefficients[0])
	out.FirstCoefficients[2] = clone(coefficients[1])
	for i := 0; i < 2; i++ {
		out.FirstCoefficients[i] = [2]*big.Rat{new(big.Rat), a.sub(new(big.Rat), a.mul(tv, c.offset[i]))}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			q := zero()
			if i == j {
				at := 2
				if i == 2 {
					at = 3
				}
				q = clone(coefficients[at])
			}
			if i < 2 && j < 2 {
				q[1] = a.sub(q[1], a.mul(tv, a.mul(c.offset[i], c.offset[j])))
			}
			out.SecondCoefficients[i][j] = q
		}
	}
	eval := func(q [2]*big.Rat) interval { return add(scale(pi, q[0]), scale(mulPos(pi, pi), q[1])) }
	product := func(x, y interval) interval {
		var v interval
		for _, xx := range x {
			for _, yy := range y {
				z := a.mul(xx, yy)
				if v[0] == nil || z.Cmp(v[0]) < 0 {
					v[0] = z
				}
				if v[1] == nil || z.Cmp(v[1]) > 0 {
					v[1] = z
				}
			}
		}
		return v
	}
	out.Volume = eval(out.VolumeCoefficients)
	if a.err != nil {
		return out, a.err
	}
	if out.Volume[0].Sign() <= 0 {
		return out, ErrConvergence
	}
	var central [3][3]interval
	for i := 0; i < 3; i++ {
		out.First[i] = eval(out.FirstCoefficients[i])
		out.Centroid[i] = divPos(out.First[i], out.Volume)
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			out.Second[i][j] = eval(out.SecondCoefficients[i][j])
			central[i][j] = sub(out.Second[i][j], divPos(product(out.First[i], out.First[j]), out.Volume))
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i != j {
				out.InertiaOrigin[i][j] = scale(out.Second[i][j], big.NewRat(-1, 1))
				out.InertiaCentroid[i][j] = scale(central[i][j], big.NewRat(-1, 1))
			} else {
				out.InertiaOrigin[i][j] = zero()
				out.InertiaCentroid[i][j] = zero()
				for k := 0; k < 3; k++ {
					if k != i {
						out.InertiaOrigin[i][j] = add(out.InertiaOrigin[i][j], out.Second[k][k])
						out.InertiaCentroid[i][j] = add(out.InertiaCentroid[i][j], central[k][k])
					}
				}
			}
		}
	}
	within := func(v interval, t *big.Rat) bool { return a.sub(v[1], v[0]).Cmp(t) <= 0 }
	ok := within(out.Volume, o.VolumeTolerance)
	for i := 0; i < 3; i++ {
		ok = ok && within(out.Centroid[i], o.CentroidTolerance)
		for j := 0; j < 3; j++ {
			ok = ok && within(out.Second[i][j], o.SecondMomentTolerance) && within(out.InertiaOrigin[i][j], o.InertiaTolerance) && within(out.InertiaCentroid[i][j], o.InertiaTolerance)
		}
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
