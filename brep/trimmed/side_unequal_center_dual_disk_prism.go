package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// UnequalCenterSideOpenDualDiskCut exposes native source-owned faces without
// inheriting the common-center-ordinate query or measure assumptions.
type UnequalCenterSideOpenDualDiskCut struct {
	Solid     Solid
	WallFaces [2][]int
	FloorFace int
	source    *sideOpenDualDiskSource
	join      [2]*big.Rat
}

// CutUnequalCenterSideOpenDualDisks builds the verified explicit-join section's
// blind or through axial continuation with exact homogeneous circle supports.
// Original model endpoints must be represented exactly; no guessed algebraic
// join, snapping, mesh Boolean, or center-ordinate normalization is performed.
func CutUnequalCenterSideOpenDualDisks(ctx context.Context, frame geom.Frame, min, max geom.Vec2, discs [2]ExtrusionCircle, join [2]*big.Rat, height, depth, tolerance float64, o trimvalidate.Options) (UnequalCenterSideOpenDualDiskCut, error) {
	var zero UnequalCenterSideOpenDualDiskCut
	if ctx == nil || !finite(height) || !finite(depth) || !finite(tolerance) || tolerance <= 0 || depth <= 0 || depth > height {
		return zero, geom.ErrInvalid
	}
	section, e := CertifyUnequalCenterSideOpenDualDiskSection(ctx, min, max, discs, join, o)
	if e != nil {
		return zero, e
	}
	s := section.GeneralSideOpenDualDiskSection
	r, e := buildSideOpenDualDiskSection(ctx, frame, min, max, discs, height, depth, tolerance, o, SideOpenDualDiskSection{Outer: s.Outer, Owner: s.Owner, Join: s.Join, XIntervals: s.XIntervals}, true, true)
	if e != nil {
		return zero, e
	}
	return UnequalCenterSideOpenDualDiskCut{Solid: r.Solid, WallFaces: r.WallFaces, FloorFace: r.FloorFace, source: r.source, join: [2]*big.Rat{new(big.Rat).Set(s.Join[0]), new(big.Rat).Set(s.Join[1])}}, nil
}
