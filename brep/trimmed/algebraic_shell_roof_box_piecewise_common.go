package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/geom"
)

// The two proved original graphs induce a conforming grid on the common
// footprint. Sew only roof and floor tiles, plus walls at the outer perimeter.
// An edge inside this grid joins two tiles directly on each cap. Source
// continuity and exact equality of the pointwise minimum on either side
// give the same geometric edge even when the selected roof changes at a knot.
func intersectPiecewiseRoofBoxes(ctx context.Context, a, b Solid, q *MixedRoofMaterialQuery, d geom.ParameterRectangle, floorIndex int, w *mixedRoofLineWork) (Solid, RoofBoxCommonHistory, error) {
	var empty RoofBoxCommonHistory
	floor := q.box[floorIndex].floor
	cuts := [2][]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for _, box := range q.box {
		if len(box.roofSpans) == 0 {
			continue
		}
		u, v := box.roof.KnotBreaks()
		for axis, breaks := range [2][]float64{u, v} {
			for _, x := range breaks {
				if x > cuts[axis][0] && x < cuts[axis][1] {
					cuts[axis] = append(cuts[axis], x)
				}
			}
		}
	}
	for i := range cuts {
		slices.Sort(cuts[i])
		cuts[i] = slices.Compact(cuts[i])
	}
	nu, nv := len(cuts[0])-1, len(cuts[1])-1
	// Keep the result within Sew's 128-face limit before spending proof work.
	if 2*(nu*nv+nu+nv) > 128 {
		return Solid{}, empty, geom.ErrUnrepresentable
	}
	h := RoofBoxCommonHistory{ProjectedContact: true}
	var parts []SewFace
	var positiveCells int
	for i := 0; i < nu; i++ {
		for j := 0; j < nv; j++ {
			if err := ctx.Err(); err != nil {
				return Solid{}, empty, err
			}
			cell := geom.ParameterRectangle{UMin: cuts[0][i], UMax: cuts[0][i+1], VMin: cuts[1][j], VMax: cuts[1][j+1]}
			xy := [2]geom.ExactParameterInterval{
				{Lower: ratInset(cell.UMin), Upper: ratInset(cell.UMax)},
				{Lower: ratInset(cell.VMin), Upper: ratInset(cell.VMax)},
			}
			var roofs [2]*mixedRoofBox
			for k, box := range q.box {
				roofs[k] = box
				if len(box.roofSpans) == 0 {
					continue
				}
				roofs[k] = nil
				for _, span := range box.roofSpans {
					if intersectRectangles(cell, span.domain) == cell {
						roofs[k] = span
						break
					}
				}
				if roofs[k] == nil {
					return Solid{}, empty, geom.ErrUnrepresentable
				}
			}
			order, err := orderedRoofBoxOrder(ctx, roofs, xy, w)
			if err != nil {
				return Solid{}, empty, err
			}
			owner := 0
			if order > 0 {
				owner = 1
			}
			gap, err := orderedRoofAgainstFloor(ctx, roofs[owner], xy, floor, w)
			if err != nil {
				return Solid{}, empty, err
			}
			if gap <= 0 {
				continue
			}
			positiveCells++
			upper, err := orderedRoofRestrictedGraph(ctx, roofs[owner], cell, w)
			if err != nil {
				return Solid{}, empty, err
			}
			if err := certifyRoofStrictlyAboveFloor(ctx, upper, floor, w); err != nil {
				return Solid{}, empty, err
			}
			lower, err := orderedRoofFlatFloor(ctx, cell, xy, floor, w)
			if err != nil {
				return Solid{}, empty, err
			}
			var walls uint8
			h.Faces = append(h.Faces, MixedRoofFaceOwner{owner, q.box[owner].roofFace}, MixedRoofFaceOwner{floorIndex, q.box[floorIndex].floorFace})
			for k, boundary := range [4]bool{i == 0, i == nu-1, j == 0, j == nv-1} {
				if boundary {
					walls |= 1 << k
					h.Faces = append(h.Faces, roofCommonWallOwner(q.box, d, k))
				}
			}
			faces, err := orderedRoofBoxFaces(ctx, upper, lower, cell, walls, w)
			if err != nil {
				return Solid{}, empty, err
			}
			parts = append(parts, faces...)
		}
	}
	if positiveCells == 0 {
		h.ExactOperations = w.operations
		return Solid{}, h, ctx.Err()
	}
	if positiveCells != nu*nv {
		return Solid{}, empty, geom.ErrUnrepresentable
	}
	result, err := sewOrderedRoofBoxFaces(ctx, parts, max(a.tolerance, b.tolerance), w)
	if err != nil {
		return Solid{}, empty, fmt.Errorf("sew ordered piecewise roof common: %w", err)
	}
	h.ExactOperations = w.operations
	return result, h, nil
}
