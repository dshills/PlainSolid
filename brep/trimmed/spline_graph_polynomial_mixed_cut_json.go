package trimmed

import (
	"context"
	"io"
	"slices"

	"plainsolid/geom"
)

const nativePolynomialMixedCut = "trimmed-polynomial-mixed-cut-components"

type polynomialMixedCutRecipe struct {
	polynomialNodeComponentsRecipe
	Orders         []int    `json:"contactOrders"`
	DerivativeGaps []string `json:"derivativeGaps"`
}

// WriteJSON records the original graph supports, both contact types, and
// the indexed component ledger. Reload reconstructs the positive layers.
func (r *PolynomialMixedCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || w == nil {
		return geom.ErrInvalid
	}
	source, e := makePolynomialNodeComponentsRecipe(ctx, nativePolynomialMixedCut, r.sources, r.axis, r.tolerance, r.material.regions, r.material.contacts)
	if e != nil {
		return e
	}
	recipe := polynomialMixedCutRecipe{polynomialNodeComponentsRecipe: source, Orders: r.orders[:]}
	for _, gap := range r.derivatives {
		recipe.DerivativeGaps = append(recipe.DerivativeGaps, gap.RatString())
	}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

// ReadPolynomialMixedCutJSON re-proves both original contacts and all
// positive source sectors before binding the component ledger.
func ReadPolynomialMixedCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialMixedCut, error) {
	var recipe polynomialMixedCutRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if len(recipe.Orders) != 2 || len(recipe.DerivativeGaps) != 2 {
		return nil, geom.ErrInvalid
	}
	sources, e := recipe.decodeSupports(ctx, limits, nativePolynomialMixedCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialMixedComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if !slices.Equal(recipe.Orders, out.orders[:]) {
		return nil, geom.ErrConvergence
	}
	for i, x := range out.derivatives {
		if recipe.DerivativeGaps[i] != x.RatString() {
			return nil, geom.ErrConvergence
		}
	}
	if e = recipe.verify(out.material.regions, out.material.contacts, out.material.components, limits); e != nil {
		return nil, e
	}
	return out, nil
}
