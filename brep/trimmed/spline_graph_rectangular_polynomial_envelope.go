package trimmed

import (
	"context"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeRectangularPolynomialEnvelope = "trimmed-rectangular-polynomial-envelope"

// CombineRectangularPolynomialEnvelope builds Common or Fuse over the
// original base's axis-aligned physical rectangular footprint. The base and
// both roofs may use independent affine XY charts. The roofs must be
// strictly above the base and meet in a cubic crossing and a quadratic
// tangency. Axis is the physical free coordinate. Each cap and wall trim
// uses its original surface's parameters.
func CombineRectangularPolynomialEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		(op != parametric.Intersection && op != parametric.Union) || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	limits := o.Section.Patch
	bp, e := geom.NewSurfacePatch(base, limits)
	if e != nil {
		return nil, e
	}
	reference, e := geom.NewAffineXYRectangleReference(ctx, bp, limits)
	if e != nil {
		return nil, e
	}
	baseAlignment, e := geom.AlignAffineXYCharts(ctx, reference, bp, limits)
	if e != nil {
		return nil, e
	}
	mapping, e := baseAlignment.MapCoefficients()
	if e != nil {
		return nil, e
	}
	orientation := (mapping[0][0].Sign() * mapping[1][1].Sign()) - (mapping[0][1].Sign() * mapping[1][0].Sign())
	baseChart := polynomialEnvelopeChart(mapping, orientation)
	patches, e := preparePositivePolynomialRoofs(ctx, baseAlignment.TargetOnSource(), [2]*geom.BSplineSurface{a, b}, o)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverProjectedSplineGraphPolynomialArrangement(ctx, reference, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	return buildNativePolynomialEnvelope(ctx, [3]*geom.BSplineSurface{base, a, b}, 0, axis, op, tolerance, o,
		nativeRectangularPolynomialEnvelope, proof.Arrangement, proof.Owners, baseChart, projectedPolynomialCharts(proof))
}

func bindRectangularEnvelopeBase(s Solid, base *geom.BSplineSurface, chart graphEnvelopeChart, h SplineGraphBooleanHistory) error {
	f := s.faces[h.Base]
	if f.Surface != base || f.Reversed != !chart.reversed || len(f.Outer) != len(h.WallFaces) {
		return geom.ErrConvergence
	}
	pieces := make(map[int]openSectionPiece, len(f.Outer))
	for _, use := range f.Outer {
		e := s.edges[use.Edge]
		a, b := s.vertices[e.Start], s.vertices[e.End]
		owned, err := chart.piece(openSectionPiece{a: geom.Vec2{X: a.X, Y: a.Y}, b: geom.Vec2{X: b.X, Y: b.Y}})
		if err != nil {
			return err
		}
		curve, ok := e.Curve.(surfaceTrimCurve)
		if !ok || curve.surface != base || curve.curve != nil || curve.a != owned.a || curve.b != owned.b ||
			use.Curve != nil || use.Reversed != chart.reversed {
			return geom.ErrConvergence
		}
		start, end := owned.a, owned.b
		if use.Reversed {
			start, end = end, start
		}
		if use.UVStart != start || use.UVEnd != end {
			return geom.ErrConvergence
		}
		pieces[use.Edge] = owned
	}
	for _, index := range h.WallFaces {
		wall := s.faces[index]
		if len(wall.Outer) != 4 {
			return geom.ErrConvergence
		}
		use := wall.Outer[0]
		owned, exists := pieces[use.Edge]
		trim, ok := graphWallSourceTrim(use.Curve, false, true)
		if !exists || !ok || use.Reversed || trim.surface != base || trim.a != owned.a || trim.b != owned.b {
			return geom.ErrConvergence
		}
		delete(pieces, use.Edge)
	}
	if len(pieces) != 0 {
		return geom.ErrConvergence
	}
	return nil
}

// ReadRectangularPolynomialEnvelopeJSON re-proves the original base footprint,
// both roof charts, and the exact original trim and junction ledger.
func ReadRectangularPolynomialEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativeRectangularPolynomialEnvelope, 2)
	if e != nil {
		return nil, e
	}
	out, e := CombineRectangularPolynomialEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
