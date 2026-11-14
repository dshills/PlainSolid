package fillet

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// LongEdgeCorner selects either a circular fillet or a two-setback chamfer.
// YSetback measures along the horizontal Y side; ZSetback along vertical Z.
// All zero selects a sharp corner. Corners run counterclockwise in YZ:
// lower/front, lower/back, upper/back, upper/front.
type LongEdgeCorner struct{ Radius, YSetback, ZSetback float64 }

// TreatLongEdgesGrouped builds disjoint parallel longitudinal edge treatments.
// Through features and the upper boss must fit inside both retained flat cap
// strips. The four original side IDs and generated corner IDs are explicit;
// absent generated faces have ID -1. No operation modifies an input solid.
func TreatLongEdgesGrouped(ctx context.Context, length, width, height float64, corners [4]LongEdgeCorner, bore *trimmed.ExtrusionCircle, pocket *trimmed.ExtrusionRectangle, boss *trimmed.ExtrusionBoss, o Options) (LongEdgeChain, error) {
	stock, e := groupedLongStock(ctx, length, width, height, corners, o)
	if e != nil {
		return LongEdgeChain{}, e
	}
	return appendLongEdgeFeatures(ctx, stock, max(stock.TopMinY, stock.BottomMinY), min(stock.TopMaxY, stock.BottomMaxY), bore, pocket, boss, o)
}

func groupedLongStock(ctx context.Context, length, width, height float64, corners [4]LongEdgeCorner, o Options) (LongEdgeTreatment, error) {
	fail := func(e error) (LongEdgeTreatment, error) { return LongEdgeTreatment{}, e }
	if ctx == nil || !finite(length) || !finite(width) || !finite(height) || length <= 0 || width <= 0 || height <= 0 {
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
	var ys, zs [4]float64
	for i, c := range corners {
		if !finite(c.Radius) || !finite(c.YSetback) || !finite(c.ZSetback) || c.Radius < 0 || c.YSetback < 0 || c.ZSetback < 0 || (c.YSetback == 0) != (c.ZSetback == 0) || c.Radius > 0 && c.YSetback > 0 {
			return fail(ErrInvalid)
		}
		ys[i], zs[i] = c.YSetback, c.ZSetback
		if c.Radius > 0 {
			ys[i], zs[i] = c.Radius, c.Radius
		}
		if ys[i] > 0 && (num(ys[i]).Cmp(pad) <= 0 || num(zs[i]).Cmp(pad) <= 0) {
			return fail(ErrRadius)
		}
	}
	remaining := []q{num(length), qs(qs(num(width), num(ys[0])), num(ys[1])), qs(qs(num(height), num(zs[1])), num(zs[2])), qs(qs(num(width), num(ys[2])), num(ys[3])), qs(qs(num(height), num(zs[3])), num(zs[0]))}
	for _, v := range remaining {
		if v.Cmp(pad) <= 0 {
			return fail(ErrRadius)
		}
	}
	base := [4]geom.Vec2{{}, {X: width}, {X: width, Y: height}, {Y: height}}
	incoming := [4]geom.Vec2{{Y: 1}, {X: -1}, {Y: -1}, {X: 1}}
	outgoing := [4]geom.Vec2{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	var points []geom.Vec2
	var selection []CornerRadius
	var first, last [4]int
	var cornerIndex [4]int
	for i, c := range corners {
		first[i] = len(points)
		cornerIndex[i] = -1
		if c.YSetback > 0 {
			for _, dir := range []geom.Vec2{incoming[i], outgoing[i]} {
				x, e := geom.RepresentedExactScalar(qa(num(base[i].X), qm(num(dir.X), num(c.YSetback))))
				if e != nil {
					return fail(e)
				}
				y, e := geom.RepresentedExactScalar(qa(num(base[i].Y), qm(num(dir.Y), num(c.ZSetback))))
				if e != nil {
					return fail(e)
				}
				points = append(points, geom.Vec2{X: x, Y: y})
			}
		} else {
			cornerIndex[i] = len(points)
			points = append(points, base[i])
			if c.Radius > 0 {
				selection = append(selection, CornerRadius{Corner: cornerIndex[i], Radius: c.Radius})
			}
		}
		last[i] = len(points) - 1
	}
	local, e := prismCorners(ctx, points, 0, length, selection, o)
	if e != nil {
		return fail(e)
	}
	world, e := local.Solid.Transformed(geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}})
	if e != nil {
		return fail(e)
	}
	r := LongEdgeTreatment{Solid: world, Profile: local, Length: length, Width: width, Height: height, EndFaces: local.Caps, BottomFace: local.StraightFaces[last[0]], BackFace: local.StraightFaces[last[1]], TopFace: local.StraightFaces[last[2]], FrontFace: local.StraightFaces[last[3]], FilletFace: -1, ChamferFace: -1, CornerFilletFaces: [4]int{-1, -1, -1, -1}, CornerChamferFaces: [4]int{-1, -1, -1, -1}, TopMinY: ys[3], BottomMinY: ys[0]}
	r.TopMaxY, e = geom.RepresentedExactScalar(qs(num(width), num(ys[2])))
	if e != nil {
		return fail(e)
	}
	r.BottomMaxY, e = geom.RepresentedExactScalar(qs(num(width), num(ys[1])))
	if e != nil {
		return fail(e)
	}
	area, perimeter := width*height, 2*(width+height)
	for i, c := range corners {
		if c.Radius > 0 {
			r.CornerFilletFaces[i] = local.Corners[cornerIndex[i]].CylinderFace
			area -= (1 - math.Pi/4) * c.Radius * c.Radius
			perimeter += (math.Pi/2 - 2) * c.Radius
		}
		if c.YSetback > 0 {
			r.CornerChamferFaces[i] = local.StraightFaces[first[i]]
			area -= c.YSetback * c.ZSetback / 2
			perimeter += math.Hypot(c.YSetback, c.ZSetback) - c.YSetback - c.ZSetback
		}
	}
	r.NominalVolume = length * area
	r.NominalSurfaceArea = 2*area + length*perimeter
	if !finite(r.NominalVolume) || !finite(r.NominalSurfaceArea) || r.NominalVolume <= 0 || r.NominalSurfaceArea <= 0 {
		return fail(ErrInvalid)
	}
	return r, nil
}
