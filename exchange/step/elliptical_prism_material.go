package step

import (
	"bytes"
	"context"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// EllipticalPrismMaterial is an explicitly reconstructed native solid. Four
// rational side charts replace one periodic source face; four exact rational
// quarter edges partition each original full rim. No source approximation is
// promoted as exact: Shell retains the enforced periodic repair evidence.
type EllipticalPrismMaterial struct {
	Solid               trimmed.Solid
	Shell               RepairedEllipticalPrismShell
	NativeFaceSourceIDs [6]int64  // lower cap, upper cap, four sides
	NativeRimEdges      [2][4]int // lower, upper, increasing source ellipse angle
	SourceRimEdgeIDs    [2]int64
}

// PromoteRepairedEllipticalPrism authenticates the retained source again, then
// constructs an embedded ellipse prism using rational quarters (weights1,1,2).
// Every world control point must be losslessly representable. Exact sewing
// proves continuous model/UV agreement; positive radii and strictly increasing
// height establish the disjoint quadrant walls and bounded cap disks. Tolerance
// is native validation tolerance, not permission to round geometric data.
func PromoteRepairedEllipticalPrism(ctx context.Context, source *File, l Limits, o EllipticalWireRepairOptions, tolerance float64) (EllipticalPrismMaterial, error) {
	fail := func(e error) (EllipticalPrismMaterial, error) { return EllipticalPrismMaterial{}, e }
	if ctx == nil || !finiteNumber(tolerance) || tolerance <= 0 || !ellipseOwnedSourceBound(source, 121) {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return fail(e)
	}
	shell, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(raw), l, o)
	if e != nil {
		return fail(e)
	}
	frame, a, b := shell.Side.Profile.Parameters()
	height := shell.Side.Height
	solid, rim, e := sewExactEllipticalPrism(ctx, frame, a, b, height, tolerance)
	if e != nil {
		return fail(e)
	}
	out := EllipticalPrismMaterial{Solid: solid, Shell: shell, SourceRimEdgeIDs: [2]int64{shell.SourceEdgeIDs[2], shell.SourceEdgeIDs[1]}}
	for i := 0; i < 4; i++ {
		out.NativeFaceSourceIDs[i+2] = shell.SourceFaceIDs[0]
	}
	out.NativeFaceSourceIDs[0] = shell.SourceFaceIDs[1]
	out.NativeFaceSourceIDs[1] = shell.SourceFaceIDs[2]
	out.NativeRimEdges = rim
	return out, nil
}
