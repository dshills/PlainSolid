package section

import (
	"context"

	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// SplitResult contains a transverse section and both closed split pieces.
type SplitResult struct {
	Negative, Positive brep.Solid
	Loops              []Loop
}

// Split partitions a nonconvex planar solid by a transverse plane. The
// section loops and the partition use the same represented cutting plane.
// Coplanar, edge and vertex contacts return ErrContact, as in Planar; callers
// that need the split solids without a section can use poly.SplitPlane.
func Split(ctx context.Context, s brep.Solid, p geom.Plane, o poly.Options) (SplitResult, error) {
	loops, err := Planar(ctx, s, p, o.MaxInputTriangles)
	if err != nil {
		return SplitResult{}, err
	}
	n, pos, err := poly.SplitPlane(ctx, s, p, o)
	if err != nil {
		return SplitResult{}, err
	}
	return SplitResult{Negative: n, Positive: pos, Loops: loops}, nil
}
