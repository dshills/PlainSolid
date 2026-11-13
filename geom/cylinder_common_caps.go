package geom

import (
	"context"
	"math/big"
)

// CylinderAxialExtent describes the exact positive raw-axis half extent.
// Its square is SquaredBase+sqrt(SquaredRadicand); Bounds enclose the extent.
// Raw-axis parameters are measured from the common centerline crossing.
type CylinderAxialExtent struct {
	SquaredBase, SquaredRadicand *big.Rat
	Bounds                       [2]*big.Rat
}

// CylinderCommonCaps certifies whether finite axial windows retain the entire
// infinite-support common of two cylinders with intersecting centerlines.
// TouchMask bits are UMin, UMax, VMin, VMax. A false Covers does not imply empty.
type CylinderCommonCaps struct {
	Complete, Covers bool
	TouchMask        uint8
	Extents          [2]CylinderAxialExtent
	ExactOperations  int
}

// ClassifyCylinderCommonCaps allows unequal positive radii and arbitrary
// nonparallel represented raw axes. Nil cap windows are unbounded. This is a
// support-extremum proof, not a clipped-solid topology or volume computation.
func ClassifyCylinderCommonCaps(ctx context.Context, u, v Vec3, ru, rv float64, capU, capV *[2]*big.Rat, o SurfacePatchEvaluationOptions) (out CylinderCommonCaps, err error) {
	if ctx == nil || !u.IsFinite() || !v.IsFinite() || u == (Vec3{}) || v == (Vec3{}) || !finite(ru) || !finite(rv) || ru <= 0 || rv <= 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, w := range []*[2]*big.Rat{capU, capV} {
		if w != nil && (w[0] == nil || w[1] == nil || w[0].Cmp(w[1]) >= 0) {
			return out, ErrInvalid
		}
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Covers = false
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	mul := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Mul(a, c) }
	dot := func(a, c Vec3) *big.Rat {
		q := new(big.Rat)
		for i := 0; i < 3; i++ {
			q.Add(q, mul(r(coordinate(a, i)), r(coordinate(c, i))))
		}
		return q
	}
	N, M, C := dot(u, u), dot(v, v), dot(u, v)
	NM := mul(N, M)
	C2 := mul(C, C)
	D := new(big.Rat).Sub(NM, C2)
	if D.Sign() <= 0 {
		return out, ErrDegenerate
	}
	R, S := r(ru), r(rv)
	out.Covers = true
	for i, w := range []*[2]*big.Rat{capU, capV} {
		n, ra, rb := N, R, S
		if i == 1 {
			n, ra, rb = M, S, R
		}
		den := mul(n, D)
		base := new(big.Rat).Quo(new(big.Rat).Add(mul(mul(rb, rb), NM), mul(mul(ra, ra), C2)), den)
		radical := new(big.Rat).Quo(mul(big.NewRat(4, 1), mul(mul(mul(ra, ra), mul(rb, rb)), mul(C2, NM))), mul(den, den))
		for _, q := range []*big.Rat{N, M, C, D, base, radical} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		lo, hi, e := rationalSqrtBounds(radical, 128, b)
		if e != nil {
			return out, e
		}
		low, _, e := rationalSqrtBounds(new(big.Rat).Add(base, lo), 128, b)
		if e != nil {
			return out, e
		}
		_, high, e := rationalSqrtBounds(new(big.Rat).Add(base, hi), 128, b)
		if e != nil {
			return out, e
		}
		out.Extents[i] = CylinderAxialExtent{base, radical, [2]*big.Rat{low, high}}
		if w != nil {
			for j, q := range *w {
				if e := b.check(q); e != nil {
					return out, e
				}
				delta := new(big.Rat).Sub(mul(q, q), base)
				cmp := -1
				if delta.Sign() >= 0 {
					sq := mul(delta, delta)
					if e := b.check(sq); e != nil {
						return out, e
					}
					cmp = sq.Cmp(radical)
				}
				correct := (j == 0 && q.Sign() < 0) || (j == 1 && q.Sign() > 0)
				if !correct || cmp < 0 {
					out.Covers = false
				}
				if correct && cmp == 0 {
					out.TouchMask |= 1 << uint(2*i+j)
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Complete = true
	return out, nil
}
