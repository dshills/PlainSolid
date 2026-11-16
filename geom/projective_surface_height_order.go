package geom

import (
	"context"
	"math/big"
)

// ProjectiveSurfaceHeightOrder gives a certified enclosure of A minus B
// over their exact closed common projection. When Complete is true, Sign
// is +1 or -1 for strict order everywhere, or 0 for identical lifts.
type ProjectiveSurfaceHeightOrder struct {
	Overlap         ProjectiveSurfaceGraphOverlap
	Difference      ExactParameterInterval
	Sign            int
	Complete        bool
	ExactOperations int
}

// CertifyProjectiveSurfaceHeightOrder checks all original common cells
// with one exact work limit. When the Bernstein hull straddles or touches
// zero without proving identity, it retains the conservative enclosure
// and returns ErrConvergence instead of asserting an order.
func CertifyProjectiveSurfaceHeightOrder(ctx context.Context, a, c *SurfacePatch, axes [2]int,
	o GeneralSurfaceHeightOptions) (r ProjectiveSurfaceHeightOrder, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16_384
	}
	atlas, err := BuildProjectiveSurfaceHeightDifference(ctx, a, c, axes, o)
	r.ExactOperations = atlas.ExactOperations
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{operations: r.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	var lo, hi *big.Rat
	for _, cell := range atlas.Cells {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		for _, h := range cell.Difference.net {
			x := new(big.Rat).Quo(h[2], h[3])
			if err := b.check(x); err != nil {
				return r, err
			}
			if lo == nil || x.Cmp(lo) < 0 {
				lo = x
			}
			if hi == nil || x.Cmp(hi) > 0 {
				hi = x
			}
		}
	}
	if lo == nil {
		return r, ErrConvergence
	}
	r.Overlap, r.Difference = atlas.Overlap, ExactParameterInterval{lo, hi}
	switch {
	case lo.Sign() > 0:
		r.Sign = 1
	case hi.Sign() < 0:
		r.Sign = -1
	case lo.Sign() == 0 && hi.Sign() == 0:
		r.Sign = 0
	default:
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
