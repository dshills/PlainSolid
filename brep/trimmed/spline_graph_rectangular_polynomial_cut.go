//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeRectangularPolynomialCut = "trimmed-rectangular-polynomial-cut-components"

// CutRectangularPolynomialComponents subtracts B from A over their shared
// base's axis-aligned physical rectangular footprint. The base and both
// roofs may use independent affine XY charts. Both roofs must be strictly
// above the base. This path requires a cubic crossing, a quadratic tangency,
// and a constant physical quadratic height scale. Each positive layer has
// original roof and wall trims, exact physical volume, and indexed junctions.
func CutRectangularPolynomialComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o PolynomialNodeCutOptions) (*AlignedPolynomialCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	stock, e := prepareRectangularPolynomialStock(ctx, base, a, b, PolynomialCubicCrossingTangencyEnvelopeOptions(o))
	if e != nil {
		return nil, e
	}
	report, e := geom.IntegrateProjectedSplineGraphPolynomialVolumes(ctx, stock.reference, stock.roofs[0], stock.roofs[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	p := report.Projection
	return buildNativePolynomialCut(ctx, [3]*geom.BSplineSurface{base, a, b}, axis, tolerance, o,
		nativeRectangularPolynomialCut, p.Arrangement, p.Owners, projectedPolynomialCharts(p),
		polynomialCutVolumes{report.Regions, report.Volumes, report.Total})
}

// ReadRectangularPolynomialCutJSON reconstructs the original source charts,
// exact physical volumes and contact incidence before publishing the native
// components.
func ReadRectangularPolynomialCutJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialNodeCutOptions) (*AlignedPolynomialCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativeRectangularPolynomialCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutRectangularPolynomialComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, out.orders[:], out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
