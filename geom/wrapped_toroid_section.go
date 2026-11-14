package geom

import (
	"context"
	"math/big"
)

// CircularToroidAxialSection describes the exact material in the original
// plane Z=Height. The two radii are MajorRadius±sqrt(RadialOffsetSquared).
// Area is 4π·MajorRadius·sqrt(RadialOffsetSquared). At an axial extremum
// the material is a single boundary circle with zero section area.
type CircularToroidAxialSection struct {
	Center                                                          [3]*big.Rat
	Height, MajorRadius, RadialOffsetSquared, AreaPiSqrtCoefficient *big.Rat
	Present, SingleCircle                                           bool
	ExactOperations                                                 int
}

// SectionWrappedCircularToroidRational certifies the original ring at an exact
// rational model height. It supports elliptic axial scaling of circular
// meridians and returns exact circle radii as radicals without rounding.
func SectionWrappedCircularToroidRational(ctx context.Context, w *WrappedSurfacePatch, height *big.Rat, o SurfacePatchOptions) (out CircularToroidAxialSection, err error) {
	defer func() {
		if err != nil {
			out = CircularToroidAxialSection{}
		}
	}()
	zero := new(big.Rat)
	opt, e := validateRationalPoint(ctx, [3]*big.Rat{zero, zero, height}, o)
	if e != nil {
		return out, e
	}
	p, e := CertifyWrappedToroidEmbedding(ctx, w, o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{operations: p.ExactOperations, maxOperations: opt.MaxOperations, maxBits: opt.MaxCoefficientBits}
	for _, stride := range []int{1, 9} {
		for i := 0; i < 8; i += 2 {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			a, c, d := w.patch.net[i*stride][3], w.patch.net[(i+1)*stride][3], w.patch.net[(i+2)*stride][3]
			left, right := new(big.Rat).Mul(a, d), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(c, c))
			for _, x := range []*big.Rat{left, right} {
				if e := b.check(x); e != nil {
					return out, e
				}
			}
			if left.Cmp(right) != 0 {
				return out, ErrConvergence
			}
		}
	}
	h := new(big.Rat).Sub(height, p.Center[2])
	fraction := new(big.Rat).Quo(h, p.AxialMinorRadius)
	remain := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(fraction, fraction))
	for _, v := range []*big.Rat{h, fraction, remain} {
		if e := b.check(v); e != nil {
			return out, e
		}
	}
	out.Center = [3]*big.Rat{new(big.Rat).Set(p.Center[0]), new(big.Rat).Set(p.Center[1]), new(big.Rat).Set(height)}
	out.Height = new(big.Rat).Set(height)
	if remain.Sign() < 0 {
		out.ExactOperations = b.operations
		return out, nil
	}
	out.Present, out.SingleCircle = true, remain.Sign() == 0
	out.MajorRadius = new(big.Rat).Set(p.MajorRadius)
	out.RadialOffsetSquared = new(big.Rat).Mul(new(big.Rat).Mul(p.RadialMinorRadius, p.RadialMinorRadius), remain)
	out.AreaPiSqrtCoefficient = new(big.Rat).Mul(big.NewRat(4, 1), p.MajorRadius)
	for _, v := range []*big.Rat{out.RadialOffsetSquared, out.AreaPiSqrtCoefficient} {
		if e := b.check(v); e != nil {
			return out, e
		}
	}
	out.ExactOperations = b.operations
	return out, ctx.Err()
}
