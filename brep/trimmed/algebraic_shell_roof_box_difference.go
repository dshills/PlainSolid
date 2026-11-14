package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SubtractOrderedRoofBoxes handles the case where the cutter covers the
// stock's entire XY rectangle. When the cutter starts at or below the
// stock floor, its top must be proved entirely below, entirely above, or
// strictly between the stock's floor and roof. A cutter with its floor
// strictly inside the stock may remove the whole top or leave an upper
// band as a second component. The original face ancestry follows the
// output face order; a retained cutter floor or roof reverses.
// Each participating roof must be a single rational Bézier span on the
// stock rectangle. Pinches and unresolved height order fail closed.
func SubtractOrderedRoofBoxes(ctx context.Context, stock, cutter Solid, proof MixedRoofQueryOptions, options geom.SurfacePatchEvaluationOptions) (Solid, RoofBoxDifferenceHistory, error) {
	var zero RoofBoxDifferenceHistory
	q, err := prepareDirectRoofBoxMeasure(ctx, stock, cutter, parametric.Difference, proof)
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
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}}
	for _, src := range []Solid{stock, cutter} {
		if err := roofBoxCheckSourceEdges(ctx, src, &w); err != nil {
			return Solid{}, zero, fmt.Errorf("roof box source edges: %w", err)
		}
	}
	a, b := q.box[0], q.box[1]
	d := a.domain
	if intersectRectangles(d, b.domain) != d || len(a.roofSpans) > 0 || len(b.roofSpans) > 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	upper, err := roofBoxRestrictedPatch(a.roof, d, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	lower, err := roofBoxRestrictedPatch(b.roof, d, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	xy := [2]geom.ExactParameterInterval{{Lower: ratInset(d.UMin), Upper: ratInset(d.UMax)}, {Lower: ratInset(d.VMin), Upper: ratInset(d.VMax)}}
	if b.floor.Cmp(a.floor) > 0 {
		return subtractFloatingRoofBox(ctx, stock, cutter, q.box, xy, upper, lower, &w)
	}
	sign, err := orderedRoofAgainstFloor(ctx, b, xy, a.floor, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	if sign <= 0 {
		// Subtraction can remove only a boundary subset. Regularization
		// retains the whole authenticated source stock and its native IDs.
		return unchangedOrderedRoofBox(ctx, stock, &w)
	}
	cache := make(roofOrderCache)
	order, err := cache.order(q.box, xy, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	sign, strict, err := orderedRoofPowerSign(ctx, cache[q.box].power, order, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	if sign <= 0 {
		return Solid{}, RoofBoxDifferenceHistory{ExactOperations: w.operations}, ctx.Err()
	}
	// Contact with the stock roof would pinch the sewn material band.
	if !strict {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	if err := certifyRoofStrictlyAboveFloor(ctx, lower, a.floor, &w); err != nil {
		return Solid{}, zero, err
	}
	s, err := assembleOrderedRoofBox(ctx, upper, lower, d, max(stock.tolerance, cutter.tolerance), &w)
	if err != nil {
		return Solid{}, zero, fmt.Errorf("roof band assembly: %w", err)
	}
	h := RoofBoxDifferenceHistory{Faces: []MixedRoofFaceOwner{{0, a.roofFace}, {1, b.roofFace}}, Reversed: []bool{false, true}, ExactOperations: w.operations}
	for _, face := range a.sideFaces {
		h.Faces = append(h.Faces, MixedRoofFaceOwner{0, face})
		h.Reversed = append(h.Reversed, false)
	}
	return s, h, nil
}

func unchangedOrderedRoofBox(ctx context.Context, stock Solid, w *mixedRoofLineWork) (Solid, RoofBoxDifferenceHistory, error) {
	if err := ctx.Err(); err != nil {
		return Solid{}, RoofBoxDifferenceHistory{}, err
	}
	h := RoofBoxDifferenceHistory{Faces: make([]MixedRoofFaceOwner, len(stock.faces)), Reversed: make([]bool, len(stock.faces)), ExactOperations: w.operations}
	for i := range h.Faces {
		h.Faces[i] = MixedRoofFaceOwner{0, i}
	}
	return stock, h, nil
}

func subtractFloatingRoofBox(ctx context.Context, stock, cutter Solid, boxes [2]*mixedRoofBox, xy [2]geom.ExactParameterInterval, stockRoof, cutterRoof *geom.SurfacePatch, w *mixedRoofLineWork) (Solid, RoofBoxDifferenceHistory, error) {
	var zero RoofBoxDifferenceHistory
	a, b := boxes[0], boxes[1]
	floorSide, err := orderedRoofAgainstFloor(ctx, a, xy, b.floor, w)
	if err != nil {
		return Solid{}, zero, err
	}
	if floorSide <= 0 {
		return unchangedOrderedRoofBox(ctx, stock, w)
	}
	if err := certifyRoofStrictlyAboveFloor(ctx, stockRoof, b.floor, w); err != nil {
		return Solid{}, zero, err
	}
	cache := make(roofOrderCache)
	order, err := cache.order(boxes, xy, w)
	if err != nil {
		return Solid{}, zero, err
	}
	sign, strict, err := orderedRoofPowerSign(ctx, cache[boxes].power, order, w)
	if err != nil {
		return Solid{}, zero, err
	}
	if sign > 0 && !strict {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	stockFloor, err := orderedRoofFlatFloor(ctx, a.domain, xy, a.floor, w)
	if err != nil {
		return Solid{}, zero, err
	}
	cutterFloor, err := orderedRoofFlatFloor(ctx, a.domain, xy, b.floor, w)
	if err != nil {
		return Solid{}, zero, err
	}
	tolerance := max(stock.tolerance, cutter.tolerance)
	below, err := assembleOrderedRoofBox(ctx, cutterFloor, stockFloor, a.domain, tolerance, w)
	if err != nil {
		return Solid{}, zero, fmt.Errorf("lower roof band: %w", err)
	}
	h := RoofBoxDifferenceHistory{}
	appendBand := func(upper, lower MixedRoofFaceOwner, upperReversed, lowerReversed bool) {
		h.Faces = append(h.Faces, upper, lower)
		h.Reversed = append(h.Reversed, upperReversed, lowerReversed)
		for _, face := range a.sideFaces {
			h.Faces = append(h.Faces, MixedRoofFaceOwner{0, face})
			h.Reversed = append(h.Reversed, false)
		}
	}
	appendBand(MixedRoofFaceOwner{1, b.floorFace}, MixedRoofFaceOwner{0, a.floorFace}, true, false)
	if sign <= 0 {
		h.ExactOperations = w.operations
		return below, h, nil
	}
	// The original cutter's positive thickness separates the closed bands.
	above, err := assembleOrderedRoofBox(ctx, stockRoof, cutterRoof, a.domain, tolerance, w)
	if err != nil {
		return Solid{}, zero, fmt.Errorf("upper roof band: %w", err)
	}
	identity := geom.Identity()
	out, err := AssembleShells(ctx, []PlacedSolid{{below, identity}, {above, identity}}, 12)
	if err != nil {
		return Solid{}, zero, err
	}
	appendBand(MixedRoofFaceOwner{0, a.roofFace}, MixedRoofFaceOwner{1, b.roofFace}, false, true)
	h.ExactOperations = w.operations
	return out, h, nil
}
