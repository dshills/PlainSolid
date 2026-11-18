//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialThreeCrossingsTangencyCut = "trimmed-polynomial-three-crossings-tangency-cut-components"

// WriteJSON writes the original supports and the indexed, typed contact ledger.
func (r *PolynomialThreeCrossingsTangencyCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativePolynomialThreeCrossingsTangencyCut, r.sources, r.axis, r.tolerance, r.material, r.orders[:], r.derivatives[:])
}

// ReadPolynomialThreeCrossingsTangencyCutJSON rebuilds the original layers and
// binds all four recorded contact orders, coordinates and local vertices.
func ReadPolynomialThreeCrossingsTangencyCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialThreeCrossingsTangencyCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativePolynomialThreeCrossingsTangencyCut, 4)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialThreeCrossingsTangencyComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, out.orders[:], out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
