package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeFivePolynomialNodeCut = "trimmed-five-polynomial-node-cut-components"

// WriteJSON writes the original supports and the indexed, typed contact ledger.
func (r *FivePolynomialNodeCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativeFivePolynomialNodeCut, r.sources, r.axis, r.tolerance, r.material, []int{1, 1, 1, 1, 1}, r.derivatives[:])
}

// ReadFivePolynomialNodeCutJSON rebuilds the original layers and
// binds all five recorded contact orders, coordinates and local vertices.
func ReadFivePolynomialNodeCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*FivePolynomialNodeCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativeFivePolynomialNodeCut, 5)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphFivePolynomialNodeComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, []int{1, 1, 1, 1, 1}, out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
