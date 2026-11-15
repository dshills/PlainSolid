package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeFourPolynomialNodeCut = "trimmed-four-polynomial-node-cut-components"

// WriteJSON records both original roofs, the shared base, and the four
// exact contact identities. The reader rebuilds the component boundaries.
func (r *FourPolynomialNodeCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialNodeComponentsRecipe(ctx, w, nativeFourPolynomialNodeCut, r.sources, r.axis, r.tolerance, r.material.regions, r.material.contacts)
}

// ReadFourPolynomialNodeCutJSON re-proves the source arrangement and exact
// component caps before binding all four recorded contact identities.
func ReadFourPolynomialNodeCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*FourPolynomialNodeCut, error) {
	recipe, sources, e := readPolynomialNodeComponentsRecipe(ctx, r, limits, nativeFourPolynomialNodeCut, 4)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphFourPolynomialNodeComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.material.regions, out.material.contacts, out.material.components, limits); e != nil {
		return nil, e
	}
	return out, nil
}
