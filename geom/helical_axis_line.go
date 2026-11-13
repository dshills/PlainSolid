package geom

import (
	"context"
	"math/big"
)

// HelicalAxisLineOptions bounds exact repeated-turn enumeration. LineRange,
// when nonnil, is a closed interval in the caller's original line parameter.
// Defaults are 1024 contacts, one million operations and 16384 coefficient bits.
type HelicalAxisLineOptions struct {
	MaxContacts, MaxOperations, MaxCoefficientBits int
	LineRange                                      *[2]*big.Rat
}

// HelicalAxisLineHit retains original turns/profile coordinates and original
// line units, with exact original XYZ. All rational values are caller-owned.
type HelicalAxisLineHit struct {
	Turn, ProfileParameter, LineParameter *big.Rat
	Point                                 [3]*big.Rat
}

// HelicalAxisLineContacts is complete only after every possible finite-domain
// turn has been enumerated and clipped. An empty complete report proves no
// contacts. Hits are ordered by increasing original turn, not line parameter.
// An error leaves Complete false and discards partial contacts.
type HelicalAxisLineContacts struct {
	Complete                        bool
	Hits                            []HelicalAxisLineHit
	CandidateTurns, ExactOperations int
}

// IntersectCardinalAxisLine intersects the entire finite surface with
// origin+lambda*axisScale*Frame.ZAxis. phase must be an exact quarter turn and
// match the line's positive radial ray; otherwise the query is unsupported.
// An exact basis determinant and radial decomposition establish that every
// possible contact has U=phase+n. Positive radius excludes the axis itself.
// Constant-radius coincident meridians return ErrConvergence; they are never
// misreported as isolated hits. No numerical trigonometry is used.
func (s HelicalProfileSurface) IntersectCardinalAxisLine(ctx context.Context, phase *big.Rat, origin [3]*big.Rat, axisScale *big.Rat, o HelicalAxisLineOptions) (out HelicalAxisLineContacts, err error) {
	if ctx == nil || phase == nil || axisScale == nil || axisScale.Sign() == 0 {
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
	inputs := append([]*big.Rat{phase, axisScale}, origin[:]...)
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
	quarter := new(big.Rat).Mul(phase, big.NewRat(4, 1))
	if e := b.check(quarter); e != nil {
		return out, e
	}
	if !quarter.IsInt() {
		return out, ErrConvergence
	}
	phase = new(big.Rat).SetFrac(new(big.Int).Mod(quarter.Num(), big.NewInt(4)), big.NewInt(4))
	quadrant := new(big.Int).Mod(new(big.Int).Mul(quarter.Num(), big.NewInt(int64(s.handedness))), big.NewInt(4)).Int64()
	cs, sn := [4]int64{1, 0, -1, 0}[quadrant], [4]int64{0, 1, 0, -1}[quadrant]
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x := [3]float64{s.frame.x.X, s.frame.x.Y, s.frame.x.Z}
	y := [3]float64{s.frame.y.X, s.frame.y.Y, s.frame.y.Z}
	z := [3]float64{s.frame.z.X, s.frame.z.Y, s.frame.z.Z}
	offset := [3]float64{s.frame.origin.X, s.frame.origin.Y, s.frame.origin.Z}
	var radial, tangent, axis, delta [3]*big.Rat
	for i := 0; i < 3; i++ {
		radial[i] = new(big.Rat).Add(new(big.Rat).Mul(rat(x[i]), big.NewRat(cs, 1)), new(big.Rat).Mul(rat(y[i]), big.NewRat(sn, 1)))
		tangent[i] = new(big.Rat).Sub(new(big.Rat).Mul(rat(y[i]), big.NewRat(cs, 1)), new(big.Rat).Mul(rat(x[i]), big.NewRat(sn, 1)))
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
	floor := func(q *big.Rat) *big.Int {
		n, r := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), r)
		if q.Sign() < 0 && r.Sign() != 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	low := new(big.Rat).Sub(rat(s.domain.UMin), phase)
	high := new(big.Rat).Sub(rat(s.domain.UMax), phase)
	first := new(big.Int).Neg(floor(new(big.Rat).Neg(low)))
	last := floor(high)
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
		u := new(big.Rat).Add(phase, new(big.Rat).SetInt(n))
		lambda := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(rat(s.lead), u), profileZ), axialOrigin), axisScale)
		for _, q := range []*big.Rat{u, lambda} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		if o.LineRange != nil && (lambda.Cmp(o.LineRange[0]) < 0 || lambda.Cmp(o.LineRange[1]) > 0) {
			continue
		}
		hit := HelicalAxisLineHit{Turn: u, ProfileParameter: new(big.Rat).Set(v), LineParameter: lambda}
		for i := 0; i < 3; i++ {
			hit.Point[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(new(big.Rat).Mul(axis[i], axisScale), lambda))
			if e := b.check(hit.Point[i]); e != nil {
				return out, e
			}
		}
		out.Hits = append(out.Hits, hit)
	}
	out.Complete = true
	return out, nil
}
