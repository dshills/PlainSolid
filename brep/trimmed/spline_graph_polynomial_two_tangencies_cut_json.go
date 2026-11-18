//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialTwoTangenciesCut = "trimmed-polynomial-two-tangencies-cut-components"

// WriteJSON writes the original supports and the indexed, typed contact ledger.
func (r *PolynomialTwoTangenciesCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativePolynomialTwoTangenciesCut, r.sources, r.axis, r.tolerance, r.material, []int{2, 2}, r.derivatives[:])
}

// ReadPolynomialTwoTangenciesCutJSON rebuilds the original layers and
// binds both quadratic contact orders, coordinates and local vertices.
func ReadPolynomialTwoTangenciesCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialTwoTangenciesCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativePolynomialTwoTangenciesCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialTwoTangenciesComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, []int{2, 2}, out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
