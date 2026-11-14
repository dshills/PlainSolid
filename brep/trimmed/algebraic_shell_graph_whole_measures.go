package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphWholeMeasures covers the complete prepared native common,
// not an arbitrary caller window. Boundary.Partition is the single reusable
// material/empty/unresolved partition used for both volume and area. Incomplete
// rim classification remains explicit; successful output never implies exact
// whole volume or area. Volume is an owned copy of that partition's enclosure.
type AlgebraicGraphWholeMeasures struct {
	SupportWindow    AlgebraicGraphProjectionWindow
	Volume           [2]*big.Rat
	Boundary         AlgebraicGraphWindowArea
	DomainOperations int
}

// BoundWholeMeasures derives the exact common projected rectangle from both
// complete original trim rectangles and signed/scaled axis-permutation affine
// projections. The immutable prepared common proves all retained fragments are
// confined to these original domains. Outside their intersection is therefore
// empty. Unsupported oblique chart domains fail closed, rather than using a
// bounding hull as material. Classification runs once; both support areas and
// volume consume its shared partition. Existing per-cell area/refinement limits
// apply unchanged, plus a256-operation exact domain preflight.
func (q *AlgebraicGraphMaterialQuery) BoundWholeMeasures(ctx context.Context, o AlgebraicGraphWindowAreaOptions) (out AlgebraicGraphWholeMeasures, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphWholeMeasures{DomainOperations: out.DomainOperations}
		}
	}()
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	exact, e := graphMaterialLimits(o.Refinement.CellOptions.Exact)
	if e != nil {
		return out, e
	}
	if exact.MaxOperations < 256 {
		return out, geom.ErrConvergence
	}
	out.DomainOperations = 256
	out.SupportWindow, err = q.exactCommonGraphRectangle(exact.MaxCoefficientBits)
	if err != nil {
		return out, err
	}
	out.Boundary, err = q.BoundWindowSurfaceArea(ctx, out.SupportWindow, o)
	if err != nil {
		return out, err
	}
	if !out.Boundary.Partition.HasBounds {
		return out, geom.ErrConvergence
	}
	for i := 0; i < 2; i++ {
		out.Volume[i] = new(big.Rat).Set(out.Boundary.Partition.Volume[i])
	}
	return out, nil
}
