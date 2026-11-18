//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TessellatePairedBezierGraphChainsRefinedCertified adds bounded exact common
// parameter refinement to the strict paired API. maxRounds must be 1..8.
// Each round replans the union of both current partitions and independently
// recertifies both UV fills and lifted meshes. Adaptive additions may require
// another round; failure to match after maxRounds rejects atomically.
// All original proof limits apply per face per round: at most 2*(maxRounds+1)
// face proofs, maxRounds bounded partition plans, and one final B display proof.
// Exact model identity and opposite directed seam incidence are required; no
// shell embedding or nonintersection claim is added.
func TessellatePairedBezierGraphChainsRefinedCertified(ctx context.Context, a *geom.SurfacePatch, qa []ExactBezierUV, baseA *big.Rat, b *geom.SurfacePatch, qb []ExactBezierUV, baseB *big.Rat, o CurvedBezierGraphChainOptions, maxRounds int) (CertifiedPairedBezierGraphMesh, error) {
	if maxRounds < 1 || maxRounds > 8 {
		return CertifiedPairedBezierGraphMesh{}, geom.ErrInvalid
	}
	return tessellatePairedBezierGraphChains(ctx, a, qa, baseA, b, qb, baseB, o, maxRounds)
}
func graphChainPartitionsEqual(a, b CertifiedCurvedBezierGraphChain) bool {
	if len(a.GraphChain.Pieces) != len(b.GraphChain.Pieces) {
		return false
	}
	for i, p := range a.GraphChain.Pieces {
		q := b.GraphChain.Pieces[i]
		if len(p.Curve.Vertices) != len(q.Curve.Vertices) {
			return false
		}
		for j, v := range p.Curve.Vertices {
			w := q.Curve.Vertices[j]
			if v.Parameter.Lower.Cmp(w.Parameter.Lower) != 0 || v.Parameter.Upper.Cmp(w.Parameter.Upper) != 0 {
				return false
			}
		}
	}
	return true
}
func commonGraphChainCuts(ctx context.Context, a, b CertifiedCurvedBezierGraphChain) ([][]*big.Rat, error) {
	extract := func(x CertifiedCurvedBezierGraphChain) ([][]*big.Rat, error) {
		out := make([][]*big.Rat, len(x.GraphChain.Pieces))
		for i, p := range x.GraphChain.Pieces {
			for _, v := range p.Curve.Vertices {
				if v.Parameter.Lower.Cmp(v.Parameter.Upper) != 0 {
					return nil, geom.ErrConvergence
				}
				out[i] = append(out[i], v.Parameter.Lower)
			}
		}
		return out, nil
	}
	aa, e := extract(a)
	if e != nil {
		return nil, e
	}
	bb, e := extract(b)
	if e != nil {
		return nil, e
	}
	plan, e := PlanExactGraphChainPartition(ctx, aa, bb, ExactGraphChainPartitionOptions{MaxParameters: 4096})
	if e != nil {
		return nil, e
	}
	cuts := make([][]*big.Rat, len(plan.Pieces))
	for i, p := range plan.Pieces {
		cuts[i] = p.Parameters
	}
	return cuts, nil
}
