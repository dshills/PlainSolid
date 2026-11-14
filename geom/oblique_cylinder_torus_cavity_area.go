package geom

import (
	"context"
	"math/big"
)

// BoundArea encloses the complete retained cylinder wall/caps and inward
// torus using exact coefficients 2c(upper-lower)*pi, c²*pi per cap and
// 4Rr*pi². The independent proper torus frame changes neither area nor owners.
// One arithmetic ledger covers coefficients and the rigorous Machin pi series.
func (s ObliqueCylinderTorusCavityMaterial) BoundArea(ctx context.Context, o TorusCylinderAreaOptions) (out TorusCylinderArea, err error) {
	defer func() {
		if err != nil {
			out = TorusCylinderArea{}
		}
	}()
	if ctx == nil || s.source.R == nil || o.AreaTolerance == nil || o.AreaTolerance.Sign() <= 0 {
		return out, ErrInvalid
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 64
	}
	if o.PrecisionBits < 16 || o.PrecisionBits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	q := s.source
	for _, v := range []*big.Rat{q.R, q.r, q.c, q.lower, q.upper, o.AreaTolerance} {
		a.check(v)
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
	coefficients := [4]*big.Rat{a.mul(big.NewRat(4, 1), a.mul(q.R, q.r)), a.mul(big.NewRat(2, 1), a.mul(q.c, a.sub(q.upper, q.lower))), a.mul(q.c, q.c), a.mul(q.c, q.c)}
	out.Lower, out.Upper = new(big.Rat), new(big.Rat)
	for i := 0; i < 4; i++ {
		for j := 0; j < 2; j++ {
			factor := pi[j]
			if i == 0 {
				factor = a.mul(factor, factor)
			}
			out.BySupport[i][j] = a.mul(coefficients[i], factor)
		}
		out.Lower = a.add(out.Lower, out.BySupport[i][0])
		out.Upper = a.add(out.Upper, out.BySupport[i][1])
	}
	width := a.sub(out.Upper, out.Lower)
	if a.err != nil {
		return out, a.err
	}
	if width.Cmp(o.AreaTolerance) > 0 {
		return out, ErrConvergence
	}
	out.Owners = TorusCylinderTorus | TorusCylinderWall | TorusCylinderLowerCap | TorusCylinderUpperCap
	out.MaterialNormalSigns = [4]int{-1, 1, 1, 1}
	out.ExactOperations = b.operations
	return out, nil
}
