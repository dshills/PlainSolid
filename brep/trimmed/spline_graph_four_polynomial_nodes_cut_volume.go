//go:build research

package trimmed

import (
	"context"

	"plainsolid/geom"
)

// FourPolynomialNodeCutVolumeOptions bounds original support conversion
// and exact arrangement reproof and integration.
type FourPolynomialNodeCutVolumeOptions = PolynomialMixedCutVolumeOptions

// ExactComponentVolumes returns the exact rational volumes of the separate
// positive layers in Components order. The original height difference must
// have a positive constant denominator on every union-knot cell. Both
// native caps and all four contact identities are bound to the measure proof.
func (r *FourPolynomialNodeCut) ExactComponentVolumes(ctx context.Context, o FourPolynomialNodeCutVolumeOptions) (geom.SplineGraphFourPolynomialNodeVolumes, error) {
	zero := geom.SplineGraphFourPolynomialNodeVolumes{}
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
	report, e := geom.IntegrateSplineGraphFourPolynomialNodeVolumes(ctx, roofs[0], roofs[1], r.axis, o.Proof)
	if e != nil {
		return zero, e
	}
	arrangement := report.Arrangement
	if !arrangement.Proven || arrangement.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return zero, geom.ErrConvergence
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return zero, e
	}
	if e = r.material.bindVolumeRegions(r.sources, report.Regions, report.Volumes,
		arrangement.Regions[:], arrangement.Edges[:], arrangement.Vertices[:], arrangement.Nodes[:], uv[:], xyz[:]); e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return report, nil
}
