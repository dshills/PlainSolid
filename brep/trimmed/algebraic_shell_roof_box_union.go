package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RoofBoxUnionHistory lists the original sources of each output face in
// result order. Roof and floor ties select the first operand; a merged wall
// names both sources because it is the union of their original wall regions.
type RoofBoxUnionHistory struct {
	Faces           [][]MixedRoofFaceOwner
	ExactOperations int
}

// FuseOrderedRoofBoxes builds the regularized union of two roof boxes with
// identical XY footprints, single-span roofs in a provable global height
// order, and vertical bands that overlap or meet at every XY point. It
// preserves the higher roof as one exact chart and sews four merged walls
// to the lower floor. A footprint mismatch, unresolved order, or vertical
// gap does not qualify for this single-band construction.
func FuseOrderedRoofBoxes(ctx context.Context, a, b Solid, proof MixedRoofQueryOptions, options geom.SurfacePatchEvaluationOptions) (Solid, RoofBoxUnionHistory, error) {
	var zero RoofBoxUnionHistory
	q, err := prepareDirectRoofBoxMeasure(ctx, a, b, parametric.Union, proof)
	if err != nil {
		return Solid{}, zero, err
	}
	o, err := q.evaluationOptions(ctx, options)
	if err != nil {
		return Solid{}, zero, err
	}
	if o.MaxOperations < 8 { // minimum allowance for the two exact footprints
		return Solid{}, zero, geom.ErrConvergence
	}
	boxes := q.box
	d := boxes[0].domain
	if d != boxes[1].domain || len(boxes[0].roofSpans) != 0 || len(boxes[1].roofSpans) != 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}}
	for _, s := range []Solid{a, b} {
		if err := roofBoxCheckSourceEdges(ctx, s, &w); err != nil {
			return Solid{}, zero, fmt.Errorf("ordered roof Fuse source edges: %w", err)
		}
	}
	xy := [2]geom.ExactParameterInterval{
		{Lower: ratInset(d.UMin), Upper: ratInset(d.UMax)},
		{Lower: ratInset(d.VMin), Upper: ratInset(d.VMax)},
	}
	order, err := orderedRoofBoxOrder(ctx, boxes, xy, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	floorIndex, roofIndex := 0, 0
	if boxes[1].floor.Cmp(boxes[0].floor) < 0 {
		floorIndex = 1
	}
	if order < 0 {
		roofIndex = 1
	}
	// Equal floors guarantee overlap. Otherwise the lower-floor box's
	// roof must reach the higher floor everywhere.
	if boxes[0].floor.Cmp(boxes[1].floor) != 0 {
		axes := [2][2]*big.Rat{{xy[0].Lower, xy[0].Upper}, {xy[1].Lower, xy[1].Upper}}
		numerator, weight, err := boxes[floorIndex].roofOrderPower(axes, &w)
		if err != nil {
			return Solid{}, zero, err
		}
		// Compare physical height to the other floor after multiplication
		// by the strictly positive roof denominator. A weak sign suffices:
		// even an interior tangency to the floor leaves no union gap.
		gap, err := roofHeightDifference(
			[2]roofPowerGrid{numerator, {{boxes[1-floorIndex].floor}}},
			[2]roofPowerGrid{weight, {{big.NewRat(1, 1)}}}, &w)
		if err != nil {
			return Solid{}, zero, err
		}
		unit := [2]geom.ExactParameterInterval{
			{Lower: new(big.Rat), Upper: big.NewRat(1, 1)},
			{Lower: new(big.Rat), Upper: big.NewRat(1, 1)},
		}
		bounds, err := mixedRoofPowerRange(gap, unit, &w)
		if err != nil {
			return Solid{}, zero, err
		}
		sign, err := orderedRoofPowerSign(ctx, gap, bounds, &w)
		if err != nil {
			return Solid{}, zero, err
		}
		if sign < 0 {
			return Solid{}, zero, geom.ErrUnrepresentable
		}
	}
	upper, err := roofBoxRestrictedPatch(boxes[roofIndex].roof, d, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	lower, err := orderedRoofFlatFloor(ctx, d, xy, boxes[floorIndex].floor, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	out, err := assembleOrderedRoofBox(ctx, upper, lower, d, max(a.tolerance, b.tolerance), &w)
	if err != nil {
		return Solid{}, zero, fmt.Errorf("ordered roof Fuse assembly: %w", err)
	}
	h := RoofBoxUnionHistory{Faces: [][]MixedRoofFaceOwner{
		{{roofIndex, boxes[roofIndex].roofFace}},
		{{floorIndex, boxes[floorIndex].floorFace}},
	}, ExactOperations: w.operations}
	for i := range 4 {
		h.Faces = append(h.Faces, []MixedRoofFaceOwner{{0, boxes[0].sideFaces[i]}, {1, boxes[1].sideFaces[i]}})
	}
	return out, h, nil
}
