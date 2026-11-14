package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// cornerCylinderRecipe proves the axis starts at a stock corner and enters a
// cardinal rectangular solid. A bounds match alone cannot establish stock.
func cornerCylinderRecipe(stock brep.Solid, tool *cylinderBody) (geom.Frame, geom.Vec3, bool) {
	vertices := stock.Vertices()
	if len(vertices) != 8 || len(stock.Edges()) != 12 || len(stock.Faces()) != 6 || tool == nil || !cardinalFrame(tool.frame) {
		return geom.Frame{}, geom.Vec3{}, false
	}
	for _, x := range []geom.Vec3{tool.frame.XAxis(), tool.frame.YAxis(), tool.frame.XAxis().Scale(-1), tool.frame.YAxis().Scale(-1)} {
		f, err := geom.NewFrame(tool.frame.Origin(), x, tool.frame.ZAxis())
		if err != nil {
			continue
		}
		local := make([]geom.Vec3, len(vertices))
		valid := true
		for i, p := range vertices {
			local[i] = f.ToLocal(p)
			if f.ToWorld(local[i]) != p {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		bounds, err := geom.Bounds(local)
		if err != nil || bounds.Min != (geom.Vec3{}) {
			continue
		}
		d := bounds.Max
		if d.X <= tool.radius || d.Y <= tool.radius || d.Z <= tool.height {
			continue
		}
		for _, p := range local {
			if p.X != 0 && p.X != d.X || p.Y != 0 && p.Y != d.Y || p.Z != 0 && p.Z != d.Z {
				valid = false
				break
			}
		}
		if valid && rectangularFaces(stock, f, geom.Vec3{}, d) {
			return f, d, true
		}
	}
	return geom.Frame{}, geom.Vec3{}, false
}

func cornerCylinderBlindRecipe(stock brep.Solid, tool *cylinderBody) (geom.Frame, geom.Vec3, bool) {
	if f, size, ok := cornerCylinderRecipe(stock, tool); ok || tool == nil || !cardinalFrame(tool.frame) {
		return f, size, ok
	}
	axis := tool.frame.ZAxis()
	end := tool.frame.ToWorld(geom.Vec3{Z: tool.height})
	if !exactAxisDistance(end.Dot(axis), tool.frame.Origin().Dot(axis), tool.height) {
		return geom.Frame{}, geom.Vec3{}, false
	}
	f, err := geom.NewFrame(end, tool.frame.XAxis(), axis.Scale(-1))
	if err != nil {
		return geom.Frame{}, geom.Vec3{}, false
	}
	reversed := *tool
	reversed.frame = f
	return cornerCylinderRecipe(stock, &reversed)
}

func (b *Bridge) cornerCylinderBoolean(a, c stored, op string) (Shape, bool, error) {
	if op == "fuse" && a.cylinder != nil && c.planar() {
		a, c = c, a
	}
	if !a.planar() || c.cylinder == nil || op != "cut" && op != "fuse" {
		return Shape{}, false, nil
	}
	if op == "cut" {
		if result, handled, err := b.sideCylinderCut(a, c); handled {
			return result, true, err
		}
		if result, handled, err := b.sideCylinderBlindCut(a, c); handled {
			return result, true, err
		}
		if f, d, ok := cornerCylinderThroughRecipe(a.solid, c.cylinder); ok {
			r := c.cylinder.radius
			solid, err := trimmed.BoxCornerCylinderThrough(context.Background(), f, d.X, d.Y, d.Z, r, 1e-9)
			if err != nil {
				return Shape{}, true, err
			}
			v := a.result.GeometryAssertions.Volume - math.Pi*r*r*d.Z/4
			area := a.result.GeometryAssertions.SurfaceArea + (math.Pi/2-2)*r*d.Z - math.Pi*r*r/2
			p := cornerNotchRecipe(f, d, r, d.Z)
			result, err := b.putNotchedAnalytic(&analyticBody{solid: solid, sideNotch: &p, volume: v, area: area}, a.result.Bounds)
			return result, true, err
		}
	}
	fuse := op == "fuse"
	recipe := cornerCylinderBlindRecipe
	if fuse {
		recipe = cornerCylinderRecipe
	}
	f, d, ok := recipe(a.solid, c.cylinder)
	if !ok {
		return Shape{}, false, nil
	}
	r, h := c.cylinder.radius, c.cylinder.height
	s, err := trimmed.BoxCornerCylinder(context.Background(), f, d.X, d.Y, d.Z, r, h, fuse, 1e-9)
	if err != nil {
		return Shape{}, true, err
	}
	volume := a.result.GeometryAssertions.Volume
	area := a.result.GeometryAssertions.SurfaceArea
	if fuse {
		volume += .75 * math.Pi * r * r * h
		area += 1.5*math.Pi*r*r + (1.5*math.Pi-2)*r*h
	} else {
		volume -= .25 * math.Pi * r * r * h
		area += (math.Pi/2 - 2) * r * h
		p := cornerNotchRecipe(f, d, r, h)
		result, err := b.putNotchedAnalytic(&analyticBody{solid: s, sideNotch: &p, volume: volume, area: area}, a.result.Bounds)
		return result, true, err
	}
	result, err := b.putAnalytic(s, volume, area)
	return result, true, err
}

func cornerNotchRecipe(f geom.Frame, size geom.Vec3, radius, depth float64) step.SideNotchBody {
	return step.SideNotchBody{Name: "PlainSolid", Frame: f, Right: size.X, Back: size.Y, Length: size.Z, Radius: radius, Depth: depth}
}
