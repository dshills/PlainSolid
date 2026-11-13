package fillet

import (
	"context"
	"plainsolid/brep/trimmed"
)

// LongEdgeTreatment retains stock [0,Length]×[0,Width]×[0,Height].
// Corner face arrays use lower/front, lower/back, upper/back, upper/front
// order, with -1 for absent treatments. Top/Bottom MinY/MaxY describe the
// surviving straight horizontal cap strips. Original side IDs refer to the
// corresponding straight profile side; EndFaces are the X-normal ends.
// Radius, setbacks and singular FilletFace/ChamferFace describe the legacy
// one-fillet/one-chamfer constructor only; grouped callers use corner arrays.
type LongEdgeTreatment struct {
	CornerFilletFaces, CornerChamferFaces                             [4]int
	TopMinY, TopMaxY, BottomMinY, BottomMaxY                          float64
	Solid                                                             trimmed.Solid
	Profile                                                           SelectiveResult
	Length, Width, Height, Radius, TopSetback, SideSetback            float64
	EndFaces                                                          [2]int
	BottomFace, BackFace, TopFace, FrontFace, FilletFace, ChamferFace int
	NominalVolume, NominalSurfaceArea                                 float64
}

// TreatOppositeLongCapEdges authors a single convex rounded/beveled profile
// extruded along X. topSetback is measured into the top face and sideSetback
// down the back face; radius rounds the opposite top/front edge. Positive
// remaining top/front/back straight portions are required with eight sewing
// tolerances of clearance. Exact represented subtraction checks prevent silent
// changes to nominal profile dimensions. Circular sewing follows Options'
// original tangency tolerance; nominal measures refer to the ideal construction.
// This is a joint constructor, not a general edge-selection or history replay.
func TreatOppositeLongCapEdges(ctx context.Context, length, width, height, radius, topSetback, sideSetback float64, o Options) (LongEdgeTreatment, error) {
	if radius <= 0 || topSetback <= 0 || sideSetback <= 0 {
		return LongEdgeTreatment{}, ErrInvalid
	}
	return treatOptionalStock(ctx, length, width, height, radius, topSetback, sideSetback, false, o)
}
