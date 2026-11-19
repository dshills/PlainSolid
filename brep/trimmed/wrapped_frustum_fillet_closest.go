//go:build research

package trimmed

import (
	"context"
	"math/big"
)

// FilletedFrustumClosestKind names the exact representation of the selected
// retained upper witness. It does not assert global nearest or uniqueness.
type FilletedFrustumClosestKind uint8

const (
	FilletedFrustumClosestNone FilletedFrustumClosestKind = iota
	FilletedFrustumClosestRational
	FilletedFrustumClosestRim
	FilletedFrustumClosestToroid
	FilletedFrustumClosestNestedToroid
	FilletedFrustumClosestCone
	FilletedFrustumClosestBore
)

// FilletedFrustumClosestBoundary is a tagged view of one complete query.
// Bore selects BoreProof.Bore; other kinds select a witness in Proof:
// Cone, Baseline.Nested,
// Baseline.Baseline.Toroid, Baseline.Baseline.Baseline.Rim, or the final
// rational Baseline.Witness. Faces are original source owners. Converged
// means only that the squared interval meets tolerance; NearestProven alone
// certifies the selected exact witness as globally nearest (not unique).
// All data are result-owned; Proof and its tagged view share no source data.
type FilletedFrustumClosestBoundary struct {
	Proof                    FilletedFrustumConeDistance  // compatibility: pre-bore proof
	BoreProof                *FilletedFrustumBoreDistance // complete query, including selected Bore
	Kind                     FilletedFrustumClosestKind
	Faces                    []int
	Lower, Upper             *big.Rat
	NearestProven, Converged bool
}

// ClosestBoundaryRational runs the existing bounded support, rim, torus and
// cone/bore chain once (at most16 additional bore checks). Candidates replace their predecessor only when their
// certified upper bound is smaller. Unresolved lower bounds never promote
// a candidate to a nearest proof; no algebraic tie/uniqueness claim is made.
func (q FilletedFrustumMaterial) ClosestBoundaryRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (FilletedFrustumClosestBoundary, error) {
	proof, e := q.BoundBoundaryBoreDistanceRational(ctx, p, FilletedFrustumBoreDistanceOptions{Original: o})
	if e != nil {
		return FilletedFrustumClosestBoundary{}, e
	}
	kind, faces := filletedClosestSelection(proof.Baseline)
	if proof.Bore != nil {
		kind, faces = FilletedFrustumClosestBore, []int{proof.Bore.Face}
	}
	return FilletedFrustumClosestBoundary{Proof: proof.Baseline, BoreProof: &proof, Kind: kind, Faces: faces, Lower: new(big.Rat).Set(proof.Lower), Upper: new(big.Rat).Set(proof.Upper), NearestProven: proof.NearestProven, Converged: proof.Converged}, nil
}

func filletedClosestSelection(p FilletedFrustumConeDistance) (FilletedFrustumClosestKind, []int) {
	if p.Cone != nil {
		return FilletedFrustumClosestCone, []int{p.Cone.Face}
	}
	n := p.Baseline
	if n.Nested != nil {
		return FilletedFrustumClosestNestedToroid, []int{n.Nested.Face}
	}
	t := n.Baseline
	if t.Toroid != nil {
		return FilletedFrustumClosestToroid, []int{t.Toroid.Face}
	}
	r := t.Baseline
	if r.Rim != nil {
		return FilletedFrustumClosestRim, append([]int(nil), r.Rim.Faces...)
	}
	return FilletedFrustumClosestRational, append([]int(nil), r.Baseline.WitnessFaces...)
}

// FilletedFrustumOccurrenceClosestBoundary tags the corresponding world-space
// witness in BoreProof.WorldBore, otherwise Proof.WorldCone or its fallback. Original proofs
// remain explicitly local. Rigid invariance preserves bounds and owner IDs.
type FilletedFrustumOccurrenceClosestBoundary struct {
	Proof                    FilletedFrustumOccurrenceConeDistance  // fallback only when no bore selected
	BoreProof                *FilletedFrustumOccurrenceBoreDistance // complete world query
	Kind                     FilletedFrustumClosestKind
	Faces                    []int
	Lower, Upper             *big.Rat
	NearestProven, Converged bool
}

// ClosestBoundaryRational performs one original query and one symbolic world
// placement of its selected witness, retaining conservative fallback status.
func (q FilletedFrustumOccurrence) ClosestBoundaryRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (FilletedFrustumOccurrenceClosestBoundary, error) {
	proof, e := q.BoundBoundaryBoreDistanceRational(ctx, p, FilletedFrustumBoreDistanceOptions{Original: o})
	if e != nil {
		return FilletedFrustumOccurrenceClosestBoundary{}, e
	}
	kind, faces := filletedClosestSelection(proof.Original.Baseline)
	if proof.WorldBore != nil {
		kind, faces = FilletedFrustumClosestBore, []int{proof.WorldBore.Face}
	}
	return FilletedFrustumOccurrenceClosestBoundary{Proof: proof.Fallback, BoreProof: &proof, Kind: kind, Faces: faces, Lower: new(big.Rat).Set(proof.Lower), Upper: new(big.Rat).Set(proof.Upper), NearestProven: proof.NearestProven, Converged: proof.Converged}, nil
}
