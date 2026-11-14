package geom

import (
	"context"
	"math/big"
)

// SeparatedTorusCylinderMaterial is the strict coaxial hole-separated family:
// 0<c<R-r and lower<-r<r<upper. It exposes only the full-support polynomial
// queries; the transverse rim/integral APIs deliberately remain unavailable.
// Common is empty, Difference is the original full torus, and Union has two
// disjoint components. Contact c=R-r is explicitly outside this proof domain.
type SeparatedTorusCylinderMaterial struct {
	source    TorusCylinderMaterial
	clearance *big.Rat
}

func NewSeparatedTorusCylinderMaterial(R, r, c, lower, upper float64, op TorusCylinderOperation) (SeparatedTorusCylinderMaterial, error) {
	var zero SeparatedTorusCylinderMaterial
	for _, v := range []float64{R, r, c, lower, upper} {
		if !finite(v) {
			return zero, ErrInvalid
		}
	}
	if R <= r || r <= 0 || c <= 0 || (op != TorusCylinderCommon && op != TorusCylinderDifference && op != TorusCylinderUnion) {
		return zero, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	s := TorusCylinderMaterial{q(R), q(r), q(c), q(lower), q(upper), op}
	gap := new(big.Rat).Sub(new(big.Rat).Sub(s.R, s.r), s.c)
	if gap.Sign() <= 0 || s.lower.Cmp(new(big.Rat).Neg(s.r)) >= 0 || s.upper.Cmp(s.r) <= 0 {
		return zero, ErrDomain
	}
	return SeparatedTorusCylinderMaterial{s, gap}, nil
}
func (s SeparatedTorusCylinderMaterial) RadialClearance() (*big.Rat, error) {
	if s.clearance == nil {
		return nil, ErrInvalid
	}
	return new(big.Rat).Set(s.clearance), nil
}
func (s SeparatedTorusCylinderMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
	return s.source.ClassifyRational(ctx, p, o)
}

// ClipRationalLine retains the original polynomial engine's exact t, finite
// clipping, isolated and coincident contacts, and arithmetic/root work limits.
// No transverse rim assertion is used: c<R-r proves all torus points are
// outside the cylinder and the two closed material supports are disjoint.
func (s SeparatedTorusCylinderMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
	return s.source.ClipRationalLine(ctx, p, d, min, max, o)
}
func (s SeparatedTorusCylinderMaterial) BoundaryNormalsRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (TorusCylinderNormals, error) {
	return s.source.BoundaryNormalsRational(ctx, p, o)
}
