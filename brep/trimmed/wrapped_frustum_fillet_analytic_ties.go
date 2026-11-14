package trimmed

import (
	"context"
	"math/big"
	"sort"
)

// FilletedFrustumAnalyticNearestFaces retains the exact algebraic witness in
// Proof, without rounding it to a rational point. Tied contains a proved
// attaining original face only for supported interior cone/torus/bore cases.
// Excluded requires a STRICT lower-bound separation from the witness upper
// enclosure. Overlap/equality is Unresolved, never a guessed tie. Complete
// classifies every original face, not uniqueness of the nearest point.
type FilletedFrustumAnalyticNearestFaces struct {
	Proof                      FilletedFrustumClosestBoundary
	Tied, Excluded, Unresolved []int
	Complete                   bool
}

// ClassifyAnalyticNearestFacesRational performs one existing bounded closest
// query and a bounded scan of its original all-support ledger. It can certify
// a complete owner set even for irrational squared distance, with no general
// algebraic equality test. Rational fallback, rims and unproved nearest cases
// intentionally retain all owners as Unresolved; use ClassifyNearestFacesRational
// for rational attained tie classification. Every error is atomic.
func (q FilletedFrustumMaterial) ClassifyAnalyticNearestFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (out FilletedFrustumAnalyticNearestFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumAnalyticNearestFaces{}
		}
	}()
	proof, e := q.ClosestBoundaryRational(ctx, p, o)
	if e != nil {
		return out, e
	}
	out.Proof = proof
	supported := proof.NearestProven && len(proof.Faces) == 1 && (proof.Kind == FilletedFrustumClosestCone || proof.Kind == FilletedFrustumClosestToroid || proof.Kind == FilletedFrustumClosestNestedToroid || proof.Kind == FilletedFrustumClosestBore)
	ledger := proof.Proof.Baseline.Baseline.Baseline.Baseline.PerFace
	for _, f := range ledger {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if !supported {
			out.Unresolved = append(out.Unresolved, f.Face)
			continue
		}
		if f.Face == proof.Faces[0] {
			out.Tied = append(out.Tied, f.Face)
		} else if f.SquaredLower.Cmp(proof.Upper) > 0 {
			out.Excluded = append(out.Excluded, f.Face)
		} else {
			out.Unresolved = append(out.Unresolved, f.Face)
		}
	}
	sort.Ints(out.Tied)
	sort.Ints(out.Excluded)
	sort.Ints(out.Unresolved)
	out.Complete = supported && len(out.Tied) == 1 && len(out.Unresolved) == 0
	return out, nil
}
