package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialTwoCrossingsTangencyEnvelope = "trimmed-polynomial-two-crossings-tangency-envelope"

func (r *PolynomialTwoCrossingsTangencyEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	mids := []int{h.Midpoints[0][0], h.Midpoints[0][1], h.Midpoints[1][0], h.Midpoints[1][1]}
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], mids, h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

// WriteJSON records the original supports and exact native identities of all
// three typed junctions and six signed roof sectors.
func (r *PolynomialTwoCrossingsTangencyEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativePolynomialTwoCrossingsTangencyEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadPolynomialTwoCrossingsTangencyEnvelopeJSON re-proves the two transverse
// crossings, tangency, original owner coedges and native vertex links before
// binding the recipe ledger.
func ReadPolynomialTwoCrossingsTangencyEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialTwoCrossingsTangencyEnvelopeOptions) (*PolynomialTwoCrossingsTangencyEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativePolynomialTwoCrossingsTangencyEnvelope, 3)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialTwoCrossingsTangency(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
