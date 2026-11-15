package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialTangencyCut = "trimmed-polynomial-tangency-cut-components"

// WriteJSON records the original graph supports and the two local vertex IDs
// sharing the exact tangent point. Component boundaries are reconstructed.
func (r *PolynomialTangencyCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil {
		return geom.ErrInvalid
	}
	return writePolynomialPointCutJSON(ctx, w, r.pointCut, nativePolynomialTangencyCut)
}

// ReadPolynomialTangencyCutJSON rederives the squared-factor and one-sided
// order proof, binds the original coedges, and verifies the shared point ledger.
func ReadPolynomialTangencyCutJSON(ctx context.Context, r io.Reader, limits JSONLimits, o PolynomialNodeCutOptions) (*PolynomialTangencyCut, error) {
	recipe, sources, e := readPolynomialPointCutRecipe(ctx, r, limits, nativePolynomialTangencyCut)
	if e != nil {
		return nil, e
	}
	out, e := CutSplineGraphPolynomialTangencyComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = bindPolynomialPointCutRecipe(out.pointCut, recipe, limits); e != nil {
		return nil, e
	}
	return out, nil
}
