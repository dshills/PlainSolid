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
// strictly between the stock's floor and roof. A cutter strictly above the
// floor and below the roof yields two disjoint bands with reversed cutter
// floor and roof. The original face ancestry follows the output face order.
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
	level, err := geom.RepresentedExactScalar(a.floor)
	if err != nil {
		return Solid{}, zero, err
	}
	plane := mixedCoordinatePlane(2, level)
	left, err := roofBoxConstructionOptions(&w)
	if err != nil {
		return Solid{}, zero, err
	}
	atFloor, err := geom.CertifySurfaceSupportingPlane(ctx, lower, plane[0], plane[1], plane[2], geom.SurfaceSupportingPlaneOptions{MaxOperations: left.MaxOperations, MaxCoefficientBits: left.MaxCoefficientBits})
	w.operations += atFloor.ExactOperations
	if err != nil {
		return Solid{}, zero, err
	}
	if !atFloor.Complete {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	if atFloor.Coincident || atFloor.HalfspaceSign < 0 {
		// Subtraction can remove only a boundary subset. Regularization
		// retains the whole authenticated source stock and its native IDs.
		h := RoofBoxDifferenceHistory{ExactOperations: w.operations}
		for i := range stock.faces {
			h.Faces = append(h.Faces, MixedRoofFaceOwner{0, i})
			h.Reversed = append(h.Reversed, false)
		}
		return stock, h, ctx.Err()
	}
	if atFloor.HalfspaceSign != 1 || atFloor.SideMask != 0 || atFloor.CornerMask != 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	cache := make(roofOrderCache)
	order, err := cache.order(q.box, xy, &w)
	if err != nil {
		return Solid{}, zero, err
	}
	if order.Upper.Sign() <= 0 {
		return Solid{}, RoofBoxDifferenceHistory{ExactOperations: w.operations}, ctx.Err()
	}
	// A strict lower bound is sufficient. Tangencies or mixed coefficients
	// are not accepted here because a pinched difference is not a manifold.
	if order.Lower.Sign() <= 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
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

func subtractFloatingRoofBox(ctx context.Context, stock, cutter Solid, boxes [2]*mixedRoofBox, xy [2]geom.ExactParameterInterval, stockRoof, cutterRoof *geom.SurfacePatch, w *mixedRoofLineWork) (Solid, RoofBoxDifferenceHistory, error) {
	var zero RoofBoxDifferenceHistory
	order, err := make(roofOrderCache).order(boxes, xy, w)
	if err != nil {
		return Solid{}, zero, err
	}
	if order.Lower.Sign() <= 0 {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	a, b := boxes[0], boxes[1]
	stockFloor, err := orderedRoofFlatFloor(ctx, a.domain, xy, a.floor, w)
	if err != nil {
		return Solid{}, zero, err
	}
	cutterFloor, err := orderedRoofFlatFloor(ctx, a.domain, xy, b.floor, w)
	if err != nil {
		return Solid{}, zero, err
	}
	tolerance := max(stock.tolerance, cutter.tolerance)
	// Both gaps adjacent to the removed tool are full-dimensional. The
	// re-proved positive thickness of the cutter separates their closed
	// supports, so assembling the two closed material shells needs no weld.
	below, err := assembleOrderedRoofBox(ctx, cutterFloor, stockFloor, a.domain, tolerance, w)
	if err != nil {
		return Solid{}, zero, fmt.Errorf("lower roof band: %w", err)
	}
	above, err := assembleOrderedRoofBox(ctx, stockRoof, cutterRoof, a.domain, tolerance, w)
	if err != nil {
		return Solid{}, zero, fmt.Errorf("upper roof band: %w", err)
	}
	identity := geom.Identity()
	out, err := AssembleShells(ctx, []PlacedSolid{{below, identity}, {above, identity}}, 12)
	if err != nil {
		return Solid{}, zero, err
	}
	h := RoofBoxDifferenceHistory{ExactOperations: w.operations}
	for i, caps := range [2][2]MixedRoofFaceOwner{{{1, b.floorFace}, {0, a.floorFace}}, {{0, a.roofFace}, {1, b.roofFace}}} {
		h.Faces = append(h.Faces, caps[:]...)
		h.Reversed = append(h.Reversed, i == 0, i == 1)
		for _, face := range a.sideFaces {
			h.Faces = append(h.Faces, MixedRoofFaceOwner{0, face})
			h.Reversed = append(h.Reversed, false)
		}
	}
	return out, h, nil
}
