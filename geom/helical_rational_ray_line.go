package geom

import (
	"context"
	"math/big"
)

// HelicalRayLineOptions adds phase enclosure precision to finite-turn limits.
// PhaseBits defaults to64 and must be in[16,512]. Boundaries not separated by
// the computed enclosure remain unresolved; increasing PhaseBits may resolve them.
type HelicalRayLineOptions struct {
	HelicalAxisLineOptions
	PhaseBits int
}

// HelicalRayLineHit has exact profile coordinate and rational closed enclosures
// for transcendental original turn, caller line parameter, and original XYZ.
type HelicalRayLineHit struct {
	Turn, LineParameter [2]*big.Rat
	ProfileParameter    *big.Rat
	Point               [3][2]*big.Rat
}
type HelicalRayLineContacts struct {
	Complete                        bool
	Hits                            []HelicalRayLineHit
	CandidateTurns, ExactOperations int
	Phase                           [2]*big.Rat
}

// IntersectRationalRayAxisLine supports exact rational unit rays in all four
// open quadrants, including (±3/5,±4/5), with either handedness. Cardinal axes
// use IntersectCardinalAxisLine instead. The caller line is
// origin+lambda*axisScale*Frame.ZAxis. Exact radial inversion and an invertible
// represented frame prove all possible roots U=phase+n. Rational alternating
// arctangent bounds and Machin's identity enclose phase; no rounded atan2 is used.
// Complete means all finite-domain turns and line clipping are decided. Errors
// discard partial hits. Coincident constant-radius meridians stay unresolved.
func (s HelicalProfileSurface) IntersectRationalRayAxisLine(ctx context.Context, cs, sn *big.Rat, origin [3]*big.Rat, axisScale *big.Rat, options HelicalRayLineOptions) (out HelicalRayLineContacts, err error) {
	o := options.HelicalAxisLineOptions
	bits := options.PhaseBits
	if bits == 0 {
		bits = 64
	}
	if bits < 16 || bits > 512 {
		return out, ErrInvalid
	}
	if ctx == nil || cs == nil || sn == nil || axisScale == nil || axisScale.Sign() == 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if e := s.check(s.domain.UMin, 0); e != nil {
		return out, e
	}
	for _, q := range origin {
		if q == nil {
			return out, ErrInvalid
		}
	}
	if o.MaxContacts == 0 {
		o.MaxContacts = 1024
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxContacts < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Hits = nil
		}
	}()
	inputs := append([]*big.Rat{cs, sn, axisScale}, origin[:]...)
	if o.LineRange != nil {
		if o.LineRange[0] == nil || o.LineRange[1] == nil || o.LineRange[0].Cmp(o.LineRange[1]) > 0 {
			return out, ErrInvalid
		}
		inputs = append(inputs, o.LineRange[:]...)
	}
	for _, q := range inputs {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	if cs.Sign() == 0 || sn.Sign() == 0 {
		return out, ErrInvalid
	}
	unit := new(big.Rat).Add(new(big.Rat).Mul(cs, cs), new(big.Rat).Mul(sn, sn))
	if e := b.check(unit); e != nil {
		return out, e
	}
	if unit.Cmp(big.NewRat(1, 1)) != 0 {
		return out, ErrInvalid
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x := [3]float64{s.frame.x.X, s.frame.x.Y, s.frame.x.Z}
	y := [3]float64{s.frame.y.X, s.frame.y.Y, s.frame.y.Z}
	z := [3]float64{s.frame.z.X, s.frame.z.Y, s.frame.z.Z}
	offset := [3]float64{s.frame.origin.X, s.frame.origin.Y, s.frame.origin.Z}
	var radial, tangent, axis, delta [3]*big.Rat
	for i := 0; i < 3; i++ {
		radial[i] = new(big.Rat).Add(new(big.Rat).Mul(rat(x[i]), cs), new(big.Rat).Mul(rat(y[i]), sn))
		tangent[i] = new(big.Rat).Sub(new(big.Rat).Mul(rat(y[i]), cs), new(big.Rat).Mul(rat(x[i]), sn))
		axis[i] = rat(z[i])
		delta[i] = new(big.Rat).Sub(origin[i], rat(offset[i]))
	}
	determinant := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		determinant.Add(determinant, new(big.Rat).Mul(tangent[i], new(big.Rat).Sub(new(big.Rat).Mul(radial[j], axis[k]), new(big.Rat).Mul(radial[k], axis[j]))))
	}
	if e := b.check(determinant); e != nil {
		return out, e
	}
	if determinant.Sign() == 0 {
		return out, ErrDegenerate
	}
	var radius, axialOrigin *big.Rat
	for i := 0; i < 3 && radius == nil; i++ {
		for j := i + 1; j < 3; j++ {
			det := new(big.Rat).Sub(new(big.Rat).Mul(radial[i], axis[j]), new(big.Rat).Mul(radial[j], axis[i]))
			if det.Sign() != 0 {
				radius = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(delta[i], axis[j]), new(big.Rat).Mul(delta[j], axis[i])), det)
				axialOrigin = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(radial[i], delta[j]), new(big.Rat).Mul(radial[j], delta[i])), det)
				break
			}
		}
	}
	if radius == nil {
		return out, ErrDegenerate
	}
	for _, q := range []*big.Rat{radius, axialOrigin} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	for i := 0; i < 3; i++ {
		v := new(big.Rat).Add(new(big.Rat).Mul(radial[i], radius), new(big.Rat).Mul(axis[i], axialOrigin))
		if e := b.check(v); e != nil {
			return out, e
		}
		if v.Cmp(delta[i]) != 0 {
			return out, ErrConvergence
		}
	}
	if radius.Sign() == 0 {
		out.Complete = true
		return out, nil
	}
	if radius.Sign() < 0 {
		return out, ErrConvergence
	}
	dr := new(big.Rat).Sub(rat(s.end.X), rat(s.start.X))
	if dr.Sign() == 0 {
		if radius.Cmp(rat(s.start.X)) == 0 {
			return out, ErrConvergence
		}
		out.Complete = true
		return out, nil
	}
	v := new(big.Rat).Quo(new(big.Rat).Sub(radius, rat(s.start.X)), dr)
	if e := b.check(v); e != nil {
		return out, e
	}
	if v.Sign() < 0 || v.Cmp(big.NewRat(1, 1)) > 0 {
		out.Complete = true
		return out, nil
	}

	phase, e := helicalRayPhase(ctx, cs, sn, s.handedness, bits, b)
	if e != nil {
		return out, e
	}
	out.Phase = phase
	floor := func(q *big.Rat) *big.Int {
		n, r := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), r)
		if q.Sign() < 0 && r.Sign() != 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	ceil := func(q *big.Rat) *big.Int { return new(big.Int).Neg(floor(new(big.Rat).Neg(q))) }
	first := ceil(new(big.Rat).Sub(rat(s.domain.UMin), phase[1]))
	last := floor(new(big.Rat).Sub(rat(s.domain.UMax), phase[0]))
	count := new(big.Int).Add(new(big.Int).Sub(last, first), big.NewInt(1))
	if count.Sign() <= 0 {
		out.Complete = true
		return out, nil
	}
	if count.Cmp(big.NewInt(int64(o.MaxContacts))) > 0 {
		return out, ErrConvergence
	}
	out.CandidateTurns = int(count.Int64())
	profileZ := new(big.Rat).Add(rat(s.start.Y), new(big.Rat).Mul(v, new(big.Rat).Sub(rat(s.end.Y), rat(s.start.Y))))
	for n := new(big.Int).Set(first); n.Cmp(last) <= 0; n.Add(n, big.NewInt(1)) {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		hit := HelicalRayLineHit{ProfileParameter: new(big.Rat).Set(v)}
		for end := 0; end < 2; end++ {
			hit.Turn[end] = new(big.Rat).Add(phase[end], new(big.Rat).SetInt(n))
			hit.LineParameter[end] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(rat(s.lead), hit.Turn[end]), profileZ), axialOrigin), axisScale)
			for _, q := range []*big.Rat{hit.Turn[end], hit.LineParameter[end]} {
				if e := b.check(q); e != nil {
					return out, e
				}
			}
		}
		if hit.Turn[0].Cmp(rat(s.domain.UMin)) < 0 || hit.Turn[1].Cmp(rat(s.domain.UMax)) > 0 {
			return out, ErrConvergence
		}
		if hit.LineParameter[0].Cmp(hit.LineParameter[1]) > 0 {
			hit.LineParameter[0], hit.LineParameter[1] = hit.LineParameter[1], hit.LineParameter[0]
		}
		if o.LineRange != nil {
			if hit.LineParameter[1].Cmp(o.LineRange[0]) < 0 || hit.LineParameter[0].Cmp(o.LineRange[1]) > 0 {
				continue
			}
			if hit.LineParameter[0].Cmp(o.LineRange[0]) < 0 || hit.LineParameter[1].Cmp(o.LineRange[1]) > 0 {
				return out, ErrConvergence
			}
		}
		for i := 0; i < 3; i++ {
			for end := 0; end < 2; end++ {
				hit.Point[i][end] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(new(big.Rat).Mul(axis[i], axisScale), hit.LineParameter[end]))
				if e := b.check(hit.Point[i][end]); e != nil {
					return out, e
				}
			}
			if hit.Point[i][0].Cmp(hit.Point[i][1]) > 0 {
				hit.Point[i][0], hit.Point[i][1] = hit.Point[i][1], hit.Point[i][0]
			}
		}
		out.Hits = append(out.Hits, hit)
	}
	out.Complete = true
	return out, nil
}

// helicalAtanSmall uses the alternating series, including its next term as a
// strict remainder bracket. The caller supplies |x|<=1/2.
func helicalAtanSmall(ctx context.Context, x, tolerance *big.Rat, maxTerms int, b *polynomialBudget) (out [2]*big.Rat, err error) {
	negative := x.Sign() < 0
	v := new(big.Rat).Abs(x)
	if v.Cmp(big.NewRat(1, 2)) > 0 {
		return out, ErrInvalid
	}
	if v.Sign() == 0 {
		return [2]*big.Rat{new(big.Rat), new(big.Rat)}, nil
	}
	square := new(big.Rat).Mul(v, v)
	power := new(big.Rat).Set(v)
	sum := new(big.Rat)
	for n := 0; n < maxTerms; n++ {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		term := new(big.Rat).Quo(power, big.NewRat(int64(2*n+1), 1))
		if n%2 == 0 {
			sum.Add(sum, term)
		} else {
			sum.Sub(sum, term)
		}
		power.Mul(power, square)
		next := new(big.Rat).Quo(power, big.NewRat(int64(2*n+3), 1))
		other := new(big.Rat).Set(sum)
		if n%2 == 0 {
			other.Sub(other, next)
		} else {
			other.Add(other, next)
		}
		for _, q := range []*big.Rat{sum, power, next, other} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		if next.Cmp(tolerance) <= 0 {
			out = [2]*big.Rat{new(big.Rat).Set(sum), other}
			if out[0].Cmp(out[1]) > 0 {
				out[0], out[1] = out[1], out[0]
			}
			if negative {
				out = [2]*big.Rat{new(big.Rat).Neg(out[1]), new(big.Rat).Neg(out[0])}
			}
			return out, nil
		}
	}
	return out, ErrConvergence
}
func helicalRayPhase(ctx context.Context, cs, sn *big.Rat, hand HelixHandedness, bits int, b *polynomialBudget) (out [2]*big.Rat, err error) {
	tolerance := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(bits+16)))
	terms := bits*4 + 64
	a, e := helicalAtanSmall(ctx, big.NewRat(1, 5), tolerance, terms, b)
	if e != nil {
		return out, e
	}
	c, e := helicalAtanSmall(ctx, big.NewRat(1, 239), tolerance, terms, b)
	if e != nil {
		return out, e
	}
	pi := [2]*big.Rat{new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a[0]), new(big.Rat).Mul(big.NewRat(4, 1), c[1])), new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a[1]), new(big.Rat).Mul(big.NewRat(4, 1), c[0]))}
	x := new(big.Rat).Quo(new(big.Rat).Abs(sn), new(big.Rat).Abs(cs))
	reciprocal := x.Cmp(big.NewRat(1, 1)) > 0
	if reciprocal {
		x.Inv(x)
	}
	shift := x.Cmp(big.NewRat(1, 2)) > 0
	if shift {
		x.Quo(new(big.Rat).Sub(x, big.NewRat(1, 1)), new(big.Rat).Add(x, big.NewRat(1, 1)))
	}
	angle, e := helicalAtanSmall(ctx, x, tolerance, terms, b)
	if e != nil {
		return out, e
	}
	if shift {
		for i := 0; i < 2; i++ {
			angle[i].Add(angle[i], new(big.Rat).Quo(pi[i], big.NewRat(4, 1)))
		}
	}
	if reciprocal {
		angle = [2]*big.Rat{new(big.Rat).Sub(new(big.Rat).Quo(pi[0], big.NewRat(2, 1)), angle[1]), new(big.Rat).Sub(new(big.Rat).Quo(pi[1], big.NewRat(2, 1)), angle[0])}
	}
	out = [2]*big.Rat{new(big.Rat).Quo(angle[0], new(big.Rat).Mul(big.NewRat(2, 1), pi[1])), new(big.Rat).Quo(angle[1], new(big.Rat).Mul(big.NewRat(2, 1), pi[0]))}

	// The first-quadrant fraction a lies in (0,1/4). Exact quadrant maps
	// are a, 1/2-a, 1/2+a, 1-a; decreasing maps swap enclosure ends.
	if cs.Sign() < 0 && sn.Sign() > 0 {
		out = [2]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 2), out[1]), new(big.Rat).Sub(big.NewRat(1, 2), out[0])}
	} else if cs.Sign() < 0 && sn.Sign() < 0 {
		out = [2]*big.Rat{new(big.Rat).Add(big.NewRat(1, 2), out[0]), new(big.Rat).Add(big.NewRat(1, 2), out[1])}
	} else if cs.Sign() > 0 && sn.Sign() < 0 {
		out = [2]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), out[1]), new(big.Rat).Sub(big.NewRat(1, 1), out[0])}
	}
	if hand == LeftHanded {
		out = [2]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), out[1]), new(big.Rat).Sub(big.NewRat(1, 1), out[0])}
	}
	for _, q := range out {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	if new(big.Rat).Sub(out[1], out[0]).Cmp(new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(bits)))) > 0 {
		return out, ErrConvergence
	}
	return out, nil
}
