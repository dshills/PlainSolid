package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// FiniteCylinderPairUnionResult retains the analytic union and raw face
// ancestry. Crossing and disjoint constructions report an exact squared
// common volume. Containment instead reports its exact coefficient of π.
// The two proof fields are mutually exclusive.
type FiniteCylinderPairUnionResult struct {
	Solid                                          Solid
	WallFaces, LowerCapFaces, UpperCapFaces        [2][]int
	CommonVolumeSquared, CommonVolumePiCoefficient *big.Rat
}

// FiniteCylinderPairUnion retains the native crossing equal-radius union of
// raw finite cylinders, two strictly separated coordinate-parallel bodies,
// and selected unequal-radius perpendicular pairs with clear symmetric caps
// or complete containment.
// A crossing requires representable crossing coordinates and recentered caps
// that clear the common. Wall and cap ancestry is in original operand order.
func FiniteCylinderPairUnion(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (FiniteCylinderPairUnionResult, error) {
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return FiniteCylinderPairUnionResult{}, geom.ErrInvalid
	}
	if _, err := geom.NewCylinderPairCommon(a, b); err != nil {
		return FiniteCylinderPairUnionResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return FiniteCylinderPairUnionResult{}, err
	}
	if rawAxesParallel(a.Axis, b.Axis) {
		if contained := parallelCoordinateContainment(a, b); contained[0] || contained[1] {
			return unionContainedParallelCylinder(ctx, a, b, contained)
		}
		return unionSeparatedCoordinateCylinders(ctx, a, b)
	}
	if a.Radius != b.Radius {
		result, ok, err := finiteContainedCylinderUnion(ctx, a, b)
		if err != nil || ok {
			return result, err
		}
		return finiteUnequalPerpendicularUnion(ctx, a, b, tolerance)
	}
	center, caps, err := finiteCylinderCrossingFrame(ctx, a, b, tolerance)
	if err != nil {
		return FiniteCylinderPairUnionResult{}, err
	}
	r, err := ObliqueEqualCylinderUnion(ctx, center, a.Axis, b.Axis, a.Radius, caps[0], caps[1], tolerance)
	if err != nil {
		return FiniteCylinderPairUnionResult{}, err
	}
	return FiniteCylinderPairUnionResult{Solid: r.Solid, WallFaces: r.WallFaces, LowerCapFaces: r.LowerCapFaces, UpperCapFaces: r.UpperCapFaces, CommonVolumeSquared: r.CommonVolumeSquared}, nil
}

// FiniteCylinderPairDifferenceResult preserves the retained stock components
// in raw-axis order, with unused slots empty. A split equal-radius result
// touches at ContactNodes; a split wider cut has a positive MinimumGapSquared.
// Face IDs are local to the corresponding solid. A nil proof means its case
// does not apply.
type FiniteCylinderPairDifferenceResult struct {
	Solids                                         [2]Solid
	StockWallFaces, CutterWallFaces, StockCapFaces [2][]int
	ContactNodes                                   *[2]geom.EqualCylinderBisectorNode
	MinimumGapSquared                              *big.Rat
}

// FiniteCylinderPairDifference cuts a crossing equal-radius or wider cutter
// from finite stock under strict cap clearance. It retains a single bored
// stock for selected narrower perpendicular or coaxial cutters and keeps the
// stock unchanged when coordinate-parallel cylinders are strictly separated.
// It returns empty when exact perpendicular or coordinate-parallel containment
// covers the stock.
// Other arrangements fail without publishing a partial solid.
func FiniteCylinderPairDifference(ctx context.Context, stock, cutter geom.RawCylinder, tolerance float64) (FiniteCylinderPairDifferenceResult, error) {
	var out FiniteCylinderPairDifferenceResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return out, geom.ErrInvalid
	}
	if _, err := geom.NewCylinderPairCommon(stock, cutter); err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if rawAxesParallel(stock.Axis, cutter.Axis) {
		if parallelCoordinateContainment(stock, cutter)[0] {
			return out, nil
		}
		if !parallelCylinderSupportsSeparated(stock, cutter, true) {
			bore, err := FiniteCoaxialCylinderThroughDifference(ctx, stock, cutter, tolerance)
			if err != nil {
				return out, err
			}
			out.Solids[0], out.StockWallFaces[0], out.CutterWallFaces[0] = bore.Solid, bore.StockWallFaces, bore.CutterWallFaces
			out.StockCapFaces[0] = append(bore.LowerCapFaces, bore.UpperCapFaces...)
			return out, nil
		}
		s, err := nativeCoordinateCylinder(stock)
		if err != nil {
			return out, err
		}
		out.Solids[0], out.StockWallFaces[0], out.StockCapFaces[0] = s, []int{0}, []int{1, 2}
		return out, nil
	}
	if stock.Radius < cutter.Radius {
		if owner, ok, err := finiteRawPerpendicularContainment(ctx, stock, cutter); err != nil {
			return out, err
		} else if ok && owner == 0 {
			return out, nil
		}
	}
	if stock.Radius > cutter.Radius {
		return finiteUnequalPerpendicularDifference(ctx, stock, cutter, tolerance)
	}
	center, caps, err := finiteCylinderCrossingFrame(ctx, stock, cutter, tolerance)
	if err != nil {
		return out, err
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
