package trimmed

import (
	"context"

	"plainsolid/geom"
)

// ExactComponentVolumes integrates the original physical height difference
// over the retained Cut regions. Each source-bound component receives an
// exact rational volume. Each union-knot height-difference denominator must
// be positive and constant.
func (r *FivePolynomialNodeCut) ExactComponentVolumes(ctx context.Context, o PolynomialMixedCutVolumeOptions) (geom.SplineGraphFivePolynomialNodeVolumes, error) {
	zero := geom.SplineGraphFivePolynomialNodeVolumes{}
	if ctx == nil || r == nil || r.sources[0] == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	roofs, e := exactPolynomialCutRoofs(r.sources, o.Patch)
	if e != nil {
		return zero, e
	}
	report, e := geom.IntegrateSplineGraphFivePolynomialNodeVolumes(ctx, roofs[0], roofs[1], r.axis, o.Proof)
	if e != nil {
		return zero, e
	}
	a := report.Arrangement
	if !isNativeFivePolynomialNodeArrangement(a) {
		return zero, geom.ErrConvergence
	}
	for i, gap := range a.DerivativeGap {
		if gap.Cmp(r.derivatives[i]) != 0 {
			return zero, geom.ErrConvergence
		}
	}
	uv, xyz, e := a.RepresentedVertices()
	if e != nil {
		return zero, e
	}
	if e = r.material.bindVolumeRegions(r.sources, report.Regions, report.Volumes, a.Regions, a.Edges, a.Vertices, a.Nodes, uv, xyz); e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return report, nil
}
