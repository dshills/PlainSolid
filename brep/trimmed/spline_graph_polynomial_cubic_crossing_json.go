package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialCubicCrossingEnvelope = "trimmed-polynomial-cubic-crossing-envelope"

func (r *PolynomialCubicCrossingEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], nil, h.RoofFaces[:], h.RoofOwners[:], h.Base, []PolynomialMixedEnvelopeContact{r.contact})
}

// WriteJSON records the original supports and exact native identities of the
// order-three junction and four signed roof sectors.
func (r *PolynomialCubicCrossingEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativePolynomialCubicCrossingEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadPolynomialCubicCrossingEnvelopeJSON re-proves the cubic separation,
// original owner coedges and native vertex link before binding the recipe
// ledger.
func ReadPolynomialCubicCrossingEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingEnvelopeOptions) (*PolynomialCubicCrossingEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativePolynomialCubicCrossingEnvelope, 1)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialCubicCrossing(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
