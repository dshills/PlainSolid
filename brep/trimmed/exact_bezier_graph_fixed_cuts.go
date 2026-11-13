package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TessellateExactBezierGraphChainWithCutsCertified retains every prescribed
// normalized parameter, including 0 and 1 for each piece. Cuts must be strictly
// increasing; up to 2048 total occurrences and 4096-bit coefficients are allowed.
// Adaptive chord subdivision may add samples. Every chord and the whole polygon
// fill are independently recertified; a proposed partition is not proof evidence.
// Half of each piece display operation budget funds final tessellation; the
// other half is divided equally among N root imports and N-1 restrictions.
// Unused allocations are not transferred. Existing composition/chart budgets
// remain separate. Failure returns no partial face.
func TessellateExactBezierGraphChainWithCutsCertified(ctx context.Context, p *geom.SurfacePatch, pieces []ExactBezierUV, baseline *big.Rat, cuts [][]*big.Rat, o ExactBezierGraphFaceOptions) (CertifiedExactBezierGraphChain, error) {
	if len(cuts) != len(pieces) || len(cuts) == 0 {
		return CertifiedExactBezierGraphChain{}, geom.ErrInvalid
	}
	count := 0
	for _, c := range cuts {
		count += len(c)
		if count > 2048 {
			return CertifiedExactBezierGraphChain{}, geom.ErrConvergence
		}
	}
	for _, c := range cuts {
		if e := validateGraphCuts(c); e != nil {
			return CertifiedExactBezierGraphChain{}, e
		}
	}
	return tessellateExactBezierGraphChainCuts(ctx, p, pieces, baseline, cuts, o)
}

func validateGraphCuts(c []*big.Rat) error {
	if len(c) < 2 {
		return geom.ErrInvalid
	}
	for i, t := range c {
		if t == nil {
			return geom.ErrInvalid
		}
		if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
			return geom.ErrConvergence
		}
		if i > 0 && c[i-1].Cmp(t) >= 0 {
			return geom.ErrInvalid
		}
	}
	if c[0].Sign() != 0 || c[len(c)-1].Cmp(big.NewRat(1, 1)) != 0 {
		return geom.ErrInvalid
	}
	return nil
}

func tessellateGraphPrescribedCuts(ctx context.Context, cells geom.SurfaceCurveCompositionCells, cuts []*big.Rat, o geom.ComposedCollectionPolylineOptions) (geom.CertifiedComposedCollectionPolyline, error) {
	if cuts == nil {
		return cells.TessellateCertified(ctx, o)
	}
	var empty geom.CertifiedComposedCollectionPolyline
	if e := validateGraphCuts(cuts); e != nil {
		return empty, e
	}
	if len(cuts) > 2048 || len(o.JunctionPoints) != 0 {
		return empty, geom.ErrInvalid
	}
	budget := o.MaxOperations
	if budget == 0 {
		budget = 8000000
	}
	share := (budget / 2) / (2*len(cuts) - 1)
	if share < 1 {
		return empty, geom.ErrConvergence
	}
	proof := geom.ExactCurvePlaneEventOptions{MaxOperations: share, MaxCoefficientBits: 4096}
	roots := make([]*geom.ExactAlgebraicRoot, len(cuts))
	for i, t := range cuts {
		r, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{new(big.Rat).Neg(t).RatString(), "1"}, Lower: t.RatString(), Upper: t.RatString()}, geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: proof})
		if e != nil {
			return empty, e
		}
		roots[i] = r
	}
	refined := geom.SurfaceCurveCompositionCells{Complete: true}
	for i := 1; i < len(roots); i++ {
		part, e := cells.RestrictAlgebraic(ctx, roots[i-1], roots[i], proof)
		if e != nil {
			return empty, e
		}
		if !part.Complete || len(refined.Cells)+len(part.Cells) > 4096 {
			return empty, geom.ErrConvergence
		}
		refined.Cells = append(refined.Cells, part.Cells...)
	}
	o.MaxOperations = budget - budget/2
	return refined.TessellateCertified(ctx, o)
}
