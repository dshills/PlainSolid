package fillet

import (
	"context"
	"plainsolid/brep/trimmed"
)

// LongEdgeChain retains the actual X-long horizontal-edge-treated stock.
// Original IDs remain stable. Only top/bottom cap charts are replaced by exact
// XY charts on the same planes, with unchanged outer model edges and added
// feature wires. Generated face arrays follow donor footprint traversal order.
type LongEdgeChain struct {
	Solid                             trimmed.Solid
	Stock                             LongEdgeTreatment
	Bore                              trimmed.ExtrusionCircle
	Pocket                            trimmed.ExtrusionRectangle
	Boss                              trimmed.ExtrusionBoss
	PocketFaces, BoreFaces            [4]int
	BossFaces                         [5]int
	NominalVolume, NominalSurfaceArea float64
}

// TreatLongCapEdgesWithFeatures composes one Z-through circular bore, one
// rectangular Z-through pocket and one upper rectangular boss with the actual
// horizontal fillet/chamfer stock. Every footprint must lie strictly in the
// retained flat top strip, with eight tolerances of clearance. Exact represented
// footprint separation is re-proved by the donor constructor. No curved stock
// face is replaced, and no bounding-box substitute is used. Failure is atomic.
func TreatLongCapEdgesWithFeatures(ctx context.Context, length, width, height, radius, topSetback, sideSetback float64, bore trimmed.ExtrusionCircle, pocket trimmed.ExtrusionRectangle, boss trimmed.ExtrusionBoss, o Options) (LongEdgeChain, error) {
	if radius <= 0 || topSetback <= 0 || sideSetback <= 0 {
		return LongEdgeChain{}, ErrInvalid
	}
	return TreatLongCapEdgesOptional(ctx, length, width, height, radius, topSetback, sideSetback, false, &bore, &pocket, &boss, o)
}
