package geom

import (
	"context"
	"math/big"
)

// CompareHelicalRetainedFullTurn proves the closed phase window [0,1] over a
// radial band. The duplicate seam is deliberate: at phase one, source integer
// turns are decreased by one relative to phase zero, preserving physical U,
// finite source cap membership and axial coordinates exactly. Ancestry Turn
// labels are relative to the reported phase, not an absolute winding ID.
// Both seam copies and every intervening event cell are proved independently.
func CompareHelicalRetainedFullTurn(ctx context.Context, a, b SeparatedHelicalStarts, shift *big.Rat, radii, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (HelicalJointWindowComparison, error) {
	return CompareHelicalRetainedJointWindow(ctx, a, b, shift, radii, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, bandA, bandB, o)
}
