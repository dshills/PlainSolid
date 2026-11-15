package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialTwoCrossingsTangencyCut = "trimmed-polynomial-two-crossings-tangency-cut-components"

// WriteJSON writes the original supports and the indexed, typed contact ledger.
func (r *PolynomialTwoCrossingsTangencyCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativePolynomialTwoCrossingsTangencyCut, r.sources, r.axis, r.tolerance, r.material, r.orders[:], r.derivatives[:])
}

// ReadPolynomialTwoCrossingsTangencyCutJSON rebuilds the original layers and
// binds all three recorded contact orders, coordinates and local vertices.
func ReadPolynomialTwoCrossingsTangencyCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialTwoCrossingsTangencyCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativePolynomialTwoCrossingsTangencyCut, 3)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialTwoCrossingsTangencyComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, out.orders[:], out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
