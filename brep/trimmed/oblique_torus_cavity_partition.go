//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// NativeObliqueCavityPartition retains the complete exact finite line partition
// and native incidence at every station/open span. Empty owner lists denote no
// exposed original boundary, including material interior and exterior gaps.
type NativeObliqueCavityPartition struct {
	geom.ExactMaterialLinePartition
	StationFaces, SpanFaces [][]int
	OwnershipOperations     int
}

// PartitionRationalLine binds all original boundary owners to one exact source
// partition. Source isolation and cumulative native sign proofs have separate
// bounds in o; any failure returns an empty result.
func (p *PreparedObliqueTorusCavity) PartitionRationalLine(ctx context.Context, origin, direction [3]*big.Rat, min, max *big.Rat, o NativeObliqueCavityLineOptions) (NativeObliqueCavityPartition, error) {
	var zero NativeObliqueCavityPartition
	if p == nil || p.major == nil {
		return zero, geom.ErrInvalid
	}
	section, e := p.source.PartitionRationalLine(ctx, origin, direction, min, max, o.Line)
	if e != nil {
		return zero, e
	}
	// Bind existing native chart sign proofs to the same immutable roots; do not
	// isolate the source a second time. Masks on nonboundary stations are empty.
	var ledger geom.BoreLineBoundary
	for _, s := range section.Stations {
		ledger.Hits = append(ledger.Hits, geom.BoreLineHit{Parameter: s.Parameter, Boundary: s.Boundary})
	}
	for _, s := range section.Spans {
		a, b := section.Stations[s.Start], section.Stations[s.End]
		ledger.Intervals = append(ledger.Intervals, geom.BoreBoundaryInterval{Start: a.Parameter, End: b.Parameter, Boundary: s.Boundary, StartBoundary: 0, EndBoundary: 0})
	}
	mapped, e := p.bindLineBoundary(ctx, origin, direction, min, max, o, &ledger)
	if e != nil {
		return zero, e
	}
	return NativeObliqueCavityPartition{ExactMaterialLinePartition: section, StationFaces: mapped.HitFaces, SpanFaces: mapped.IntervalFaces, OwnershipOperations: mapped.OwnershipOperations}, nil
}

// PartitionPlacedRationalLine preserves t, material states and original owners.
// The two exact inverse transforms have separate 4096-bit bounds.
func (p *PreparedObliqueTorusCavity) PartitionPlacedRationalLine(ctx context.Context, pose placement.RationalRigid, origin, direction [3]*big.Rat, min, max *big.Rat, o NativeObliqueCavityLineOptions) (NativeObliqueCavityPartition, error) {
	a, e := pose.TransformPoint(ctx, origin, true, 4096)
	if e != nil {
		return NativeObliqueCavityPartition{}, e
	}
	d, e := pose.TransformVector(ctx, direction, true, 4096)
	if e != nil {
		return NativeObliqueCavityPartition{}, e
	}
	return p.PartitionRationalLine(ctx, a, d, min, max, o)
}
