package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeFourPolynomialNodeEnvelope = "trimmed-four-polynomial-node-envelope"

func (r *FourPolynomialNodeEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	var mids []int
	for _, pair := range h.Midpoints {
		mids = append(mids, pair[:]...)
	}
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], mids, h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

// WriteJSON records the original supports and the native ledgers for all
// four crossings and their seven signed sectors.
func (r *FourPolynomialNodeEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativeFourPolynomialNodeEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadFourPolynomialNodeEnvelopeJSON re-proves original coverage, exact owner
// coedges, and native manifold links before binding the recorded identities.
func ReadFourPolynomialNodeEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits, o FourPolynomialNodeEnvelopeOptions) (*FourPolynomialNodeEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativeFourPolynomialNodeEnvelope, 4)
	if e != nil {
		return nil, e
	}
	out, e := CombineSplineGraphFourPolynomialNodes(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
