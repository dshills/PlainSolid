//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialSimpleCubicCrossingsCut = "trimmed-polynomial-simple-cubic-crossings-cut-components"

// WriteJSON writes the original supports and the indexed, typed contact ledger.
func (r *PolynomialSimpleCubicCrossingsCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativePolynomialSimpleCubicCrossingsCut, r.sources, r.axis, r.tolerance, r.material, r.orders[:], r.derivatives[:])
}

// ReadPolynomialSimpleCubicCrossingsCutJSON rebuilds the original layers and
// binds both exact crossing orders and their component-local vertices.
func ReadPolynomialSimpleCubicCrossingsCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialSimpleCubicCrossingsCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativePolynomialSimpleCubicCrossingsCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialSimpleCubicCrossingsComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, out.orders[:], out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
