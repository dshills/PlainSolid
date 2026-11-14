package geom

import (
	"context"
	"math/big"
)

// EncompassingTorusCylinderMaterial proves the whole torus strictly inside the
// finite cylinder: c>R+r and lower<-r<r<upper. Common is the original torus,
// Difference is empty, and Union is the original cylinder. No transverse rim
// formula is exposed and contact at either radial or axial support rejects.
type EncompassingTorusCylinderMaterial struct {
	source     TorusCylinderMaterial
	clearances [3]*big.Rat
}

func NewEncompassingTorusCylinderMaterial(R, r, c, lower, upper float64, op TorusCylinderOperation) (EncompassingTorusCylinderMaterial, error) {
	var zero EncompassingTorusCylinderMaterial
	for _, x := range []float64{R, r, c, lower, upper} {
		if !finite(x) {
			return zero, ErrInvalid
		}
	}
	if R <= r || r <= 0 || c <= 0 || (op != TorusCylinderCommon && op != TorusCylinderDifference && op != TorusCylinderUnion) {
		return zero, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	s := TorusCylinderMaterial{q(R), q(r), q(c), q(lower), q(upper), op}
	gaps := [3]*big.Rat{new(big.Rat).Sub(s.c, new(big.Rat).Add(s.R, s.r)), new(big.Rat).Sub(new(big.Rat).Neg(s.r), s.lower), new(big.Rat).Sub(s.upper, s.r)}
	for _, gap := range gaps {
		if gap.Sign() <= 0 {
			return zero, ErrDomain
		}
	}
	return EncompassingTorusCylinderMaterial{s, gaps}, nil
}

// Clearances returns owned exact radial, lower-cap and upper-cap distances.
func (s EncompassingTorusCylinderMaterial) Clearances() ([3]*big.Rat, error) {
	if s.clearances[0] == nil {
		return [3]*big.Rat{}, ErrInvalid
	}
	return [3]*big.Rat{new(big.Rat).Set(s.clearances[0]), new(big.Rat).Set(s.clearances[1]), new(big.Rat).Set(s.clearances[2])}, nil
}
func (s EncompassingTorusCylinderMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
	return s.source.ClassifyRational(ctx, p, o)
}

// ClipRationalLine retains the original exact quartic/quadratic event engine's
// finite t, isolated/coincident contacts and explicit per-proof work bounds.
func (s EncompassingTorusCylinderMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
	return s.source.ClipRationalLine(ctx, p, d, min, max, o)
}
func (s EncompassingTorusCylinderMaterial) BoundaryNormalsRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (TorusCylinderNormals, error) {
	return s.source.BoundaryNormalsRational(ctx, p, o)
}
