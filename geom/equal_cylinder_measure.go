package geom

import (
	"context"
	"math/big"
)

// EqualCylinderCommonMeasure describes the intersection of two infinite
// equal-radius cylinders. VolumeSquared is exact; VolumeBounds is outward.
// RawAxialHalfExtentSquared bounds the line parameter t along either raw axis
// measured from their common crossing. CapsCover certifies that optional cap
// windows leave the ENTIRE common solid unchanged; otherwise volume is only
// for the infinite-support common, not the clipped result. CapTouchMask bits
// are UMin,UMax,VMin,VMax. Nil windows mean unbounded cylinders.
type EqualCylinderCommonMeasure struct {
	Complete, CapsCover                                  bool
	CapTouchMask                                         uint8
	SinSquared, VolumeSquared, RawAxialHalfExtentSquared *big.Rat
	VolumeBounds                                         [2]*big.Rat
	ExactOperations                                      int
	radiusSquared, sinSquared                            *big.Rat
}

// MeasureEqualCylinderCommon accepts nonparallel represented raw axes with
// exactly equal squared norm. Section coordinate y is physical distance along
// their common perpendicular. Its parallelogram area is4(r²-y²)/sin(theta),
// and integrating y∈[-r,r] gives16r³/(3sin(theta)). No pi factor occurs.
func MeasureEqualCylinderCommon(ctx context.Context, u, v Vec3, radius float64, capU, capV *[2]*big.Rat, o SurfacePatchEvaluationOptions) (out EqualCylinderCommonMeasure, err error) {
	if ctx == nil || !u.IsFinite() || !v.IsFinite() || u == (Vec3{}) || v == (Vec3{}) || !finite(radius) || radius <= 0 {
		return out, ErrInvalid
	}
	for _, w := range []*[2]*big.Rat{capU, capV} {
		if w != nil && (w[0] == nil || w[1] == nil || w[0].Cmp(w[1]) >= 0) {
			return out, ErrInvalid
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
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
			out.CapsCover = false
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	dot := func(a, c Vec3) *big.Rat {
		q := new(big.Rat)
		for i := 0; i < 3; i++ {
			q.Add(q, new(big.Rat).Mul(r(coordinate(a, i)), r(coordinate(c, i))))
		}
		return q
	}
	N, M, C := dot(u, u), dot(v, v), dot(u, v)
	if N.Cmp(M) != 0 {
		return out, ErrDomain
	}
	D := new(big.Rat).Sub(new(big.Rat).Mul(N, N), new(big.Rat).Mul(C, C))
	if D.Sign() <= 0 {
		return out, ErrDegenerate
	}
	R2 := new(big.Rat).Mul(r(radius), r(radius))
	sin2 := new(big.Rat).Quo(D, new(big.Rat).Mul(N, N))
	volume2 := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(256, 9), new(big.Rat).Mul(R2, new(big.Rat).Mul(R2, R2))), sin2)
	absC := new(big.Rat).Abs(C)
	extent := new(big.Rat).Quo(new(big.Rat).Mul(R2, new(big.Rat).Add(N, absC)), new(big.Rat).Mul(N, new(big.Rat).Sub(N, absC)))
	for _, q := range []*big.Rat{N, M, C, D, R2, sin2, volume2, extent} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	lo, hi, e := rationalSqrtBounds(volume2, 128, b)
	if e != nil {
		return out, e
	}
	out.CapsCover = true
	for i, w := range []*[2]*big.Rat{capU, capV} {
		if w == nil {
			continue
		}
		for j, q := range *w {
			if e := b.check(q); e != nil {
				return out, e
			}
			correct := j == 0 && q.Sign() < 0 || j == 1 && q.Sign() > 0
			cmp := new(big.Rat).Mul(q, q).Cmp(extent)
			if !correct || cmp < 0 {
				out.CapsCover = false
			}
			if correct && cmp == 0 {
				out.CapTouchMask |= 1 << uint(2*i+j)
			}
		}
	}
	out.SinSquared = sin2
	out.VolumeSquared = volume2
	out.RawAxialHalfExtentSquared = extent
	out.VolumeBounds = [2]*big.Rat{lo, hi}
	out.radiusSquared = new(big.Rat).Set(R2)
	out.sinSquared = new(big.Rat).Set(sin2)
	out.Complete = true
	return out, nil
}

// SectionAreaSquared returns the exact squared area at rational physical
// common-perpendicular coordinate y. Outside or at support extrema gives zero.
func (p EqualCylinderCommonMeasure) SectionAreaSquared(ctx context.Context, y *big.Rat, o SurfacePatchEvaluationOptions) (*big.Rat, error) {
	if ctx == nil || y == nil || p.radiusSquared == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	if e := b.check(y); e != nil {
		return nil, e
	}
	q := new(big.Rat).Sub(p.radiusSquared, new(big.Rat).Mul(y, y))
	if q.Sign() <= 0 {
		return new(big.Rat), nil
	}
	area2 := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(16, 1), new(big.Rat).Mul(q, q)), p.sinSquared)
	if e := b.check(area2); e != nil {
		return nil, e
	}
	return area2, nil
}
