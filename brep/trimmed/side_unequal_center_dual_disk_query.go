//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ClassifyRational evaluates each original circle at its own center ordinate,
// retaining exposed original support masks and regularized blind-floor semantics.
func (c UnequalCenterSideOpenDualDiskCut) ClassifyRational(ctx context.Context, point [3]*big.Rat) (OffsetDualDiskPoint, error) {
	return classifyOffsetDualDiskPoint(ctx, c.source, point)
}
func (c UnequalCenterSideOpenDualDiskCut) LocateRational(ctx context.Context, point [3]*big.Rat) (parametric.Location, error) {
	r, e := c.ClassifyRational(ctx, point)
	return r.Location, e
}
func (c UnequalCenterSideOpenDualDiskCut) Locate(ctx context.Context, point geom.Vec3) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	return c.LocateRational(ctx, [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)})
}

// Measures returns rounded analytic volume, area and centroid, not certified
// enclosures. It integrates each original circle's exposed upper graph, split
// at the immutable independently verified join, without sampled geometry.
func (c UnequalCenterSideOpenDualDiskCut) Measures() (SideOpenDualDiskMeasures, error) {
	if c.source == nil || c.join[0] == nil {
		return SideOpenDualDiskMeasures{}, geom.ErrInvalid
	}
	left, right := c.source.discs[0], c.source.discs[1]
	if left.Center.X > right.Center.X {
		left, right = right, left
	}
	relative := new(big.Rat).Sub(c.join[0], ratInset(left.Center.X))
	join, _ := relative.Float64()
	return measureDualDiskUpperSection(c.source, left, right, join)
}
