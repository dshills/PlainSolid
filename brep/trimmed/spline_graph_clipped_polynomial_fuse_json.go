package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type clippedPolynomialFuseRecipe struct {
	polynomialEnvelopeRecipe
	Footprints [2]geom.ParameterRectangle  `json:"footprints"`
	Walls      []ClippedPolynomialFuseWall `json:"walls"`
	Volume     string                      `json:"exactVolume"`
}

// WriteJSON records the original stocks and the native wall, contact and
// source-volume ledger. Reload reconstructs the boundary from the stocks.
func (r *ClippedPolynomialFuse) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	supports, e := encodePolynomialGraphSupports(ctx, r.sources)
	if e != nil {
		return e
	}
	axis, op := r.axis, parametric.Union
	recipe := polynomialEnvelopeRecipe{Version: 1, Kind: nativeClippedPolynomialFuse, Sources: supports,
		Axis: &axis, Operation: &op, Tolerance: r.tolerance, Ledger: r.ledger}
	return writePolynomialGraphRecipe(ctx, w, clippedPolynomialFuseRecipe{recipe, r.footprints, r.walls, r.volume.RatString()})
}

// ReadClippedPolynomialFuseJSON re-proves the exact original stock coverage,
// steps and stations before binding the rebuilt edge and face incidences.
func ReadClippedPolynomialFuseJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*ClippedPolynomialFuse, error) {
	var recipe clippedPolynomialFuseRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if (!recipe.validHeader(nativeClippedPolynomialFuse, 2) &&
		!recipe.validHeader(nativeClippedPolynomialFuse, 3) &&
		!recipe.validHeader(nativeClippedPolynomialFuse, 4)) ||
		*recipe.Operation != parametric.Union ||
		!recipe.Footprints[0].Valid() || !recipe.Footprints[1].Valid() || len(recipe.Walls) == 0 || recipe.Volume == "" {
		return nil, geom.ErrInvalid
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if e != nil {
		return nil, e
	}
	out, e := FuseClippedPolynomialEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.ledger, out.solid, limits); e != nil {
		return nil, e
	}
	if recipe.Footprints != out.footprints || recipe.Volume != out.volume.RatString() || !reflect.DeepEqual(recipe.Walls, out.walls) {
		return nil, geom.ErrConvergence
	}
	return out, nil
}
