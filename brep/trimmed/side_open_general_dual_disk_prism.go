package trimmed

import (
	"context"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// GeneralSideOpenDualDiskCut retains distinct opt-in construction semantics,
// inheriting the unequal result's per-disk point queries and signed-offset
// analytic measures. Its source binding is the actual original geometry.
type GeneralSideOpenDualDiskCut struct{ UnequalSideOpenDualDiskCut }

// CutGeneralSideOpenDualDisks builds a certified generalized section, combining
// each exposed upper circular portion into one exact native cylinder chart.
// The radical join may lie outside the centers. Native control poles, weights
// and endpoints must still be losslessly representable; a valid planar section
// need not qualify for this native subset. No sampled fallback or pole rounding
// is performed. Blind/through axial placement and signed frames follow the
// existing qualified native construction.
func CutGeneralSideOpenDualDisks(ctx context.Context, frame geom.Frame, min, max geom.Vec2, discs [2]ExtrusionCircle, height, depth, tolerance float64, o trimvalidate.Options) (GeneralSideOpenDualDiskCut, error) {
	r, e := cutSideOpenDualDisks(ctx, frame, min, max, discs, height, depth, tolerance, o, true, true)
	if e != nil {
		return GeneralSideOpenDualDiskCut{}, e
	}
	return GeneralSideOpenDualDiskCut{UnequalSideOpenDualDiskCut{Solid: r.Solid, WallFaces: r.WallFaces, FloorFace: r.FloorFace, source: &sideOpenDualDiskSource{frame: frame, min: min, max: max, discs: discs, height: height, depth: depth}}}, nil
}
