package trimmed

import (
	"context"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeProjectedPolynomialEnvelope = "trimmed-projected-polynomial-envelope"

// CombineProjectedPolynomialEnvelope builds Common or Fuse for a cubic
// crossing and a quadratic tangency using the unit identity base as an
// independent physical chart. Both original roofs may use distinct affine
// XY charts. They must cover the full base and be strictly above it. Axis
// selects the physical free coordinate. Native trims use each original roof.
func CombineProjectedPolynomialEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		(op != parametric.Intersection && op != parametric.Union) || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	limits := o.Section.Patch
	if e := requireIdentitySplineChart(ctx, base, limits); e != nil {
		return nil, e
	}
	bp, e := geom.NewSurfacePatch(base, limits)
	if e != nil {
		return nil, e
	}
	patches, e := preparePositivePolynomialRoofs(ctx, bp, [2]*geom.BSplineSurface{a, b}, o)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverProjectedSplineGraphPolynomialArrangement(ctx, bp, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	var charts [2]graphEnvelopeChart
	for i, owner := range proof.Owners {
		charts[i] = polynomialEnvelopeChart(proof.Maps[i], owner.ProjectionOrientation)
	}
	return buildNativePolynomialEnvelope(ctx, [3]*geom.BSplineSurface{base, a, b}, 0, axis, op, tolerance, o,
		nativeProjectedPolynomialEnvelope, proof.Arrangement, proof.Owners, charts)
}

// ReadProjectedPolynomialEnvelopeJSON rebuilds each original cap and wall
// trace through its exact chart map and verifies the native contact ledger.
func ReadProjectedPolynomialEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativeProjectedPolynomialEnvelope, 2)
	if e != nil {
		return nil, e
	}
	out, e := CombineProjectedPolynomialEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
