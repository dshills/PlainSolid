package trimmed

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep/edgecheck"
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

// FuseOrderedRoofBoxes builds the regularized union of two proved roof boxes.
// Disjoint footprints retain both original shells. For contained or identical
// footprints, single-function ordered roofs also qualify: a fully contained box
// keeps just its original container; matching bands can merge, or a strictly
// separating floor keeps both. Partial overlaps and unresolved height orders
// fail closed.
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
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}}
	for _, s := range []Solid{a, b} {
		if err := roofBoxCheckSourceEdges(ctx, s, &w); err != nil {
			return Solid{}, zero, fmt.Errorf("ordered roof Fuse source edges: %w", err)
		}
	}
	if footprintStrictlyDisjoint(d, boxes[1].domain) {
		return fuseSeparateOrderedRoofBoxes(ctx, a, b, &w)
	}
	if len(boxes[0].roofSpans) != 0 || len(boxes[1].roofSpans) != 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	if d != boxes[1].domain {
		out, h, err := fuseContainedOrderedRoofBoxes(ctx, [2]Solid{a, b}, boxes, &w)
		if !errors.Is(err, geom.ErrUnrepresentable) {
			return out, h, err
		}
		return fuseStackedOrderedRoofBoxes(ctx, a, b, boxes, &w)
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
			return fuseSeparateOrderedRoofBoxes(ctx, a, b, &w)
		}
	}
	order, err := orderedRoofBoxOrder(ctx, boxes, xy, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	if order < 0 {
		roofIndex = 1
	}
	upper, err := orderedRoofRestrictedGraph(ctx, boxes[roofIndex], d, &w)
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

// At a proved horizontal interface, the existing overlap assembler removes
// only the common cap area and reconciles every split with the incident
// original wall. Roof-box occupancy proves the whole low body lies below
// the plane and the whole high body above it before using that assembler.
func fuseStackedOrderedRoofBoxes(ctx context.Context, a, b Solid, boxes [2]*mixedRoofBox, w *mixedRoofLineWork) (Solid, RoofBoxUnionHistory, error) {
	var zero RoofBoxUnionHistory
	d := intersectRectangles(boxes[0].domain, boxes[1].domain)
	order := boxes[0].floor.Cmp(boxes[1].floor)
	if !d.Valid() || order == 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	lo := 0
	if order > 0 {
		lo = 1
	}
	low := boxes[lo]
	xy := [2]geom.ExactParameterInterval{
		{Lower: ratInset(low.domain.UMin), Upper: ratInset(low.domain.UMax)},
		{Lower: ratInset(low.domain.VMin), Upper: ratInset(low.domain.VMax)},
	}
	sign, err := orderedRoofAgainstFloor(ctx, low, xy, boxes[1-lo].floor, w)
	if err != nil {
		return Solid{}, zero, err
	}
	if sign != 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	remaining, err := roofBoxConstructionOptions(w)
	if err != nil {
		return Solid{}, zero, err
	}
	out, h, err := CombineOverlappingInterfaceSplineShells(ctx, a, b, parametric.Union, SplineShellJoinOptions{
		Embedding: SplineRectangleEmbeddingOptions{Patches: remaining},
		Sew:       SewOptions{Tolerance: max(a.tolerance, b.tolerance), Proof: edgecheck.Options{MaxWork: remaining.MaxOperations, MaxBits: remaining.MaxCoefficientBits}},
	})
	if err != nil {
		if errors.Is(err, geom.ErrConvergence) {
			return Solid{}, zero, fmt.Errorf("%w: stacked roof-box overlap: %w", geom.ErrUnrepresentable, err)
		}
		return Solid{}, zero, err
	}
	interfaceFaces := [2]int{h.InterfaceA, h.InterfaceB}
	if interfaceFaces[lo] != low.roofFace || interfaceFaces[1-lo] != boxes[1-lo].floorFace {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	if err := roofBoxCheckSourceEdges(ctx, out, w); err != nil {
		return Solid{}, zero, err
	}
	r := RoofBoxUnionHistory{Faces: make([][]MixedRoofFaceOwner, len(out.faces)), ExactOperations: w.operations}
	for operand, rows := range [2][][]int{h.FacesA, h.FacesB} {
		for face, fragments := range rows {
			for _, id := range fragments {
				r.Faces[id] = append(r.Faces[id], MixedRoofFaceOwner{operand, face})
			}
		}
	}
	return out, r, nil
}

func footprintStrictlyDisjoint(a, b geom.ParameterRectangle) bool {
	return a.UMax < b.UMin || b.UMax < a.UMin || a.VMax < b.VMin || b.VMax < a.VMin
}

func fuseSeparateOrderedRoofBoxes(ctx context.Context, a, b Solid, w *mixedRoofLineWork) (Solid, RoofBoxUnionHistory, error) {
	identity := geom.Identity()
	out, err := AssembleShells(ctx, []PlacedSolid{{a, identity}, {b, identity}}, 12)
	if err != nil {
		return Solid{}, RoofBoxUnionHistory{}, err
	}
	h := RoofBoxUnionHistory{ExactOperations: w.operations}
	for i, s := range []Solid{a, b} {
		for face := range s.faces {
			h.Faces = append(h.Faces, []MixedRoofFaceOwner{{i, face}})
		}
	}
	return out, h, nil
}

func fuseContainedOrderedRoofBoxes(ctx context.Context, solids [2]Solid, boxes [2]*mixedRoofBox, w *mixedRoofLineWork) (Solid, RoofBoxUnionHistory, error) {
	var zero RoofBoxUnionHistory
	d := intersectRectangles(boxes[0].domain, boxes[1].domain)
	outer := 0
	if d == boxes[0].domain {
		outer = 1
	}
	inner := 1 - outer
	if d != boxes[inner].domain || boxes[outer].floor.Cmp(boxes[inner].floor) > 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	xy := [2]geom.ExactParameterInterval{
		{Lower: ratInset(d.UMin), Upper: ratInset(d.UMax)},
		{Lower: ratInset(d.VMin), Upper: ratInset(d.VMax)},
	}
	sign, err := orderedRoofBoxOrder(ctx, [2]*mixedRoofBox{boxes[outer], boxes[inner]}, xy, w)
	if err != nil {
		return Solid{}, zero, err
	}
	if sign < 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	if err := ctx.Err(); err != nil {
		return Solid{}, zero, err
	}
	stock := solids[outer]
	h := RoofBoxUnionHistory{ExactOperations: w.operations, Faces: make([][]MixedRoofFaceOwner, len(stock.faces))}
	for i := range stock.faces {
		h.Faces[i] = []MixedRoofFaceOwner{{outer, i}}
	}
	return stock, h, nil
}
