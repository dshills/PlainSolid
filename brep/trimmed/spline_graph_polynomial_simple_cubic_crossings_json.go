//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialSimpleCubicCrossingsEnvelope = "trimmed-polynomial-simple-cubic-crossings-envelope"

func (r *PolynomialSimpleCubicCrossingsEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], h.Midpoints[:], h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

// WriteJSON records the original supports and exact native identities of both
// crossings and the five signed roof sectors.
func (r *PolynomialSimpleCubicCrossingsEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativePolynomialSimpleCubicCrossingsEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadPolynomialSimpleCubicCrossingsEnvelopeJSON re-proves both original
// crossings, owner coedges and native vertex links before binding the
// recipe ledger.
func ReadPolynomialSimpleCubicCrossingsEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialSimpleCubicCrossingsEnvelopeOptions) (*PolynomialSimpleCubicCrossingsEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativePolynomialSimpleCubicCrossingsEnvelope, 2)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialSimpleCubicCrossings(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
