package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeParallelogramPolynomialCut = "trimmed-parallelogram-polynomial-cut-components"

// CutParallelogramPolynomialComponents subtracts B from A over the exact
// shared parallelogram of the original base and the two original affine
// XY roofs. Both roofs must lie strictly above the base. Axis selects the
// free axis of the positively oriented unit footprint chart. A cubic
// crossing and a quadratic tangency bound the separate positive layers.
// Each native shell retains its original roof trims, exact physical wall
// charts and physical volume.
func CutParallelogramPolynomialComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o PolynomialNodeCutOptions) (*AlignedPolynomialCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	options := PolynomialCubicCrossingTangencyEnvelopeOptions(o)
	stock, e := prepareFootprintPolynomialStock(ctx, base, a, b, options, geom.NewAffineXYParallelogramReference)
	if e != nil {
		return nil, e
	}
	reference, e := newGraphEnvelopeReference(ctx, stock.reference, options)
	if e != nil {
		return nil, e
	}
	report, e := geom.IntegrateAffineProjectedSplineGraphPolynomialVolumes(ctx, stock.reference, stock.roofs[0], stock.roofs[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	p := report.Projection
	return buildNativePolynomialCutOnFootprint(ctx, [3]*geom.BSplineSurface{base, a, b}, axis, tolerance, o,
		nativeParallelogramPolynomialCut, p.Arrangement, p.Owners, projectedPolynomialCharts(p),
		polynomialCutVolumes{report.Regions, report.Volumes, report.Total}, reference)
}

// ReadParallelogramPolynomialCutJSON reconstructs the original charts,
// absolute physical contacts, exact wall traces and component volumes before
// returning the verified native shells.
func ReadParallelogramPolynomialCutJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialNodeCutOptions) (*AlignedPolynomialCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativeParallelogramPolynomialCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutParallelogramPolynomialComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, out.orders[:], out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
