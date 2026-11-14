package geom

import (
	"context"
	"math/big"
)

// ExactMaterialLineStation is an original rational line parameter represented
// exactly (possibly algebraic), never a rounded isolation bracket. Multiplicity
// zero marks a clip-only endpoint; support roots retain their multiplicity.
type ExactMaterialLineStation struct {
	Parameter    *ExactAlgebraicRoot
	Location     BorePointLocation
	Boundary     BoreBoundaryMask
	Multiplicity int
}

// ExactMaterialLineSpan classifies the OPEN interval between adjacent stations.
// Endpoint membership is separate; e.g. an exterior tangent is not material.
type ExactMaterialLineSpan struct {
	Start, End int
	Location   BorePointLocation
	Boundary   BoreBoundaryMask
}

// ExactMaterialLinePartition covers the whole requested finite range. Even
// trimmed-away support roots remain stations, with no exposed boundary owner.
// Boundary retains the legacy exposed hit/coincident-interval ledger.
type ExactMaterialLinePartition struct {
	Stations []ExactMaterialLineStation
	Spans    []ExactMaterialLineSpan
	Boundary BoreLineBoundary
}

// PartitionRationalLine proves constant original polynomial signs on each
// root-free gap. It performs one isolation pass. Exact rational evaluation in
// a proved gap establishes its sign; it does not sample geometric approximations.
// Polynomial work is cumulative. Root compare/enclosure retain the existing
// per-call limits: at most ten stations, 45 ordering comparisons and
// 18*MaxSubdivisions endpoint-enclosure calls, plus bounded root imports.
// Any failure returns an empty partition, including cancellation.
func (s ObliqueCylinderTorusCavityMaterial) PartitionRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (ExactMaterialLinePartition, error) {
	var out ExactMaterialLinePartition
	b, e := s.partitionRationalLine(ctx, p, d, min, max, o, false, &out)
	if e != nil {
		return ExactMaterialLinePartition{}, e
	}
	out.Boundary = BoreLineBoundary{Hits: b.Points, Intervals: b.Intervals}
	return out, nil
}
