//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphConvexRefineOptions bounds two independent certificates.
// Convex.Exact includes a 256-operation reservation for eligibility and exact
// determinant scaling. Whole retains its existing per-cell, surface-cell and
// total partition limits; it is not charged against Convex.Exact. Unsupported
// original domain geometry skips Whole, but an attempted Whole failure is atomic.
type AlgebraicGraphConvexRefineOptions struct {
	Convex AlgebraicGraphConvexOverlapOptions
	Whole  AlgebraicGraphWindowAreaOptions
}

// AlgebraicGraphRefinedConvexVolume retains the exact affine-image polygon and
// intersects its conservative volume with a determinant-scaled original whole
// partition when available. OriginalWhole remains in ORIGINAL world coordinates;
// none of its area, window or leaf coordinates describe the affine image.
// Nil OriginalWhole means the original domain does not support the rectangular
// whole-partition path, or its projected overlap has zero area. Only the wide
// convex bound is then present.
// Even with a partition, unresolved rim contributions remain conservative.
type AlgebraicGraphRefinedConvexVolume struct {
	AlgebraicGraphConvexOverlap
	OriginalWhole *AlgebraicGraphWholeMeasures
}

// graphWholePartitionEligible is only a structural domain gate. It does not
// prove source membership or replace exactCommonGraphRectangle's arithmetic.
func (q *AlgebraicGraphMaterialQuery) graphWholePartitionEligible() bool {
	for i, g := range q.graphs {
		if g == nil || !exactFullGraphRectangle(q.faces[i], g.Domain()) {
			return false
		}
		inverse, e := g.InverseCoefficients()
		if e != nil {
			return false
		}
		seen := [2]bool{}
		for _, row := range inverse {
			axis := -1
			for j := 0; j < 2; j++ {
				if row[j].Sign() != 0 {
					if axis != -1 {
						return false
					}
					axis = j
				}
			}
			if axis < 0 || seen[axis] {
				return false
			}
			seen[axis] = true
		}
	}
	return true
}

// BoundRefinedAffineConvexWholeVolume tightens the affine-image volume using
// one original whole-measure partition, without additional classification beyond
// that partition or reuse of mutable caller evidence. The exact affine determinant scales volume;
// area is not determinant-scaled or otherwise claimed in the image. Intersection
// with the independent polygon bound preserves exclusion outside the true convex
// overlap. The old convex methods and their inexpensive defaults are unchanged.
func (q *AlgebraicGraphMaterialQuery) BoundRefinedAffineConvexWholeVolume(ctx context.Context, m geom.Affine, axes [2]int, o AlgebraicGraphConvexRefineOptions) (out AlgebraicGraphRefinedConvexVolume, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphRefinedConvexVolume{AlgebraicGraphConvexOverlap: AlgebraicGraphConvexOverlap{ExactOperations: out.ExactOperations}}
		}
	}()
	if ctx == nil || q == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o.Convex.Exact, err = graphMaterialLimits(o.Convex.Exact)
	if err != nil {
		return out, err
	}
	const overhead = 256
	if o.Convex.Exact.MaxOperations <= overhead {
		return out, geom.ErrConvergence
	}
	wide := o.Convex
	wide.Exact.MaxOperations -= overhead
	out.AlgebraicGraphConvexOverlap, err = q.BoundAffineConvexWholeVolume(ctx, m, axes, wide)
	out.ExactOperations += overhead
	if err != nil {
		return out, err
	}
	if !q.graphWholePartitionEligible() || out.ProjectedArea.Sign() == 0 {
		return out, nil
	}
	whole, e := q.BoundWholeMeasures(ctx, o.Whole)
	if e != nil {
		return out, e
	}
	det := new(big.Rat).Sub(new(big.Rat).Mul(ratInset(m[axes[0]][q.axes[0]]), ratInset(m[axes[1]][q.axes[1]])), new(big.Rat).Mul(ratInset(m[axes[0]][q.axes[1]]), ratInset(m[axes[1]][q.axes[0]])))
	det.Mul(det, ratInset(m[out.ResidualAxis][q.residual]))
	det.Abs(det)
	if !graphPolynomialBits(det, o.Convex.Exact.MaxCoefficientBits) {
		return out, geom.ErrConvergence
	}
	scaled := [2]*big.Rat{}
	for i := 0; i < 2; i++ {
		scaled[i] = new(big.Rat).Mul(det, whole.Volume[i])
		if !graphPolynomialBits(scaled[i], o.Convex.Exact.MaxCoefficientBits) {
			return out, geom.ErrConvergence
		}
	}
	if scaled[0].Cmp(out.Volume[0]) > 0 {
		out.Volume[0] = scaled[0]
	}
	if scaled[1].Cmp(out.Volume[1]) < 0 {
		out.Volume[1] = scaled[1]
	}
	if out.Volume[0].Cmp(out.Volume[1]) > 0 {
		return out, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.OriginalWhole = &whole
	return out, nil
}
