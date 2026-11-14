package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
)

// RectangularIslandPocket removes a rectangular annular footprint from the
// upper cap of a rectangular prism. The disk island remains connected to the
// stock below the blind floor and extends to the original upper cap.
type RectangularIslandPocket struct {
	Frame                                                  geom.Frame
	OuterMin, OuterMax, PocketMin, PocketMax, IslandCenter geom.Vec2
	IslandRadius, Height, Depth                            float64
}
type RectangularIslandPocketResult struct {
	Solid                                  Solid
	OuterTopFace, FloorFace, IslandTopFace int
}

func (p RectangularIslandPocket) validate() (float64, error) {
	if !uvFinite(p.OuterMin) || !uvFinite(p.OuterMax) || !uvFinite(p.PocketMin) || !uvFinite(p.PocketMax) || !uvFinite(p.IslandCenter) || !finite(p.IslandRadius) || !finite(p.Height) || !finite(p.Depth) || p.Frame.ZAxis().Length() == 0 {
		return 0, geom.ErrInvalid
	}
	width, height := p.OuterMax.X-p.OuterMin.X, p.OuterMax.Y-p.OuterMin.Y
	if !finite(width) || !finite(height) || min(width, height, p.Height, p.Depth, p.IslandRadius) <= 0 {
		return 0, geom.ErrDomain
	}
	tol := max(width, height, p.Height) * 1e-11
	gap := 8 * tol
	if !finite(tol) || tol <= 0 || p.Depth <= gap || p.Height-p.Depth <= gap || p.IslandRadius <= gap ||
		p.PocketMin.X-p.OuterMin.X <= gap || p.PocketMin.Y-p.OuterMin.Y <= gap || p.OuterMax.X-p.PocketMax.X <= gap || p.OuterMax.Y-p.PocketMax.Y <= gap ||
		p.PocketMax.X-p.PocketMin.X <= gap || p.PocketMax.Y-p.PocketMin.Y <= gap ||
		min(p.IslandCenter.X-p.PocketMin.X, p.IslandCenter.Y-p.PocketMin.Y, p.PocketMax.X-p.IslandCenter.X, p.PocketMax.Y-p.IslandCenter.Y)-p.IslandRadius <= gap {
		return 0, geom.ErrDomain
	}
	return tol, nil
}
func (p RectangularIslandPocket) NominalVolume() (float64, error) {
	if _, e := p.validate(); e != nil {
		return 0, e
	}
	stock := (p.OuterMax.X - p.OuterMin.X) * (p.OuterMax.Y - p.OuterMin.Y) * p.Height
	removed := ((p.PocketMax.X-p.PocketMin.X)*(p.PocketMax.Y-p.PocketMin.Y) - math.Pi*p.IslandRadius*p.IslandRadius) * p.Depth
	v := stock - removed
	if !finite(v) || v <= 0 {
		return 0, geom.ErrUnrepresentable
	}
	return v, nil
}
func (p RectangularIslandPocket) SurfaceArea() (float64, error) {
	if _, e := p.validate(); e != nil {
		return 0, e
	}
	w, h := p.OuterMax.X-p.OuterMin.X, p.OuterMax.Y-p.OuterMin.Y
	area := 2*w*h + 2*(w+h)*p.Height + 2*(p.PocketMax.X-p.PocketMin.X+p.PocketMax.Y-p.PocketMin.Y+math.Pi*p.IslandRadius)*p.Depth
	if !finite(area) || area <= 0 {
		return 0, geom.ErrUnrepresentable
	}
	return area, nil
}
func (p RectangularIslandPocket) Solid(ctx context.Context) (Solid, error) {
	r, e := p.Build(ctx)
	return r.Solid, e
}
func (p RectangularIslandPocket) Build(ctx context.Context) (RectangularIslandPocketResult, error) {
	var out RectangularIslandPocketResult
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	tol, e := p.validate()
	if e != nil {
		return out, e
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: p.Frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	stock := extrusionRectangleRing(ExtrusionRectangle{Min: p.OuterMin, Max: p.OuterMax})
	outer := []geom.Vec2{stock[0], stock[3], stock[2], stock[1]}
	if e = b.horizontal(outer, nil, 0, true, 0); e != nil {
		return out, e
	}
	if e = b.horizontal(outer, nil, p.Height, false, 0); e != nil {
		return out, e
	}
	for i, u := range outer {
		if e = b.wall(u, outer[(i+1)%4], 0, p.Height); e != nil {
			return out, e
		}
	}
	floor := p.Height - p.Depth
	if e = b.addRectanglePocket(1, extrusionRectangleRing(ExtrusionRectangle{Min: p.PocketMin, Max: p.PocketMax}), floor, p.Height); e != nil {
		return out, e
	}
	out.OuterTopFace, out.FloorFace = 1, len(b.faces)-1
	h := ExtrusionCircle{Center: p.IslandCenter, Radius: p.IslandRadius}
	rim, e := mixedCardinalPoints(h.Center, h.Radius)
	if e != nil {
		return out, e
	}
	points := make([]geom.Vec3, 4)
	uv := []geom.Vec2{rim[0], rim[3], rim[2], rim[1]}
	curves := map[int]PCurve{}
	for i, u := range uv {
		points[i] = geom.Vec3{X: u.X, Y: u.Y, Z: floor}
		curves[i], e = extrusionArc(h, i, u, uv[(i+1)%4], true)
		if e != nil {
			return out, e
		}
	}
	loop, e := b.wire(b.faces[out.FloorFace].Surface, points, uv, curves)
	if e != nil {
		return out, e
	}
	b.faces[out.FloorFace].Holes = append(b.faces[out.FloorFace].Holes, loop)
	capFrame, e := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{Z: p.Height}), p.Frame.XAxis(), p.Frame.ZAxis())
	if e != nil {
		return out, e
	}
	cap, e := geom.NewPlaneSurface(capFrame)
	if e != nil {
		return out, e
	}
	uv = append([]geom.Vec2(nil), rim[:]...)
	curves = map[int]PCurve{}
	for i, u := range uv {
		points[i] = geom.Vec3{X: u.X, Y: u.Y, Z: p.Height}
		curves[i], e = extrusionArc(h, i, u, uv[(i+1)%4], false)
		if e != nil {
			return out, e
		}
	}
	out.IslandTopFace = len(b.faces)
	if e = b.face(cap, points, uv, curves, false); e != nil {
		return out, e
	}
	cylinderFrame, e := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y}), p.Frame.XAxis(), p.Frame.ZAxis())
	if e != nil {
		return out, e
	}
	wall, e := geom.NewCylinderSurface(cylinderFrame, h.Radius)
	if e != nil {
		return out, e
	}
	for i, u := range rim {
		v := rim[(i+1)%4]
		a, z := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		xyz := []geom.Vec3{{X: u.X, Y: u.Y, Z: floor}, {X: v.X, Y: v.Y, Z: floor}, {X: v.X, Y: v.Y, Z: p.Height}, {X: u.X, Y: u.Y, Z: p.Height}}
		chart := []geom.Vec2{{X: a, Y: floor}, {X: z, Y: floor}, {X: z, Y: p.Height}, {X: a, Y: p.Height}}
		if e = b.face(wall, xyz, chart, nil, false); e != nil {
			return out, e
		}
	}
	out.Solid, e = NewSolidContext(ctx, b.vertices, b.edges, b.faces, tol)
	return out, e
}
