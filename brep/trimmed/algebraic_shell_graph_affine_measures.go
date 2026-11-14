package trimmed

import (
	"context"
	"plainsolid/geom"
)

// AlgebraicGraphAffineMeasureOptions requests one original whole partition,
// optional first/second moments according to Original.MomentOrder, and optional
// transformed retained boundary area. Transform limits affine mass propagation;
// MaxImageCells/ImageCellOperations/AreaRootBits retain the affine-area limits.
// IncludeBoundary=false (default) incurs no transformed surface-cell work.
// Original.Boundary controls the ONLY source classification/refinement pass.
type AlgebraicGraphAffineMeasureOptions struct {
	Original                                         AlgebraicGraphWholeMassOptions
	Transform                                        geom.ExactCurvePlaneEventOptions
	IncludeBoundary                                  bool
	MaxImageCells, ImageCellOperations, AreaRootBits int
}

// AlgebraicGraphAffineMeasures retains one source partition for the complete
// request. Mass.Original and Boundary.Original (when present) reference that
// same owned evidence; do not add their CellAttempts to estimate work. All image
// values have the specified Placement through their respective result fields.
// Source unknown rim cells remain unknown in every requested measure. No public
// result or mutable caller partition is ever accepted as certification input.
type AlgebraicGraphAffineMeasures struct {
	Mass                 AlgebraicGraphAffineMass
	Boundary             *AlgebraicGraphAffineArea
	OriginalCellAttempts int
}

// BoundAffineWholeMeasures shares the private original whole partition between
// mass integration and cofactor area propagation. Every requested certificate
// must succeed; cancellation or any later area limit returns no partial result.
// Existing independent mass and area convenience methods remain unchanged.
func (q *AlgebraicGraphMaterialQuery) BoundAffineWholeMeasures(ctx context.Context, m geom.Affine, o AlgebraicGraphAffineMeasureOptions) (out AlgebraicGraphAffineMeasures, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphAffineMeasures{}
		}
	}()
	out.Mass, err = q.BoundAffineWholeMassProperties(ctx, m, AlgebraicGraphAffineMassOptions{Original: o.Original, Transform: o.Transform})
	if err != nil {
		return out, err
	}
	out.OriginalCellAttempts = out.Mass.Original.Whole.Boundary.Partition.CellAttempts
	if o.IncludeBoundary {
		area, e := q.boundAffineWholeSurfaceAreaPrepared(ctx, m, AlgebraicGraphAffineAreaOptions{Original: o.Original.Boundary, MaxImageCells: o.MaxImageCells, ImageCellOperations: o.ImageCellOperations, RootBits: o.AreaRootBits}, &out.Mass.Original.Whole)
		if e != nil {
			return out, e
		}
		out.Boundary = &area
	}
	return out, nil
}
