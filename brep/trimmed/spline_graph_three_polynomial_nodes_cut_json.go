package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeThreePolynomialNodeCut = "trimmed-three-polynomial-node-cut-components"

// WriteJSON writes the original graph supports and the three indexed point
// contacts. Reload reconstructs the positive native layers from these sources.
func (r *ThreePolynomialNodeCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialNodeComponentsRecipe(ctx, w, nativeThreePolynomialNodeCut, r.sources, r.axis, r.tolerance, r.regions, r.contacts[:])
}

// ReadThreePolynomialNodeCutJSON independently re-proves original arrangement
// coverage, reconstructs the layers, and checks all point-contact identities.
func ReadThreePolynomialNodeCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*ThreePolynomialNodeCut, error) {
	recipe, sources, e := readPolynomialNodeComponentsRecipe(ctx, r, limits, nativeThreePolynomialNodeCut, 3)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphThreePolynomialNodeComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.regions, out.contacts[:], out.components, limits); e != nil {
		return nil, e
	}
	return out, nil
}
