package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
)

// FuseAnnularPedestal adds the exposed portion [baseHeight,topHeight] of a
// coaxial annulus to an upward planar stock cap. The caller handles any finite
// overlap below that cap. The cap must be convex and have no existing holes;
// its chord polygon must strictly cover the entire outer disk. Stock bounds
// must lie below this cap, allowing only native bound/sewing padding. Original face
// IDs are retained. The returned index identifies the exposed center disk;
// the next face is the pedestal's top annulus.
func FuseAnnularPedestal(ctx context.Context, stock Solid, cap int, frame geom.Frame, inner, outer, baseHeight, topHeight float64) (Solid, int, error) {
	fail := func(e error) (Solid, int, error) { return Solid{}, -1, e }
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if cap < 0 || cap >= len(stock.faces) || !finite(inner) || !finite(outer) || !finite(baseHeight) || !finite(topHeight) || inner <= 0 || outer <= inner || topHeight <= baseHeight {
		return fail(geom.ErrInvalid)
	}
	face := stock.faces[cap]
	plane, ok := face.Surface.(geom.PlaneSurface)
	if !ok || face.Reversed || len(face.Holes) != 0 || plane.Frame().ZAxis() != frame.ZAxis() {
		return fail(geom.ErrDomain)
	}
	f := plane.Frame()
	worldCenter := frame.ToWorld(geom.Vec3{Z: baseHeight})
	center := f.ToLocal(worldCenter)
	if center.Z != 0 || f.ToWorld(center) != worldCenter {
		return fail(geom.ErrUnrepresentable)
	}
	// Bound the entire original stock below the attachment plane. This
	// deliberately declines cavity floors and other caps beneath overhangs.
	for _, v := range stock.vertices {
		if frame.ToLocal(v).Z > baseHeight {
			return fail(geom.ErrDomain)
		}
	}
	box, e := stock.Bounds()
	if e != nil {
		return fail(e)
	}
	pad := 8*stock.tolerance + 256*0x1p-52*max(math.Abs(box.Min.X), math.Abs(box.Min.Y), math.Abs(box.Min.Z), math.Abs(box.Max.X), math.Abs(box.Max.Y), math.Abs(box.Max.Z), math.Abs(baseHeight))
	for _, x := range []float64{box.Min.X, box.Max.X} {
		for _, y := range []float64{box.Min.Y, box.Max.Y} {
			for _, z := range []float64{box.Min.Z, box.Max.Z} {
				if frame.ToLocal(geom.Vec3{X: x, Y: y, Z: z}).Z > baseHeight+pad {
					return fail(geom.ErrDomain)
				}
			}
		}
	}
	gap := 8 * stock.tolerance
	depth := topHeight - baseHeight
	if min(inner, outer-inner, depth) <= gap {
		return fail(geom.ErrDomain)
	}
	polygon := make([]geom.Vec3, len(face.Outer))
	uvs := make([]geom.Vec2, len(face.Outer))
	for i, u := range face.Outer {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		if !pedestalOutwardArc(u.Curve) {
			return fail(geom.ErrDomain)
		}
		uvs[i] = u.UVStart
		var e error
		polygon[i], e = plane.Eval(u.UVStart.X, u.UVStart.Y)
		if e != nil {
			return fail(e)
		}
	}
	positive := false
	for i, p := range uvs {
		turn, e := geom.Orient2D(p, uvs[(i+1)%len(uvs)], uvs[(i+2)%len(uvs)])
		if e != nil || turn < 0 {
			return fail(geom.ErrDomain)
		}
		positive = positive || turn > 0
	}
	if !positive {
		return fail(geom.ErrDomain)
	}
	disk, e := geom.NewPlanarCircularProfile(worldCenter, frame.ZAxis(), outer+gap)
	if e != nil {
		return fail(e)
	}
	coverage, e := geom.ClassifyDiskPlanarPolygon(ctx, disk, polygon, nil, geom.DiskPolygonOptions{})
	if e != nil {
		return fail(e)
	}
	if !coverage.Complete || coverage.Relation != geom.DiskPolygonStrictlyCovered {
		return fail(geom.ErrDomain)
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: f, vertices: stock.Vertices(), edges: stock.Edges(), faces: stock.Faces(), edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	for i, p := range b.vertices {
		b.verticesByPoint[p] = i
	}
	for i, e := range b.edges {
		b.edgesByEnds[[2]int{min(e.Start, e.End), max(e.Start, e.End)}] = i
	}
	circleLoop := func(surface geom.Surface, r, z float64, clockwise bool) ([]Use, error) {
		h := ExtrusionCircle{Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: r}
		rim, e := mixedCardinalPoints(h.Center, r)
		if e != nil {
			return nil, e
		}
		points := make([]geom.Vec3, 4)
		uv := append([]geom.Vec2(nil), rim[:]...)
		if clockwise {
			uv = []geom.Vec2{rim[0], rim[3], rim[2], rim[1]}
		}
		curves := map[int]PCurve{}
		for i, p := range uv {
			points[i] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
			curves[i], e = extrusionArc(h, i, p, uv[(i+1)%4], clockwise)
			if e != nil {
				return nil, e
			}
		}
		return b.wire(surface, points, uv, curves)
	}
	hole, e := circleLoop(plane, outer, 0, true)
	if e != nil {
		return fail(e)
	}
	b.faces[cap].Holes = append(b.faces[cap].Holes, hole)
	centerLoop, e := circleLoop(plane, inner, 0, false)
	if e != nil {
		return fail(e)
	}
	centerFace := len(b.faces)
	b.faces = append(b.faces, Face{Surface: plane, Outer: centerLoop})
	topFrame, e := geom.NewFrame(f.ToWorld(geom.Vec3{Z: depth}), f.XAxis(), f.ZAxis())
	if e != nil {
		return fail(e)
	}
	top, e := geom.NewPlaneSurface(topFrame)
	if e != nil {
		return fail(e)
	}
	rim, e := circleLoop(top, outer, depth, false)
	if e != nil {
		return fail(e)
	}
	hole, e = circleLoop(top, inner, depth, true)
	if e != nil {
		return fail(e)
	}
	b.faces = append(b.faces, Face{Surface: top, Outer: rim, Holes: [][]Use{hole}})
	cylinderFrame, e := geom.NewFrame(worldCenter, f.XAxis(), f.ZAxis())
	if e != nil {
		return fail(e)
	}
	for _, r := range []float64{outer, inner} {
		surface, e := geom.NewCylinderSurface(cylinderFrame, r)
		if e != nil {
			return fail(e)
		}
		points, e := mixedCardinalPoints(geom.Vec2{X: center.X, Y: center.Y}, r)
		if e != nil {
			return fail(e)
		}
		for i, p := range points {
			q := points[(i+1)%4]
			a, z := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
			xyz := []geom.Vec3{{X: p.X, Y: p.Y}, {X: q.X, Y: q.Y}, {X: q.X, Y: q.Y, Z: depth}, {X: p.X, Y: p.Y, Z: depth}}
			uv := []geom.Vec2{{X: a}, {X: z}, {X: z, Y: depth}, {X: a, Y: depth}}
			if e = b.face(surface, xyz, uv, nil, r == inner); e != nil {
				return fail(e)
			}
		}
	}
	result, e := NewSolidContext(ctx, b.vertices, b.edges, b.faces, stock.tolerance)
	if e != nil {
		return fail(e)
	}
	return result, centerFace, nil
}

// Positive elliptical arcs of at most a half turn lie outside their chord
// polygon. The conservative disk-in-chord proof therefore remains sufficient.
func pedestalOutwardArc(curve PCurve) bool {
	if curve == nil {
		return true
	}
	var c EllipseUV
	switch q := curve.(type) {
	case anchoredCircularUV:
		c = q.EllipseUV
	case EllipseUV:
		c = q
	default:
		return false
	}
	if c.Linear != (geom.Vec2{}) || c.To <= c.From || c.To-c.From > math.Pi/2 {
		return false
	}
	sign, e := geom.Orient2D(geom.Vec2{}, c.Cos, c.Sin)
	return e == nil && sign > 0
}
