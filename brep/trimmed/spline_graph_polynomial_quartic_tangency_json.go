//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialQuarticTangencyEnvelope = "trimmed-polynomial-quartic-tangency-envelope"

func (r *PolynomialQuarticTangencyEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], nil, h.RoofFaces[:], h.RoofOwners[:], h.Base, []PolynomialMixedEnvelopeContact{r.contact})
}

// WriteJSON records the original supports and exact native identities of the
// order-four junction and four signed roof sectors.
func (r *PolynomialQuarticTangencyEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativePolynomialQuarticTangencyEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadPolynomialQuarticTangencyEnvelopeJSON re-proves the quartic separation,
// original owner coedges and native vertex link before binding the recipe
// ledger.
func ReadPolynomialQuarticTangencyEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialQuarticTangencyEnvelopeOptions) (*PolynomialQuarticTangencyEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativePolynomialQuarticTangencyEnvelope, 1)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialQuarticTangency(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
