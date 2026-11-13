package plaincadbridge

import (
	"plainsolid/brep"
	"plainsolid/geom"
)

// Require exact represented axial coverage, not an infinite drill. The stock
// may lie anywhere inside the tool's finite axis interval.
func cornerCylinderThroughRecipe(stock brep.Solid, tool *cylinderBody) (geom.Frame, geom.Vec3, bool) {
	fail := func() (geom.Frame, geom.Vec3, bool) { return geom.Frame{}, geom.Vec3{}, false }
	if tool == nil || !cardinalFrame(tool.frame) || len(stock.Vertices()) != 8 || len(stock.Edges()) != 12 || len(stock.Faces()) != 6 {
		return fail()
	}
	for _, x := range []geom.Vec3{tool.frame.XAxis(), tool.frame.YAxis(), tool.frame.XAxis().Scale(-1), tool.frame.YAxis().Scale(-1)} {
		f, e := geom.NewFrame(tool.frame.Origin(), x, tool.frame.ZAxis())
		if e != nil {
			continue
		}
		local := make([]geom.Vec3, 8)
		valid := true
		for i, p := range stock.Vertices() {
			local[i] = f.ToLocal(p)
			if f.ToWorld(local[i]) != p {
				valid = false
			}
		}
		if !valid {
			continue
		}
		bounds, e := geom.Bounds(local)
		if e != nil || bounds.Min.X != 0 || bounds.Min.Y != 0 || bounds.Min.Z < 0 || bounds.Max.Z > tool.height {
			continue
		}
		size := bounds.Max.Sub(bounds.Min)
		if size.X <= tool.radius || size.Y <= tool.radius || size.Z <= 0 {
			continue
		}
		origin := f.ToWorld(bounds.Min)
		nf, e := geom.NewFrame(origin, x, tool.frame.ZAxis())
		if e != nil || f.ToLocal(origin) != bounds.Min {
			continue
		}
		if rectangularFaces(stock, nf, geom.Vec3{}, size) {
			return nf, size, true
		}
	}
	return fail()
}
