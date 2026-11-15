package trimmed

import (
	"context"
	"io"
	"math/big"

	"plainsolid/geom"
)

const nativePolynomialQuarticTangencyCut = "trimmed-polynomial-quartic-tangency-cut-components"

// WriteJSON writes the original supports and the indexed, typed contact ledger.
func (r *PolynomialQuarticTangencyCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, nativePolynomialQuarticTangencyCut, r.sources, r.axis, r.tolerance, r.material, []int{4}, []*big.Rat{r.fourthDerivativeGap})
}

// ReadPolynomialQuarticTangencyCutJSON rebuilds the original layers and
// binds the order-four contact and its component-local vertices.
func ReadPolynomialQuarticTangencyCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialQuarticTangencyCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativePolynomialQuarticTangencyCut, 1)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialQuarticTangencyComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, []int{4}, []*big.Rat{out.fourthDerivativeGap}, limits); e != nil {
		return nil, e
	}
	return out, nil
}
