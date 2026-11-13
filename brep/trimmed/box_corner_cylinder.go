package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BoxCornerCylinder cuts or fuses a finite cylinder centered at the lower-left
// radial corner of rectangular stock [0,width]×[0,height]×[0,length] in frame.
// Its axial interval is [0,depth], strictly shorter than the stock. The radius
// must leave both opposite radial sides clear. The result retains analytic
// cylindrical walls and circular cap trims, including the internal cut floor
// or external union shoulder. This is a bounded constructor, not general CSG.
func BoxCornerCylinder(ctx context.Context, frame geom.Frame, width, height, length, radius, depth float64, fuse bool, tolerance float64) (Solid, error) {
	if ctx == nil || !finite(width) || !finite(height) || !finite(length) || !finite(radius) || !finite(depth) || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	pad := 8 * tolerance
	if !finite(pad) || radius <= pad || depth <= pad || length-depth <= pad || width-radius <= pad || height-radius <= pad {
		return Solid{}, geom.ErrDomain
	}
	if frame.XAxis().Length() == 0 || frame.ZAxis().Length() == 0 {
		return Solid{}, geom.ErrInvalid
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	// Counterclockwise outer profile below the finite cylinder endpoint.
	o, a, c, d, e := geom.Vec2{}, geom.Vec2{X: radius}, geom.Vec2{X: width}, geom.Vec2{X: width, Y: height}, geom.Vec2{Y: height}
	rim := geom.Vec2{Y: radius}
	lower := []geom.Vec2{a, c, d, e, rim}
	arcs := map[int][2]float64{4: {math.Pi / 2, 0}}
	if fuse {
		lower = append(lower, geom.Vec2{X: -radius}, geom.Vec2{Y: -radius})
		arcs = map[int][2]float64{4: {math.Pi / 2, math.Pi}, 5: {math.Pi, 3 * math.Pi / 2}, 6: {3 * math.Pi / 2, 2 * math.Pi}}
	}
	upper := []geom.Vec2{o, a, c, d, e, rim}
	if err := b.horizontal(lower, arcs, 0, true, radius); err != nil {
		return Solid{}, err
	}
	if err := b.horizontal(upper, nil, length, false, radius); err != nil {
		return Solid{}, err
	}
	if fuse {
		if err := b.horizontal([]geom.Vec2{o, rim, {X: -radius}, {Y: -radius}, a}, map[int][2]float64{1: {math.Pi / 2, math.Pi}, 2: {math.Pi, 3 * math.Pi / 2}, 3: {3 * math.Pi / 2, 2 * math.Pi}}, depth, false, radius); err != nil {
			return Solid{}, err
		}
	} else {
		if err := b.horizontal([]geom.Vec2{o, a, rim}, map[int][2]float64{1: {0, math.Pi / 2}}, depth, true, radius); err != nil {
			return Solid{}, err
		}
	}
	for i, p := range lower {
		q := lower[(i+1)%len(lower)]
		if arc, ok := arcs[i]; ok {
			lo, hi := math.Min(arc[0], arc[1]), math.Max(arc[0], arc[1])
			if err := b.cylinder(lo, hi, depth, radius, !fuse, p, q); err != nil {
				return Solid{}, err
			}
		} else {
			if err := b.wall(p, q, 0, depth); err != nil {
				return Solid{}, err
			}
		}
	}
	for i, p := range upper {
		if err := b.wall(p, upper[(i+1)%len(upper)], depth, length); err != nil {
			return Solid{}, err
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}

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
func (b *cornerCylinderBuilder) cylinder(lo, hi, depth, radius float64, reversed bool, p, q geom.Vec2) error {
	if reversed {
		p, q = q, p
	}
	return b.cylinderBand(lo, hi, 0, depth, radius, reversed, p, q)
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
