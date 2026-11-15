package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

type clippedPolynomialCutRecipe struct {
	polynomialMixedCutRecipe
	Footprints [2]geom.ParameterRectangle        `json:"footprints"`
	Shells     []clippedPolynomialCutShellLedger `json:"shells"`
	Total      string                            `json:"exactTotal"`
}

// WriteJSON records the original stocks, retained chamber groups and
// volumes, exposed walls and exact component-local contact vertices.
func (r *ClippedPolynomialCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || r == nil || r.native == nil || w == nil {
		return geom.ErrInvalid
	}
	n := r.native
	source, e := makePolynomialNodeComponentsRecipe(ctx, nativeClippedPolynomialCut, n.sources, n.axis, n.tolerance, n.material.regions, n.material.contacts)
	if e != nil {
		return e
	}
	mixed := polynomialMixedCutRecipe{polynomialNodeComponentsRecipe: source, Orders: n.orders[:]}
	for _, d := range n.derivatives {
		mixed.DerivativeGaps = append(mixed.DerivativeGaps, d.RatString())
	}
	recipe := clippedPolynomialCutRecipe{mixed, r.footprints, r.shells, n.total.RatString()}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

// ReadClippedPolynomialCutJSON re-proves the original chamber graph and
// reconstructs each embedded shell before binding the recorded contacts.
func ReadClippedPolynomialCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*ClippedPolynomialCut, error) {
	var recipe clippedPolynomialCutRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if len(recipe.Orders) != 2 || len(recipe.DerivativeGaps) != 2 || len(recipe.Shells) < 2 || recipe.Total == "" ||
		!recipe.Footprints[0].Valid() || !recipe.Footprints[1].Valid() {
		return nil, geom.ErrInvalid
	}
	sources, e := recipe.decodeSupports(ctx, limits, nativeClippedPolynomialCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutClippedPolynomialComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	n := out.native
	if e = recipe.verifyTypedContacts(n.material, n.orders[:], n.derivatives[:], limits); e != nil {
		return nil, e
	}
	if recipe.Footprints != out.footprints || recipe.Total != n.total.RatString() || !reflect.DeepEqual(recipe.Shells, out.shells) {
		return nil, geom.ErrConvergence
	}
	return out, nil
}
