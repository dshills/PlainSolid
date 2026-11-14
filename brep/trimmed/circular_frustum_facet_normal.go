package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Called only with freshly proved exact circular source geometry. Reversing the
// facet for an inner material wall reduces both incidences to the outward proof.
func circularFrustumFacetNormalBound(ctx context.Context, p *geom.SurfacePatch, proof geom.WrappedFrustumEmbedding, lo, hi *big.Rat, facet [3]*big.Rat, inward bool, o geom.SurfacePatchEvaluationOptions) (*big.Rat, error) {
	if inward {
		for k := range facet {
			facet[k] = new(big.Rat).Neg(facet[k])
		}
	}
	if lo.Cmp(hi) >= 0 {
		return nil, geom.ErrConvergence
	}
	for _, cut := range proof.QuarterParameters {
		q := ratInset(cut)
		if q.Cmp(lo) > 0 && q.Cmp(hi) < 0 {
			return nil, geom.ErrConvergence
		}
	}
	h := new(big.Rat).Sub(proof.UpperZ, proof.LowerZ)
	dr := new(big.Rat).Sub(proof.UpperRadius, proof.LowerRadius)
	axial := new(big.Rat).Neg(new(big.Rat).Mul(proof.LowerRadius, dr))
	maximum := new(big.Rat)
	for _, u := range []*big.Rat{lo, hi} {
		jet, e := p.EvaluateRational(ctx, u, ratInset(p.Domain().VMin), o)
		if e != nil {
			return nil, e
		}
		radial := [3]*big.Rat{new(big.Rat).Sub(jet.Point[0], proof.CenterX), new(big.Rat).Sub(jet.Point[1], proof.CenterY), new(big.Rat)}
		if ddDot(facet, radial).Sign() <= 0 {
			return nil, geom.ErrConvergence
		}
		source := [3]*big.Rat{new(big.Rat).Mul(h, radial[0]), new(big.Rat).Mul(h, radial[1]), axial}
		q, ok := ddTanSquared(facet, source)
		if !ok {
			return nil, geom.ErrConvergence
		}
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
	}
	return maximum, nil
}
