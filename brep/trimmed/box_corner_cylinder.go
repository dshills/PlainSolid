package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// cornerCylinderBuilder collects the vertices, edges and faces of a solid
// of planes and cylinders that a constructor lays out face by face.
type cornerCylinderBuilder struct {
	ctx             context.Context
	frame           geom.Frame
	vertices        []geom.Vec3
	edges           []parametric.Edge
	faces           []Face
	verticesByPoint map[geom.Vec3]int
	edgesByEnds     map[[2]int]int
}

func (b *cornerCylinderBuilder) vertex(p geom.Vec3) (int, error) {
	w := b.frame.ToWorld(p)
	if !w.IsFinite() || b.frame.ToLocal(w) != p {
		return 0, geom.ErrUnrepresentable
	}
	if id, ok := b.verticesByPoint[w]; ok {
		return id, nil
	}
	id := len(b.vertices)
	b.vertices = append(b.vertices, w)
	b.verticesByPoint[w] = id
	return id, nil
}
func (b *cornerCylinderBuilder) face(surface geom.Surface, points []geom.Vec3, uv []geom.Vec2, curves map[int]PCurve, reversed bool) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	loop, err := b.wire(surface, points, uv, curves)
	if err != nil {
		return err
	}
	b.faces = append(b.faces, Face{Surface: surface, Reversed: reversed, Outer: loop})
	return nil
}

func (b *cornerCylinderBuilder) wire(surface geom.Surface, points []geom.Vec3, uv []geom.Vec2, curves map[int]PCurve) ([]Use, error) {
	ids := make([]int, len(points))
	for i, p := range points {
		id, e := b.vertex(p)
		if e != nil {
			return nil, e
		}
		ids[i] = id
	}
	loop := make([]Use, 0, len(ids))
	for i, a := range ids {
		j := (i + 1) % len(ids)
		z := ids[j]
		key := [2]int{min(a, z), max(a, z)}
		id, exists := b.edgesByEnds[key]
		if !exists {
			id = len(b.edges)
			b.edgesByEnds[key] = id
			b.edges = append(b.edges, parametric.Edge{Start: a, End: z, Curve: surfaceTrimCurve{surface: surface, a: uv[i], b: uv[j], curve: curves[i]}, Range: parametric.Interval{Max: 1}})
		}
		loop = append(loop, Use{Edge: id, Reversed: b.edges[id].Start != a, UVStart: uv[i], UVEnd: uv[j], Curve: curves[i]})
	}
	return loop, nil
}
func (b *cornerCylinderBuilder) horizontal(profile []geom.Vec2, arcs map[int][2]float64, z float64, reversed bool, radius float64) error {
	f, e := geom.NewFrame(b.frame.ToWorld(geom.Vec3{Z: z}), b.frame.XAxis(), b.frame.ZAxis())
	if e != nil {
		return e
	}
	surface, e := geom.NewPlaneSurface(f)
	if e != nil {
		return e
	}
	points := make([]geom.Vec3, len(profile))
	curves := map[int]PCurve{}
	for i, p := range profile {
		points[i] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
		if arc, ok := arcs[i]; ok {
			sin := radius
			if arc[1] < arc[0] {
				sin = -radius
				arc = [2]float64{2*math.Pi - arc[0], 2*math.Pi - arc[1]}
			}
			c, e := newAnchoredCircularUV(EllipseUV{Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: sin}, From: arc[0], To: arc[1]}, p, profile[(i+1)%len(profile)])
			if e != nil {
				return e
			}
			curves[i] = c
		}
	}
	return b.face(surface, points, profile, curves, reversed)
}
func (b *cornerCylinderBuilder) wall(p, q geom.Vec2, lo, hi float64) error {
	points := []geom.Vec3{{X: p.X, Y: p.Y, Z: lo}, {X: q.X, Y: q.Y, Z: lo}, {X: q.X, Y: q.Y, Z: hi}, {X: p.X, Y: p.Y, Z: hi}}
	world := b.frame.ToWorld(points[0])
	dx := b.frame.ToWorld(points[1]).Sub(world)
	normal := dx.Cross(b.frame.ZAxis())
	f, e := geom.NewFrame(world, dx, normal)
	if e != nil {
		return e
	}
	surface, e := geom.NewPlaneSurface(f)
	if e != nil {
		return e
	}
	uv := make([]geom.Vec2, len(points))
	for i, p := range points {
		v := f.ToLocal(b.frame.ToWorld(p))
		uv[i] = geom.Vec2{X: v.X, Y: v.Y}
	}
	return b.face(surface, points, uv, nil, false)
}
func (b *cornerCylinderBuilder) cylinderBand(lo, hi, bottom, top, radius float64, reversed bool, p, q geom.Vec2) error {
	surface, e := geom.NewCylinderSurface(b.frame, radius)
	if e != nil {
		return e
	}
	points := []geom.Vec3{{X: p.X, Y: p.Y, Z: bottom}, {X: q.X, Y: q.Y, Z: bottom}, {X: q.X, Y: q.Y, Z: top}, {X: p.X, Y: p.Y, Z: top}}
	uv := []geom.Vec2{{X: lo, Y: bottom}, {X: hi, Y: bottom}, {X: hi, Y: top}, {X: lo, Y: top}}
	return b.face(surface, points, uv, nil, reversed)
}
