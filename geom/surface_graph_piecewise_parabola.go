package geom

import (
	"context"
	"math/big"
)

// PiecewiseParabolaOptions bounds the complete union of original knot slabs.
// MaxPieces defaults to 8 and cannot exceed 32. Proof quotas are cumulative,
// including restriction, full slab proofs and exact one-sided join evaluation.
// MaxCells cumulatively counts difference cells and injectivity cell visits.
type PiecewiseParabolaOptions struct {
	Proof               GraphPointContactOptions
	FreeAxis, MaxPieces int
}

// ParabolaKnotJoin retains a shared original UV/model point and separate
// one-sided model tangents in increasing original free-coordinate units.
// Tangents need not agree across a C0 knot. Both owners independently agree
// with each listed side. No smooth continuation across the knot is asserted.
type ParabolaKnotJoin struct {
	UV       [2]*big.Rat
	Point    [3]*big.Rat
	Tangents [2][3]*big.Rat
}

// PiecewiseSplineGraphParabola is a complete contact report, not a native
// edge recipe. Each piece retains its original domain and knot-cell evidence.
// Internal terminal masks denote slab edges, not original outer boundaries.
// PositiveSide is the common sign toward increasing dependent coordinate.
type PiecewiseSplineGraphParabola struct {
	Proven                 bool
	Domain                 ParameterRectangle
	FreeAxis, PositiveSide int
	Pieces                 []SplineGraphParabola
	Joins                  []ParabolaKnotJoin
	ExactOperations        int
}

// DiscoverPiecewiseSplineGraphParabola proves a continuous open contact whose
// quadratic formula may change at original knots. Every original free-axis
// knot is retained, including redundant refinement knots. At least two slabs
// are required. All slabs cover the entire dependent range, have strict
// Bernstein boundary clearance, and independently prove all original cells.
// Failed, canceled or exhausted discovery publishes no partial contact.
func DiscoverPiecewiseSplineGraphParabola(ctx context.Context, a, b *SurfacePatch, o PiecewiseParabolaOptions) (PiecewiseSplineGraphParabola, error) {
	r, e := discoverPiecewiseGraph(ctx, a, b, o, DiscoverSplineGraphParabolaOnAxis, func(p SplineGraphParabola) graphPieceDescription {
		return graphPieceDescription{p.Power[:], p.Controls[:], p.PositiveSide, len(p.CertifiedCells) + p.Graph.VisitedCells, p.ExactOperations}
	})
	return PiecewiseSplineGraphParabola{Proven: r.Proven, Domain: r.Domain, FreeAxis: r.FreeAxis, PositiveSide: r.PositiveSide, Pieces: r.Pieces, Joins: r.Joins, ExactOperations: r.ExactOperations}, e
}
