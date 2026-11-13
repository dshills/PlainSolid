package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// FiniteCylinderPairUnion retains the native crossing equal-radius union of
// raw finite cylinders. It requires representable crossing coordinates and
// raw cap limits recentered about that crossing. Caps must clear the common.
// Wall and cap ancestry remains in original operand and raw parameter order.
func FiniteCylinderPairUnion(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (ObliqueEqualCylinderUnionResult, error) {
	center, caps, err := finiteCylinderCrossingFrame(ctx, a, b, tolerance)
	if err != nil {
		return ObliqueEqualCylinderUnionResult{}, err
	}
	if a.Radius != b.Radius {
		return ObliqueEqualCylinderUnionResult{}, geom.ErrDomain
	}
	return ObliqueEqualCylinderUnion(ctx, center, a.Axis, b.Axis, a.Radius, caps[0], caps[1], tolerance)
}

// FiniteCylinderPairDifferenceResult preserves the two stock-side components
// ordered by the first cylinder's original raw axis. For equal radii the
// closures touch at ContactNodes; otherwise MinimumGapSquared bounds their
// separation from below. Face IDs are local to the corresponding solid.
// A nil contact or gap report means that case does not apply.
type FiniteCylinderPairDifferenceResult struct {
	Solids                                         [2]Solid
	StockWallFaces, CutterWallFaces, StockCapFaces [2][]int
	ContactNodes                                   *[2]geom.EqualCylinderBisectorNode
	MinimumGapSquared                              *big.Rat
}

// FiniteCylinderPairDifference cuts a crossing equal-radius or wider cutter
// from finite stock. The same strict cap-clearance and representability
// requirements as the native analytic constructors apply. Other arrangements
// fail without publishing a partial solid.
func FiniteCylinderPairDifference(ctx context.Context, stock, cutter geom.RawCylinder, tolerance float64) (FiniteCylinderPairDifferenceResult, error) {
	var out FiniteCylinderPairDifferenceResult
	center, caps, err := finiteCylinderCrossingFrame(ctx, stock, cutter, tolerance)
	if err != nil {
		return out, err
	}
	if stock.Radius > cutter.Radius {
		return out, geom.ErrDomain
	}
	if stock.Radius == cutter.Radius {
		r, e := ObliqueEqualCylinderDifference(ctx, center, stock.Axis, cutter.Axis, stock.Radius, caps[0], caps[1], tolerance)
		if e != nil {
			return out, e
		}
		return FiniteCylinderPairDifferenceResult{Solids: r.Solids, StockWallFaces: r.StockWallFaces, CutterWallFaces: r.CutterWallFaces, StockCapFaces: r.StockCapFaces, ContactNodes: &r.ContactNodes}, nil
	}
	r, e := ObliqueCylinderSplitCut(ctx, center, stock.Axis, cutter.Axis, stock.Radius, cutter.Radius, caps[0], caps[1], tolerance)
	if e != nil {
		return out, e
	}
	return FiniteCylinderPairDifferenceResult{Solids: r.Solids, StockWallFaces: r.StockWallFaces, CutterWallFaces: r.CutterWallFaces, StockCapFaces: r.StockCapFaces, MinimumGapSquared: r.MinimumGapSquared}, nil
}

func finiteCylinderCrossingFrame(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (geom.Vec3, [2][2]float64, error) {
	var center geom.Vec3
	var caps [2][2]float64
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return center, caps, geom.ErrInvalid
	}
	if _, e := geom.NewCylinderPairCommon(a, b); e != nil {
		return center, caps, e
	}
	if e := ctx.Err(); e != nil {
		return center, caps, e
	}
	if rawAxesParallel(a.Axis, b.Axis) {
		return center, caps, geom.ErrDomain
	}
	m, e := geom.NewObliqueSkewCylinderMap(ctx, a.Origin, a.Axis, a.Radius, b.Origin, b.Axis, b.Radius, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return center, caps, e
	}
	if m.SeparationSign != 0 {
		return center, caps, geom.ErrDomain
	}
	for i, p := range [3]*float64{&center.X, &center.Y, &center.Z} {
		v, e := geom.RepresentedExactScalar(m.ClosestFeet[0][i])
		if e != nil {
			return geom.Vec3{}, [2][2]float64{}, e
		}
		*p = v
	}
	for i, c := range [2]geom.RawCylinder{a, b} {
		for j, x := range [2]float64{c.Min, c.Max} {
			v, e := geom.RepresentedExactScalar(new(big.Rat).Sub(new(big.Rat).SetFloat64(x), m.ClosestRawParameters[i]))
			if e != nil {
				return geom.Vec3{}, [2][2]float64{}, e
			}
			caps[i][j] = v
		}
	}
	return center, caps, nil
}
