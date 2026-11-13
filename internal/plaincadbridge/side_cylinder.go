package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func sideCylinderThroughRecipe(stock brep.Solid, tool *cylinderBody) (geom.Frame, geom.Box, bool) {
	return sideCylinderStockRecipe(stock, tool, false)
}

func sideCylinderStockRecipe(stock brep.Solid, tool *cylinderBody, blind bool) (geom.Frame, geom.Box, bool) {
	vertices := stock.Vertices()
	if tool == nil || !cardinalFrame(tool.frame) || len(vertices) != 8 || len(stock.Edges()) != 12 || len(stock.Faces()) != 6 {
		return geom.Frame{}, geom.Box{}, false
	}
	for _, x := range []geom.Vec3{tool.frame.XAxis(), tool.frame.YAxis(), tool.frame.XAxis().Scale(-1), tool.frame.YAxis().Scale(-1)} {
		f, e := geom.NewFrame(tool.frame.Origin(), x, tool.frame.ZAxis())
		if e != nil {
			continue
		}
		v := make([]geom.Vec3, 8)
		good := true
		for i, p := range vertices {
			v[i] = f.ToLocal(p)
			if f.ToWorld(v[i]) != p {
				good = false
			}
		}
		if !good {
			continue
		}
		if blind {
			origin := f.Origin()
			for i, p := range vertices {
				q := v[i]
				if !exactAxisDistance(p.Dot(f.XAxis()), origin.Dot(f.XAxis()), q.X) ||
					!exactAxisDistance(p.Dot(f.YAxis()), origin.Dot(f.YAxis()), q.Y) ||
					!exactAxisDistance(p.Dot(f.ZAxis()), origin.Dot(f.ZAxis()), q.Z) {
					good = false
					break
				}
			}
			finish := f.ToWorld(geom.Vec3{Z: tool.height})
			if !good || !exactAxisDistance(finish.Dot(f.ZAxis()), origin.Dot(f.ZAxis()), tool.height) {
				continue
			}
		}
		box, e := geom.Bounds(v)
		r := tool.radius
		if e != nil || box.Min.X >= -r || box.Max.X <= r || math.Abs(box.Min.Y) >= r || box.Max.Y <= r || box.Min.Z < 0 {
			continue
		}
		if blind && (box.Min.Z != 0 || box.Max.Z <= tool.height) || !blind && box.Max.Z > tool.height {
			continue
		}
		origin := f.ToWorld(geom.Vec3{Z: box.Min.Z})
		nf, e := geom.NewFrame(origin, x, tool.frame.ZAxis())
		if e != nil || f.ToLocal(origin) != (geom.Vec3{Z: box.Min.Z}) {
			continue
		}
		box.Max.Z -= box.Min.Z
		box.Min.Z = 0
		if rectangularFaces(stock, nf, box.Min, box.Max) {
			return nf, box, true
		}
	}
	return geom.Frame{}, geom.Box{}, false
}
func sideCylinderMeasures(box geom.Box, r float64) (float64, float64) {
	area, perimeter := sideCylinderSection(-box.Min.Y, r)
	return area * box.Max.Z, perimeter*box.Max.Z - 2*area
}

func sideCylinderSection(a, r float64) (area, wallDelta float64) {
	sweep := math.Pi + 2*math.Asin(a/r)
	chord := 2 * math.Sqrt((r-a)*(r+a))
	return r*r*sweep/2 + a*chord/2, r*sweep - chord
}

func sideCylinderBlindRecipe(stock brep.Solid, tool *cylinderBody) (geom.Frame, geom.Box, bool) {
	if f, box, ok := sideCylinderStockRecipe(stock, tool, true); ok || tool == nil || !cardinalFrame(tool.frame) {
		return f, box, ok
	}
	axis := tool.frame.ZAxis()
	end := tool.frame.ToWorld(geom.Vec3{Z: tool.height})
	if !exactAxisDistance(end.Dot(axis), tool.frame.Origin().Dot(axis), tool.height) {
		return geom.Frame{}, geom.Box{}, false
	}
	f, err := geom.NewFrame(end, tool.frame.XAxis(), axis.Scale(-1))
	if err != nil {
		return geom.Frame{}, geom.Box{}, false
	}
	reversed := *tool
	reversed.frame = f
	return sideCylinderStockRecipe(stock, &reversed, true)
}

func (b *Bridge) sideCylinderBlindCut(a, c stored) (Shape, bool, error) {
	if !a.planar() {
		return Shape{}, false, nil
	}
	f, box, ok := sideCylinderBlindRecipe(a.solid, c.cylinder)
	if !ok {
		return Shape{}, false, nil
	}
	radius, depth := c.cylinder.radius, c.cylinder.height
	p := sideNotchRecipe(f, box, radius, depth)
	s, err := trimmed.BoxSideCylinderBlind(context.Background(), p.Frame, p.Left, p.Right, p.Back, p.Inset, p.Length, p.Radius, p.Depth, 1e-9)
	if err != nil {
		return Shape{}, true, err
	}
	section, wall := sideCylinderSection(p.Inset, radius)
	body := &analyticBody{solid: s, sideNotch: &p,
		volume: a.result.GeometryAssertions.Volume - section*depth,
		area:   a.result.GeometryAssertions.SurfaceArea + wall*depth}
	out, err := b.putNotchedAnalytic(body, a.result.Bounds)
	return out, true, err
}
func sideNotchRecipe(f geom.Frame, box geom.Box, radius, depth float64) step.SideNotchBody {
	return step.SideNotchBody{Name: "PlainSolid", Frame: f, Left: -box.Min.X, Right: box.Max.X,
		Back: box.Max.Y, Inset: -box.Min.Y, Length: box.Max.Z, Radius: radius, Depth: depth}
}

func (b *Bridge) sideCylinderCut(a, c stored) (Shape, bool, error) {
	if !a.planar() || c.cylinder == nil {
		return Shape{}, false, nil
	}
	f, box, ok := sideCylinderThroughRecipe(a.solid, c.cylinder)
	if !ok {
		return Shape{}, false, nil
	}
	p := sideNotchRecipe(f, box, c.cylinder.radius, box.Max.Z)
	s, e := trimmed.BoxSideCylinderThrough(context.Background(), p.Frame, p.Left, p.Right, p.Back, p.Inset, p.Length, p.Radius, 1e-9)
	if e != nil {
		return Shape{}, true, e
	}
	v, area := sideCylinderMeasures(box, p.Radius)
	body := &analyticBody{solid: s, sideNotch: &p,
		volume: a.result.GeometryAssertions.Volume - v,
		area:   a.result.GeometryAssertions.SurfaceArea + area}
	out, e := b.putNotchedAnalytic(body, a.result.Bounds)
	return out, true, e
}

// Every original box extremum survives a qualified one-side or corner notch.
func (b *Bridge) putNotchedStock(s trimmed.Solid, volume, area float64, bounds Bounds) (Shape, error) {
	return b.putNotchedAnalytic(&analyticBody{solid: s, volume: volume, area: area}, bounds)
}

func (b *Bridge) putNotchedAnalytic(body *analyticBody, bounds Bounds) (Shape, error) {
	if !finite(body.volume) || !finite(body.area) || body.volume <= 0 || body.area <= 0 || len(body.solid.Topology().Shells) != 1 {
		return Shape{}, unsupportedEdge("invalid notched stock measures")
	}
	return b.publish(stored{analytic: body, result: Shape{GeometryAssertions: Assertions{true, body.volume, body.area, 1}, Bounds: bounds}})
}

// The qualified notch leaves each coordinate extremum attained elsewhere
// on the box. Box bounds exclude the empty part of the cylinder chart.
func sideNotchBounds(p step.SideNotchBody) (geom.Box, error) {
	var points [8]geom.Vec3
	i := 0
	for _, x := range []float64{-p.Left, p.Right} {
		for _, y := range []float64{-p.Inset, p.Back} {
			for _, z := range []float64{0, p.Length} {
				points[i] = p.Frame.ToWorld(geom.Vec3{X: x, Y: y, Z: z})
				i++
			}
		}
	}
	return geom.Bounds(points[:])
}
