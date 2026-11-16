package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeClippedPolynomialThreeTransverseCut = "trimmed-clipped-polynomial-three-transverse-cut-components"

// WriteJSON records the original stocks, all three indexed contacts and
// each positive source chamber, wall binding and volume.
func (r *ClippedPolynomialThreeTransverseCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || r == nil || r.cut == nil || w == nil {
		return geom.ErrInvalid
	}
	cut := r.cut
	source, e := makePolynomialNodeComponentsRecipe(ctx, nativeClippedPolynomialThreeTransverseCut,
		r.sources, r.axis, r.tolerance, cut.material.regions, cut.material.contacts)
	if e != nil {
		return e
	}
	mixed := polynomialMixedCutRecipe{polynomialNodeComponentsRecipe: source, Orders: cut.orders}
	for _, d := range cut.derivatives {
		mixed.DerivativeGaps = append(mixed.DerivativeGaps, d.RatString())
	}
	recipe := clippedPolynomialCutRecipe{mixed, cut.footprints, cut.shells, cut.total.RatString()}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

// ReadClippedPolynomialThreeTransverseCutJSON re-proves the original
// three-crossing arrangement and positive shell links before checking the
// recorded contact vertices, wall ownership and physical volumes.
func ReadClippedPolynomialThreeTransverseCutJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialNodeCutOptions) (*ClippedPolynomialThreeTransverseCut, error) {
	var recipe clippedPolynomialCutRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if len(recipe.Orders) != 3 || len(recipe.DerivativeGaps) != 3 || len(recipe.Shells) < 2 || recipe.Total == "" ||
		!recipe.Footprints[0].Valid() || !recipe.Footprints[1].Valid() {
		return nil, geom.ErrInvalid
	}
	sources, e := recipe.decodeSupports(ctx, limits, nativeClippedPolynomialThreeTransverseCut, 3)
	if e != nil {
		return nil, e
	}
	out, e := CutClippedPolynomialThreeTransverseComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	cut := out.cut
	if e = recipe.verifyTypedContacts(cut.material, cut.orders, cut.derivatives, limits); e != nil {
		return nil, e
	}
	if recipe.Footprints != cut.footprints || recipe.Total != cut.total.RatString() || !reflect.DeepEqual(recipe.Shells, cut.shells) {
		return nil, geom.ErrConvergence
	}
	return out, nil
}
