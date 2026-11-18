//go:build research

package trimmed

import (
	"context"
	"io"
	"math/big"

	"plainsolid/geom"
)

const nativePolynomialCubicCrossingCut = "trimmed-polynomial-cubic-crossing-cut-components"

// WriteJSON writes the original supports and the indexed, typed contact ledger.
func (r *PolynomialCubicCrossingCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativePolynomialCubicCrossingCut, r.sources, r.axis, r.tolerance, r.material, []int{3}, []*big.Rat{r.thirdDerivativeGap})
}

// ReadPolynomialCubicCrossingCutJSON rebuilds the original layers and
// binds the order-three contact and its component-local vertices.
func ReadPolynomialCubicCrossingCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialCubicCrossingCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativePolynomialCubicCrossingCut, 1)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialCubicCrossingComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, []int{3}, []*big.Rat{out.thirdDerivativeGap}, limits); e != nil {
		return nil, e
	}
	return out, nil
}
