//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeFivePolynomialNodeEnvelope = "trimmed-five-polynomial-node-envelope"

func (r *FivePolynomialNodeEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	var mids []int
	for _, pair := range h.Midpoints {
		mids = append(mids, pair[:]...)
	}
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], mids, h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

// WriteJSON records the original supports and the native ledgers for all
// five crossings and their eight signed sectors.
func (r *FivePolynomialNodeEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativeFivePolynomialNodeEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadFivePolynomialNodeEnvelopeJSON re-proves original coverage, exact owner
// coedges, and native manifold links before binding the recorded identities.
func ReadFivePolynomialNodeEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits, o FivePolynomialNodeEnvelopeOptions) (*FivePolynomialNodeEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativeFivePolynomialNodeEnvelope, 5)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphFivePolynomialNodes(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
