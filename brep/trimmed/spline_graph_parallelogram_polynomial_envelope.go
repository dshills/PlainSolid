package trimmed

import (
	"context"
	"io"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeParallelogramPolynomialEnvelope = "trimmed-parallelogram-polynomial-envelope"

// CombineParallelogramPolynomialEnvelope builds Common or Fuse on the exact
// physical parallelogram shared by an original base and two independently
// parameterized affine XY roofs. The roofs must be strictly above the base
// and meet in a cubic crossing and a quadratic tangency. Axis selects the
// free axis of the positively oriented unit footprint chart beginning at
// its least physical XY corner. Native trims retain all original supports.
func CombineParallelogramPolynomialEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		(op != parametric.Intersection && op != parametric.Union) || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	stock, e := prepareFootprintPolynomialStock(ctx, base, a, b, o, geom.NewAffineXYParallelogramReference)
	if e != nil {
		return nil, e
	}
	baseChart, e := polynomialEnvelopeBaseChart(stock.base)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverProjectedSplineGraphPolynomialArrangement(ctx, stock.reference, stock.roofs[0], stock.roofs[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	graph, e := geom.NewAffineXYGraph(ctx, stock.reference, o.Section.Patch)
	if e != nil {
		return nil, e
	}
	reference := func(p geom.Vec3) (geom.Vec2, error) {
		q, e := graph.InvertRational(ctx, [2]*big.Rat{ratInset(p.X), ratInset(p.Y)}, geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.Proof.MaxOperations, MaxCoefficientBits: o.Proof.MaxCoefficientBits,
		})
		if e != nil {
			return geom.Vec2{}, e
		}
		if !q.Inside {
			return geom.Vec2{}, geom.ErrDomain
		}
		return exactGraphPartitionUV(q.UV)
	}
	return buildNativePolynomialEnvelopeOnFootprint(ctx, [3]*geom.BSplineSurface{base, a, b}, 0, axis, op, tolerance, o,
		nativeParallelogramPolynomialEnvelope, proof.Arrangement, proof.Owners, baseChart, projectedPolynomialCharts(proof), reference)
}

// ReadParallelogramPolynomialEnvelopeJSON re-proves the original footprint,
// heights, wall and roof chart identities and physical junctions.
func ReadParallelogramPolynomialEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativeParallelogramPolynomialEnvelope, 2)
	if e != nil {
		return nil, e
	}
	out, e := CombineParallelogramPolynomialEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
