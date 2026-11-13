package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func sideCylinderThroughRecipe(stock brep.Solid, tool *cylinderBody) (geom.Frame, geom.Box, bool) {
	if tool == nil || !cardinalFrame(tool.frame) || len(stock.Vertices()) != 8 || len(stock.Edges()) != 12 || len(stock.Faces()) != 6 {
		return geom.Frame{}, geom.Box{}, false
	}
	for _, x := range []geom.Vec3{tool.frame.XAxis(), tool.frame.YAxis(), tool.frame.XAxis().Scale(-1), tool.frame.YAxis().Scale(-1)} {
		f, e := geom.NewFrame(tool.frame.Origin(), x, tool.frame.ZAxis())
		if e != nil {
			continue
		}
		v := make([]geom.Vec3, 8)
		good := true
		for i, p := range stock.Vertices() {
			v[i] = f.ToLocal(p)
			if f.ToWorld(v[i]) != p {
				good = false
			}
		}
		if !good {
			continue
		}
		box, e := geom.Bounds(v)
		r := tool.radius
		if e != nil || box.Min.X >= -r || box.Max.X <= r || math.Abs(box.Min.Y) >= r || box.Max.Y <= r || box.Min.Z < 0 || box.Max.Z > tool.height {
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
	a := -box.Min.Y
	sweep := math.Pi + 2*math.Asin(a/r)
	chord := 2 * math.Sqrt((r-a)*(r+a))
	area := r*r*sweep/2 + a*chord/2
	return area * box.Max.Z, (r*sweep-chord)*box.Max.Z - 2*area
}
func (b *Bridge) sideCylinderCut(a, c stored) (Shape, bool, error) {
	if !a.planar() || c.cylinder == nil {
		return Shape{}, false, nil
	}
	f, box, ok := sideCylinderThroughRecipe(a.solid, c.cylinder)
	if !ok {
		return Shape{}, false, nil
	}
	r := c.cylinder.radius
	s, e := trimmed.BoxSideCylinderThrough(context.Background(), f, -box.Min.X, box.Max.X, box.Max.Y, -box.Min.Y, box.Max.Z, r, 1e-9)
	if e != nil {
		return Shape{}, true, e
	}
	v, area := sideCylinderMeasures(box, r)
	out, e := b.putNotchedStock(s, a.result.GeometryAssertions.Volume-v, a.result.GeometryAssertions.SurfaceArea+area, a.result.Bounds)
	return out, true, e
}

// Every original box extremum survives a qualified one-side or corner notch.
func (b *Bridge) putNotchedStock(s trimmed.Solid, volume, area float64, bounds Bounds) (Shape, error) {
	if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 || len(s.Topology().Shells) != 1 {
		return Shape{}, unsupportedEdge("invalid notched stock measures")
	}
	return b.publish(stored{analytic: &analyticBody{solid: s, volume: volume, area: area}, result: Shape{GeometryAssertions: Assertions{true, volume, area, 1}, Bounds: bounds}})
}
