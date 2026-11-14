package trimmed

import (
	"context"
	"math/big"
)

// fdTwoPositiveRadicalsSign gives sign(A-B*sqrt(S)+C*sqrt(T)) for
// B,C,S,T>0. First establish the sign of L=A+C*sqrt(T). If L<=0,
// L-B*sqrt(S)<0. Otherwise squaring preserves order and the difference
// L²-B²*S has just one radical. No numerical enclosure or unchecked
// squaring is used. Callers prove positivity from immutable bore geometry.
func fdTwoPositiveRadicalsSign(a, b, s, c, t *big.Rat) int {
	if fdRadicalSign(a, new(big.Rat).Neg(c), t) <= 0 {
		return -1
	}
	constant := fdSub(fdAdd(fdSquare(a), fdMul(fdSquare(c), t)), fdMul(fdSquare(b), s))
	coefficient := new(big.Rat).Neg(fdMul(big.NewRat(2, 1), fdMul(a, c)))
	return fdRadicalSign(constant, coefficient, t)
}

// ClassifyIndependentBlindRimsRational extends the ordered coaxial query to
// different original bore centers. It requires opposite blind entries with a
// strictly positive floor gap, a query strictly within that gap and radially
// outside both disks. Both wall/disk minima are attained at their original
// rims. Their squared distances are compared by an exact guarded two-radical
// sign; all unrelated original faces still require strict lower separation.
// Uses one support query and at most MaxPairs pairs. No partial result on error.
func (q FilletedFrustumMaterial) ClassifyIndependentBlindRimsRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumOpposingRimOptions) (FilletedFrustumOrderedRimFaces, error) {
	return q.classifyBlindRimFaces(ctx, p, o, true)
}

// ClassifyIndependentBlindRimsRational preserves exact parameter-free rim
// witnesses, owner IDs and ordering under a proper rational rigid occurrence.
// Both WorldRims are attained candidates; only Original.Tied is nearest.
func (q FilletedFrustumOccurrence) ClassifyIndependentBlindRimsRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumOpposingRimOptions) (FilletedFrustumOccurrenceOrderedRimFaces, error) {
	return q.classifyBlindRimFaces(ctx, p, o, true)
}
