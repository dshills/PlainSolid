package trimmed

import (
	"context"
	"fmt"

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
// identical XY footprints and single-span roofs. When the bands overlap or
// meet everywhere in a proved roof order, it sews four merged walls to the
// higher roof and lower floor. A strictly separating floor preserves both
// original, disconnected shells. Gaps limited to part of the footprint and
// unresolved height orders fail closed.
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
	floorIndex, roofIndex := 0, 0
	if boxes[1].floor.Cmp(boxes[0].floor) < 0 {
		floorIndex = 1
	}
	// Equal floors guarantee overlap. Otherwise the lower-floor box's
	// roof must reach the higher floor everywhere.
	if boxes[0].floor.Cmp(boxes[1].floor) != 0 {
		sign, strict, err := orderedRoofAgainstFloorDetail(ctx, boxes[floorIndex], xy, boxes[1-floorIndex].floor, &w)
		if err != nil {
			return Solid{}, zero, err
		}
		if sign < 0 {
			if !strict {
				return Solid{}, zero, geom.ErrUnrepresentable
			}
			identity := geom.Identity()
			out, err := AssembleShells(ctx, []PlacedSolid{{a, identity}, {b, identity}}, 12)
			if err != nil {
				return Solid{}, zero, err
			}
			h := RoofBoxUnionHistory{ExactOperations: w.operations}
			for i, s := range []Solid{a, b} {
				for face := range s.faces {
					h.Faces = append(h.Faces, []MixedRoofFaceOwner{{i, face}})
				}
			}
			return out, h, nil
		}
	}
	order, err := orderedRoofBoxOrder(ctx, boxes, xy, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	if order < 0 {
		roofIndex = 1
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
