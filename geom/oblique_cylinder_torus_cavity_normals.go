package geom

import (
	"context"
	"math/big"
)

// ObliqueCavityRationalNormals gives exact outward material UNIT normals of
// every exposed original support. Slots are torus, wall, lower cap, upper cap.
// TorusQuarterSigns are exact local x,y,rho-R,z signs for chart incidence.
// Absent normal slots are nil; simultaneous owners are never averaged.
type ObliqueCavityRationalNormals struct {
	Owners            BoreBoundaryMask
	Units             [4][3]*big.Rat
	TorusQuarterSigns [4]int
	ExactOperations   int
}

// BoundaryUnitNormalsRational proves membership first. At a rational torus
// boundary point, S=x²+y²+z²+R²-r²=2R*rho>0, so rho is rational. Thus its unit
// normal is rational even when generic normalization would introduce radicals.
// The inward cavity normal is -((rho-R)x/(r*rho),(rho-R)y/(r*rho),z/r), mapped
// through the original proper frame. Every returned squared norm is checked
// equal to 1. Classification, inverse, and all units share one exact ledger.
func (s ObliqueCylinderTorusCavityMaterial) BoundaryUnitNormalsRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (out ObliqueCavityRationalNormals, err error) {
	defer func() {
		if err != nil {
			out = ObliqueCavityRationalNormals{}
		}
	}()
	if ctx == nil || s.source.R == nil {
		return out, ErrInvalid
	}
	for _, q := range p {
		if q == nil {
			return out, ErrInvalid
		}
	}
	options, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, q := range p {
		a.check(q)
	}
	if a.err != nil {
		return out, a.err
	}
	location, mask, e := s.classifyWithBudget(ctx, p, b)
	if e != nil {
		return out, e
	}
	if location != BoreBoundary {
		return out, ErrDomain
	}
	out.Owners = mask
	if mask&TorusCylinderTorus != 0 {
		var local [3]*big.Rat
		for i := 0; i < 3; i++ {
			local[i] = new(big.Rat)
			for j := 0; j < 3; j++ {
				local[i] = a.add(local[i], a.mul(s.pose[j][i], a.sub(p[j], s.pose[j][3])))
			}
		}
		S := a.sub(a.mul(s.source.R, s.source.R), a.mul(s.source.r, s.source.r))
		for _, x := range local {
			S = a.add(S, a.mul(x, x))
		}
		rho := a.div(S, a.mul(big.NewRat(2, 1), s.source.R))
		if a.err != nil {
			return out, a.err
		}
		if rho.Sign() <= 0 {
			return out, ErrDomain
		}
		offset := a.sub(rho, s.source.R)
		factor := a.div(offset, a.mul(s.source.r, rho))
		normal := [3]*big.Rat{a.sub(new(big.Rat), a.mul(local[0], factor)), a.sub(new(big.Rat), a.mul(local[1], factor)), a.sub(new(big.Rat), a.div(local[2], s.source.r))}
		for i := 0; i < 3; i++ {
			out.Units[0][i] = new(big.Rat)
			for j := 0; j < 3; j++ {
				out.Units[0][i] = a.add(out.Units[0][i], a.mul(s.pose[i][j], normal[j]))
			}
		}
		out.TorusQuarterSigns = [4]int{local[0].Sign(), local[1].Sign(), offset.Sign(), local[2].Sign()}
	}
	if mask&TorusCylinderWall != 0 {
		out.Units[1] = [3]*big.Rat{a.div(p[0], s.source.c), a.div(p[1], s.source.c), new(big.Rat)}
	}
	if mask&TorusCylinderLowerCap != 0 {
		out.Units[2] = [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(-1, 1)}
	}
	if mask&TorusCylinderUpperCap != 0 {
		out.Units[3] = [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
	}
	for i, unit := range out.Units {
		if mask&BoreBoundaryMask(1<<i) == 0 {
			continue
		}
		squared := new(big.Rat)
		for _, x := range unit {
			squared = a.add(squared, a.mul(x, x))
		}
		if a.err != nil {
			return out, a.err
		}
		if squared.Cmp(big.NewRat(1, 1)) != 0 {
			return out, ErrDomain
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.ExactOperations = b.operations
	return out, nil
}
