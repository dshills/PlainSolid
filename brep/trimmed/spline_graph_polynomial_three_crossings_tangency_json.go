//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativePolynomialThreeCrossingsTangencyEnvelope = "trimmed-polynomial-three-crossings-tangency-envelope"

func (r *PolynomialThreeCrossingsTangencyEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	mids := []int{h.Midpoints[0][0], h.Midpoints[0][1], h.Midpoints[1][0], h.Midpoints[1][1], h.Midpoints[2][0], h.Midpoints[2][1]}
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], mids, h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

// WriteJSON records the original supports and exact native identities of all
// four typed junctions and seven signed roof sectors.
func (r *PolynomialThreeCrossingsTangencyEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativePolynomialThreeCrossingsTangencyEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadPolynomialThreeCrossingsTangencyEnvelopeJSON re-proves the three transverse
// crossings, tangency, original owner coedges and native vertex links before
// binding the recipe ledger.
func ReadPolynomialThreeCrossingsTangencyEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialThreeCrossingsTangencyEnvelopeOptions) (*PolynomialThreeCrossingsTangencyEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativePolynomialThreeCrossingsTangencyEnvelope, 4)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphPolynomialThreeCrossingsTangency(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
