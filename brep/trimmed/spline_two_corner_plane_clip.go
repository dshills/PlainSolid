//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CombineSplineTwoCornerPlaneClip clips by one active affine cutter plane,
// retaining two independently certified, distinct original trihedral vertices.
// The contacts must lie on distinct point-only source faces and nonadjacent
// vertices. Each fan owns two distinct affine crossing faces; each crossing
// segment has just one corner end. Every other source face must have an exact
// strict halfspace proof. The full section is independently re-proved as one
// simple oriented cycle. Inactive cutter planes strictly contain the stock.
// Unknown, tangent, coincident, missing and additional contacts fail atomically.
// Limits apply per bounded reproof, as in CombineSplinePlaneClip.
func CombineSplineTwoCornerPlaneClip(ctx context.Context, fans [2]*SplineCornerFan, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions) (SplinePlaneClipResult, error) {
	var zero SplinePlaneClipResult
	if ctx == nil || fans[0] == nil || fans[1] == nil {
		return zero, geom.ErrInvalid
	}
	a, b := fans[0], fans[1]
	for _, fan := range fans {
		if fan.isolated == nil || fan.anchors[0] == nil || fan.anchors[1] == nil {
			return zero, geom.ErrInvalid
		}
	}
	stock, tool := a.anchors[0].solids[0], a.anchors[0].solids[1]
	if a.vertex == b.vertex || !b.isolated.Matches(stock, tool, [2]int{b.isolated.faces[0], a.isolated.faces[1]}) {
		return zero, geom.ErrInvalid
	}
	for _, e := range stock.edges {
		if e.Start == a.vertex && e.End == b.vertex || e.Start == b.vertex && e.End == a.vertex {
			return zero, geom.ErrConvergence
		}
	}
	return combineSplinePlaneClipFans(ctx, stock, tool, a.isolated.faces[1], fans[:], op, o)
}
