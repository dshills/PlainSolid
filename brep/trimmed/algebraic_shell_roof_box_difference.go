package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SubtractOrderedRoofBoxes handles the case where the cutter covers the
// stock's entire XY rectangle and starts at or below its floor. Its top
// must be proved entirely below, entirely above, or strictly between the
// stock's floor and roof. In the intermediate case, exactly one resulting
// band is sewn, using the original cutter roof as the reversed bottom.
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
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}}
	for _, src := range []Solid{stock, cutter} {
		if err := roofBoxCheckSourceEdges(ctx, src, &w); err != nil {
			return Solid{}, zero, fmt.Errorf("roof box source edges: %w", err)
		}
	}
	a, b := q.box[0], q.box[1]
	d := a.domain
	if intersectRectangles(d, b.domain) != d || b.floor.Cmp(a.floor) > 0 || len(a.roofSpans) > 0 || len(b.roofSpans) > 0 {
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
	xy := [2]geom.ExactParameterInterval{{Lower: ratInset(d.UMin), Upper: ratInset(d.UMax)}, {Lower: ratInset(d.VMin), Upper: ratInset(d.VMax)}}
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
