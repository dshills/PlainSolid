package trimmed

import (
	"context"
	"io"
	"math/big"
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
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativePolynomialMixedCut, r.sources, r.axis, r.tolerance, r.material, r.orders[:], r.derivatives[:])
}

func writePolynomialMixedCutRecipe(ctx context.Context, w io.Writer, kind string, stock [3]*geom.BSplineSurface, axis int, tolerance float64,
	material polynomialNodeCutMaterial, orders []int, derivatives []*big.Rat) error {
	if w == nil {
		return geom.ErrInvalid
	}
	source, e := makePolynomialNodeComponentsRecipe(ctx, kind, stock, axis, tolerance, material.regions, material.contacts)
	if e != nil {
		return e
	}
	recipe := polynomialMixedCutRecipe{polynomialNodeComponentsRecipe: source, Orders: orders}
	for _, gap := range derivatives {
		recipe.DerivativeGaps = append(recipe.DerivativeGaps, gap.RatString())
	}
	return writePolynomialGraphRecipe(ctx, w, recipe)
}

// ReadPolynomialMixedCutJSON re-proves both original contacts and all
// positive source sectors before binding the component ledger.
func ReadPolynomialMixedCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialMixedCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativePolynomialMixedCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialMixedComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, out.orders[:], out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}

func readPolynomialMixedCutRecipe(ctx context.Context, r io.Reader, limits JSONLimits, kind string, nodes int) (polynomialMixedCutRecipe, [3]*geom.BSplineSurface, error) {
	var recipe polynomialMixedCutRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return polynomialMixedCutRecipe{}, [3]*geom.BSplineSurface{}, e
	}
	if len(recipe.Orders) != nodes || len(recipe.DerivativeGaps) != nodes {
		return polynomialMixedCutRecipe{}, [3]*geom.BSplineSurface{}, geom.ErrInvalid
	}
	sources, e := recipe.decodeSupports(ctx, limits, kind, nodes)
	return recipe, sources, e
}

func (recipe polynomialMixedCutRecipe) verifyTypedContacts(material polynomialNodeCutMaterial, orders []int, derivatives []*big.Rat, limits JSONLimits) error {
	if !slices.Equal(recipe.Orders, orders) || len(recipe.DerivativeGaps) != len(derivatives) {
		return geom.ErrConvergence
	}
	for i, x := range derivatives {
		if recipe.DerivativeGaps[i] != x.RatString() {
			return geom.ErrConvergence
		}
	}
	return recipe.verify(material.regions, material.contacts, material.components, limits)
}
