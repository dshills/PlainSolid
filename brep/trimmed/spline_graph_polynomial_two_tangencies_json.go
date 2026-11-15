package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialTwoTangenciesEnvelope = "trimmed-polynomial-two-tangencies-envelope"

func (r *PolynomialTwoTangenciesEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], h.Midpoints[:], h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

// WriteJSON records the original supports and exact native identities of both
// quadratic junctions and five signed roof sectors.
func (r *PolynomialTwoTangenciesEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativePolynomialTwoTangenciesEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadPolynomialTwoTangenciesEnvelopeJSON re-proves both quadratic
// tangencies, original owner coedges and native vertex links before binding
// the recipe ledger.
func ReadPolynomialTwoTangenciesEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialTwoTangenciesEnvelopeOptions) (*PolynomialTwoTangenciesEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativePolynomialTwoTangenciesEnvelope, 2)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialTwoTangencies(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
