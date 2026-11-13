package geom

import (
	"context"
	"math/big"
)

// CollectionCircleStationaryEvent uniquely owns a stationary original parameter
// occurrence, retaining each incident stationary cell germ. A zero side means no
// stationary germ was supplied on that side; it does not assert a regular
// neighbor's angular speed vanishes. Reverses requires both strict side signs.
// Distinct domain endpoints are not welded even if their model images agree.
type CollectionCircleStationaryEvent struct {
	Event            *ExactComposedCurveEvent
	Cells            []int
	Local            []CircleStationaryEvent
	SideOrientations [2]int
	Reverses         bool
}

// StationaryCircleContactComponents adds angular stationary events without
// reporting them as isolated circle intersections. Component Pieces partition
// coincident cells in exact original parameter order, retaining source indices.
type StationaryCircleContactComponents struct {
	CircleContactComponents
	Stationary []CollectionCircleStationaryEvent
}

// ClassifyStationaryPlanarCircleComponents opts into finite angular stationary
// points and folds of coincident cells. Constant continuations remain unresolved.
// Exact seams are reverified; shared stationary endpoint roots are merged only
// for adjacent coincident cells. Limits match ClassifyPlanarCircleComponents,
// with raw stationary roots additionally charged to MaxEvents before merging.
// The strict API continues to reject any stationary coincident cell.
func (c SurfaceCurveCompositionCells) ClassifyStationaryPlanarCircleComponents(ctx context.Context, axes [2]int, center [3]*big.Rat, radiusSquared *big.Rat, o ComposedPlaneContactsOptions) (StationaryCircleContactComponents, error) {
	return c.circleContactComponents(ctx, axes, center, radiusSquared, o, true, false)
}
