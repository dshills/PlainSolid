package trimmed

import (
	"context"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// OffsetSideOpenDualDiskCut retains native topology and original cutter face
// ancestry for the explicit external-center domain. It deliberately does not
// inherit side-centered analytic measures or material query methods.
type OffsetSideOpenDualDiskCut struct {
	Solid     Solid
	WallFaces [2][]int
	FloorFace int
}

// CutOffsetSideOpenDualDisks constructs the certified offset section's axial
// continuation using exact homogeneous cylindrical supports. The positive
// common side offset, rational endpoints, signed coordinate placement and
// blind/through depth are independently checked. No rounded curve controls or
// sampled Boolean fallback are used. Native model vertices must remain exactly
// representable under the existing original-boundary assembly contract.
func CutOffsetSideOpenDualDisks(ctx context.Context, frame geom.Frame, min, max geom.Vec2, discs [2]ExtrusionCircle, height, depth, tolerance float64, o trimvalidate.Options) (OffsetSideOpenDualDiskCut, error) {
	var zero OffsetSideOpenDualDiskCut
	if ctx == nil || !finite(height) || !finite(depth) || !finite(tolerance) || tolerance <= 0 || depth <= 0 || depth > height {
		return zero, geom.ErrInvalid
	}
	section, e := CertifyOffsetSideOpenDualDiskSection(ctx, min, max, discs, o)
	if e != nil {
		return zero, e
	}
	s := section.GeneralSideOpenDualDiskSection
	r, e := buildSideOpenDualDiskSection(ctx, frame, min, max, discs, height, depth, tolerance, o, SideOpenDualDiskSection{Outer: s.Outer, Owner: s.Owner, Join: s.Join, XIntervals: s.XIntervals}, true, true)
	if e != nil {
		return zero, e
	}
	return OffsetSideOpenDualDiskCut{Solid: r.Solid, WallFaces: r.WallFaces, FloorFace: r.FloorFace}, nil
}
