package fillet

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// LongEdgeTreatment describes stock [0,Length]×[0,Width]×[0,Height]. The
// fillet replaces the X-directed top/front edge (Y=0,Z=Height); the chamfer
// replaces the opposite top/back edge (Y=Width,Z=Height). Face IDs survive the
// exact cyclic coordinate placement from the authored YZ profile.
type LongEdgeTreatment struct {
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
	fail := func(e error) (LongEdgeTreatment, error) { return LongEdgeTreatment{}, e }
	if ctx == nil || !finite(length) || !finite(width) || !finite(height) || !finite(radius) || !finite(topSetback) || !finite(sideSetback) || length <= 0 || width <= 0 || height <= 0 || radius <= 0 || topSetback <= 0 || sideSetback <= 0 {
		return fail(ErrInvalid)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	if !finite(tol) || tol <= 0 {
		return fail(ErrInvalid)
	}
	pad := qm(num(8), num(tol))
	for _, remaining := range []q{qs(qs(num(width), num(radius)), num(topSetback)), qs(num(height), num(radius)), qs(num(height), num(sideSetback)), num(length), num(topSetback), num(sideSetback)} {
		if remaining.Cmp(pad) <= 0 {
			return fail(ErrRadius)
		}
	}
	topY, e := geom.RepresentedExactScalar(qs(num(width), num(topSetback)))
	if e != nil {
		return fail(e)
	}
	backZ, e := geom.RepresentedExactScalar(qs(num(height), num(sideSetback)))
	if e != nil {
		return fail(e)
	}
	if _, e = geom.RepresentedExactScalar(qs(num(height), num(radius))); e != nil {
		return fail(e)
	}
	polygon := []geom.Vec2{{}, {X: width}, {X: width, Y: backZ}, {X: topY, Y: height}, {Y: height}}
	local, e := PrismCorners(ctx, polygon, 0, length, []CornerRadius{{Corner: 4, Radius: radius}}, o)
	if e != nil {
		return fail(e)
	}
	// (local X,local Y,local Z)=(world Y,world Z,world X); determinant +1.
	world, e := local.Solid.Transformed(geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}})
	if e != nil {
		return fail(e)
	}
	r := LongEdgeTreatment{Solid: world, Profile: local, Length: length, Width: width, Height: height, Radius: radius, TopSetback: topSetback, SideSetback: sideSetback, EndFaces: local.Caps, BottomFace: local.StraightFaces[0], BackFace: local.StraightFaces[1], ChamferFace: local.StraightFaces[2], TopFace: local.StraightFaces[3], FrontFace: local.StraightFaces[4], FilletFace: local.Corners[4].CylinderFace}
	r.NominalVolume = length * (width*height - (1-math.Pi/4)*radius*radius - topSetback*sideSetback/2)
	perimeter := 2*(width+height) - 2*radius + math.Pi*radius/2 - topSetback - sideSetback + math.Hypot(topSetback, sideSetback)
	r.NominalSurfaceArea = 2*r.NominalVolume/length + length*perimeter
	return r, nil
}
