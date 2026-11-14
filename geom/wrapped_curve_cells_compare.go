package geom

import (
	"context"
	"math/big"
)

// CompareModel proves complete same-parameter model equality of two immutable
// wrapped compositions, optionally using B(1-t) for reversed traversal. The
// exact union of both algebraic endpoint partitions is used; matching endpoint
// representatives alone cannot establish equality. Original lifted UV and
// period ancestry remain on each owner's Piece descriptors and are not equated.
//
// Equivalent=false disproves this parameter correspondence, not equality of
// unparameterized images. No injectivity, physical branch count, native face or
// shell claim follows. Options retain the collection comparator's per-stage
// arithmetic limits and bounded total cell/cut counts. Any error is atomic.
func (a *WrappedCurveCells) CompareModel(ctx context.Context, b *WrappedCurveCells, reverseB bool, o ComposedCollectionAffineParameterOptions) (ComposedCollectionAffineParameterComparison, error) {
	if ctx == nil || a == nil || b == nil || len(a.pieces) == 0 || len(b.pieces) == 0 {
		return ComposedCollectionAffineParameterComparison{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return ComposedCollectionAffineParameterComparison{}, e
	}
	if o.MaxCells == 0 {
		o.MaxCells = 64
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 {
		return ComposedCollectionAffineParameterComparison{}, ErrInvalid
	}
	if len(a.pieces) > o.MaxCells || len(b.pieces) > o.MaxCells {
		return ComposedCollectionAffineParameterComparison{}, ErrConvergence
	}
	// Only constructor-owned immutable cells enter this adapter. Do not consume
	// caller-asserted completeness flags or a user-selected subset of pieces.
	collections := [2]SurfaceCurveCompositionCells{{Complete: true}, {Complete: true}}
	for owner, source := range []*WrappedCurveCells{a, b} {
		for _, piece := range source.pieces {
			collections[owner].Cells = append(collections[owner].Cells, piece.Model)
		}
	}
	offset, scale := new(big.Rat), big.NewRat(1, 1)
	if reverseB {
		offset.SetInt64(1)
		scale.SetInt64(-1)
	}
	return CompareComposedCollectionsAffineParameter(ctx, collections[0], collections[1], offset, scale, o)
}
