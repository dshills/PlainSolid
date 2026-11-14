package trimmed

import (
	"context"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// UnequalSideOpenDualDiskSection retains the original disk ownership and exact
// radical crossing. Unlike SideOpenDualDiskSection it does not imply equal radii.
type UnequalSideOpenDualDiskSection struct{ SideOpenDualDiskSection }

// CertifyUnequalSideOpenDualDiskSection admits unequal positive radii when their
// upper transverse crossing is rational, exactly representable, and strictly
// between the centers. Both disks must strictly clear the other stock sides.
// The exact rational arc proof independently validates the resulting envelope.
func CertifyUnequalSideOpenDualDiskSection(ctx context.Context, min, max geom.Vec2, discs [2]ExtrusionCircle, o trimvalidate.Options) (UnequalSideOpenDualDiskSection, error) {
	r, e := certifySideOpenDualDiskSection(ctx, min, max, discs, o, true)
	if e != nil {
		return UnequalSideOpenDualDiskSection{}, e
	}
	return UnequalSideOpenDualDiskSection{r}, nil
}

// UnequalSideOpenDualDiskCut exposes only the native solid and original source
// face ancestry, together with immutable geometry for its generalized queries.
type UnequalSideOpenDualDiskCut struct {
	source    *sideOpenDualDiskSource
	Solid     Solid
	WallFaces [2][]int
	FloorFace int
}

// CutUnequalSideOpenDualDisks uses exact qualified circular charts and the
// certified section to build blind or through stock. Lossless storage, topology,
// and original cap placement gates match the equal-radius constructor; no
// approximation is substituted if a control or endpoint is unrepresentable.
func CutUnequalSideOpenDualDisks(ctx context.Context, frame geom.Frame, min, max geom.Vec2, discs [2]ExtrusionCircle, height, depth, tolerance float64, o trimvalidate.Options) (UnequalSideOpenDualDiskCut, error) {
	r, e := cutSideOpenDualDisks(ctx, frame, min, max, discs, height, depth, tolerance, o, true, false)
	if e != nil {
		return UnequalSideOpenDualDiskCut{}, e
	}
	return UnequalSideOpenDualDiskCut{Solid: r.Solid, WallFaces: r.WallFaces, FloorFace: r.FloorFace, source: r.source}, nil
}
