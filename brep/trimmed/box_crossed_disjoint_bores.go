package trimmed

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

// CardinalThroughBore describes a local cardinal through axis. Axis is0,1,2
// for X,Y,Z; Center's axial coordinate must be zero. Radius is positive.
type CardinalThroughBore struct {
	Axis   int
	Center geom.Vec3
	Radius float64
}

const MaxCrossedDisjointBores = 16

// BoxCrossedBoresWithDisjoint preserves the equal-radius crossed saddle and
// adds cardinal through holes whose padded complete envelopes are disjoint
// from both crossing cylinders and each other. Faces0..21 remain stable;
// added four-wall groups follow canonical axis/center/radius order.
func BoxCrossedBoresWithDisjoint(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius float64, additional []CardinalThroughBore, tolerance float64) (BoxCrossedBoresResult, error) {
	fail := func(e error) (BoxCrossedBoresResult, error) { return BoxCrossedBoresResult{}, e }
	if ctx == nil || len(additional) == 0 || len(additional) > MaxCrossedDisjointBores || !finite(tolerance) || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	stock, e := BoxCrossedBores(ctx, frame, dimensions, crossing, radius, tolerance)
	if e != nil {
		return fail(e)
	}
	core := []CardinalThroughBore{{Axis: 0, Center: geom.Vec3{Y: crossing.Y, Z: crossing.Z}, Radius: radius}, {Axis: 2, Center: geom.Vec3{X: crossing.X, Y: crossing.Y}, Radius: radius}}
	return appendCrossedDisjointBores(ctx, stock, frame, dimensions, core, additional, tolerance)
}

// appendCrossedDisjointBores merges independent through donors onto a freshly
// constructed crossed core. Its first six faces are the original stock planes.
// Core envelopes conservatively cover the entire projected crossing tools,
// including when the retained core is blind. No curved intersections are added.
func appendCrossedDisjointBores(ctx context.Context, stock BoxCrossedBoresResult, frame geom.Frame, dimensions geom.Vec3, core []CardinalThroughBore, additional []CardinalThroughBore, tolerance float64) (BoxCrossedBoresResult, error) {
	fail := func(e error) (BoxCrossedBoresResult, error) { return BoxCrossedBoresResult{}, e }
	var e error
	sites := append([]CardinalThroughBore(nil), additional...)
	component := func(p geom.Vec3, i int) float64 { return []float64{p.X, p.Y, p.Z}[i] }
	set := func(p *geom.Vec3, i int, v float64) {
		switch i {
		case 0:
			p.X = v
		case 1:
			p.Y = v
		case 2:
			p.Z = v
		}
	}
	envelope := func(site CardinalThroughBore) (geom.Box, error) {
		if site.Axis < 0 || site.Axis > 2 || !site.Center.IsFinite() || !finite(site.Radius) || site.Radius <= 8*tolerance || component(site.Center, site.Axis) != 0 {
			return geom.Box{}, geom.ErrDomain
		}
		lo, hi := site.Center, site.Center
		for i := 0; i < 3; i++ {
			a, b := 0., component(dimensions, i)
			if i != site.Axis {
				a = component(site.Center, i) - site.Radius
				b = component(site.Center, i) + site.Radius
				if a <= 8*tolerance || component(dimensions, i)-b <= 8*tolerance {
					return geom.Box{}, geom.ErrDomain
				}
			}
			pad := tolerance + 128*0x1p-52*max(math.Abs(a), math.Abs(b), 1)
			set(&lo, i, math.Nextafter(a-pad, math.Inf(-1)))
			set(&hi, i, math.Nextafter(b+pad, math.Inf(1)))
		}
		box := geom.Box{Min: lo, Max: hi}
		if !box.Valid() {
			return geom.Box{}, geom.ErrInvalid
		}
		return box, nil
	}

	var bounds []geom.Box
	for _, c := range core {
		b, e := envelope(c)
		if e != nil {
			return fail(e)
		}
		bounds = append(bounds, b)
	}
	for _, c := range sites {
		if frame.ToLocal(frame.ToWorld(c.Center)) != c.Center {
			return fail(geom.ErrUnrepresentable)
		}
		if e = ctx.Err(); e != nil {
			return fail(e)
		}
		box, e := envelope(c)
		if e != nil {
			return fail(e)
		}
		for _, old := range bounds {
			if _, hit := box.Intersect(old); hit {
				return fail(geom.ErrDomain)
			}
		}
		bounds = append(bounds, box)
	}
	sort.Slice(sites, func(i, j int) bool {
		a, b := sites[i], sites[j]
		if a.Axis != b.Axis {
			return a.Axis < b.Axis
		}
		for k := 0; k < 3; k++ {
			x, y := component(a.Center, k), component(b.Center, k)
			if x != y {
				return x < y
			}
		}
		return a.Radius < b.Radius
	})
	if frame.ToLocal(frame.ToWorld(dimensions)) != dimensions {
		return fail(geom.ErrUnrepresentable)
	}
	box, e := geom.Bounds([]geom.Vec3{frame.Origin(), frame.ToWorld(dimensions)})
	if e != nil {
		return fail(e)
	}
	base, e := brep.Box(box.Min, box.Max)
	if e != nil {
		return fail(e)
	}
	planar, e := FromPlanar(base, tolerance)
	if e != nil {
		return fail(e)
	}
	mapCaps := make([]int, 6)
	for i, f := range planar.faces {
		p := f.Surface.(geom.PlaneSurface).Frame()
		matched := -1
		for j, g := range stock.Solid.faces[:6] {
			q := g.Surface.(geom.PlaneSurface).Frame()
			if p.ZAxis() == q.ZAxis() && p.Origin().Sub(q.Origin()).Dot(q.ZAxis()) == 0 {
				if matched >= 0 {
					return fail(geom.ErrDomain)
				}
				matched = j
			}
		}
		if matched < 0 {
			return fail(geom.ErrDomain)
		}
		mapCaps[i] = matched
	}
	cuts := make([]CylinderCut, len(sites))
	axes := []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()}
	for i, c := range sites {
		direction := axes[c.Axis]
		entry, exit := -1, -1
		for j, f := range planar.faces {
			normal := f.Surface.(geom.PlaneSurface).Frame().ZAxis()
			if normal == direction.Scale(-1) {
				entry = j
			}
			if normal == direction {
				exit = j
			}
		}
		if entry < 0 || exit < 0 {
			return fail(geom.ErrDomain)
		}
		cuts[i] = CylinderCut{Entry: entry, Exit: exit, AxisPoint: frame.ToWorld(c.Center), Direction: direction, Radius: c.Radius}
	}
	donor, _, e := BoreCylinders(base, cuts, tolerance)
	if e != nil {
		return fail(e)
	}
	vertices := append(stock.Solid.Vertices(), donor.Vertices()...)
	edges, faces := stock.Solid.Edges(), stock.Solid.Faces()
	eo, vo := len(edges), len(stock.Solid.vertices)
	for _, edge := range donor.edges {
		edge.Start += vo
		edge.End += vo
		edges = append(edges, edge)
	}
	for i, source := range donor.faces[:6] {
		target := &faces[mapCaps[i]]
		from, to := source.Surface.(geom.PlaneSurface).Frame(), target.Surface.(geom.PlaneSurface).Frame()
		project := func(p geom.Vec2) geom.Vec2 {
			v := to.ToLocal(from.ToWorld(geom.Vec3{X: p.X, Y: p.Y}))
			return geom.Vec2{X: v.X, Y: v.Y}
		}
		vector := func(p geom.Vec2) geom.Vec2 {
			v := from.XAxis().Scale(p.X).Add(from.YAxis().Scale(p.Y))
			return geom.Vec2{X: v.Dot(to.XAxis()), Y: v.Dot(to.YAxis())}
		}
		for _, loop := range source.Holes {
			out := append([]Use(nil), loop...)
			for j := range out {
				u := &out[j]
				c, ok := u.Curve.(EllipseUV)
				if !ok {
					return fail(geom.ErrDomain)
				}
				c.Center = project(c.Center)
				c.Cos = vector(c.Cos)
				c.Sin = vector(c.Sin)
				c.Linear = vector(c.Linear)
				u.Edge += eo
				u.UVStart, u.UVEnd = project(u.UVStart), project(u.UVEnd)
				u.Curve = unionArcUV{c, u.UVStart, u.UVEnd}
			}
			if source.Reversed != target.Reversed {
				out = reverseUnionLoop(out)
			}
			target.Holes = append(target.Holes, out)
		}
	}
	for _, f := range donor.faces[6:] {
		f.Outer = append([]Use(nil), f.Outer...)
		if len(f.Holes) != 0 {
			return fail(geom.ErrDomain)
		}
		for i := range f.Outer {
			u := &f.Outer[i]
			u.Edge += eo
			// A cardinal opening produces exactly affine horizontal cylinder trims.
			// Normalize only zero-trigonometric coefficients, keeping model edges.
			if c, ok := u.Curve.(EllipseUV); ok && c.Cos == (geom.Vec2{}) && c.Sin == (geom.Vec2{}) {
				u.Curve = nil
			}
		}
		faces = append(faces, f)
	}
	solid, e := compactCrossedDonor(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return fail(e)
	}
	removed, delta := stock.RemovedVolume, stock.SurfaceAreaDelta
	for _, c := range sites {
		length := component(dimensions, c.Axis)
		removed += math.Pi * c.Radius * c.Radius * length
		delta += 2*math.Pi*c.Radius*length - 2*math.Pi*c.Radius*c.Radius
	}
	if !finite(removed) || !finite(delta) {
		return fail(geom.ErrInvalid)
	}
	return BoxCrossedBoresResult{Solid: solid, RemovedVolume: removed, SurfaceAreaDelta: delta}, nil
}

func (s Solid) TessellateCrossedWithDisjoint(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	if e := ctx.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 128
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if len(s.faces) < 26 || len(s.faces) > 22+4*MaxCrossedDisjointBores || (len(s.faces)-22)%4 != 0 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	found := false
	for _, f := range s.faces {
		base, _ := parametric.SurfaceMapping(f.Surface)
		switch base.(type) {
		case geom.PlaneSurface, geom.CylinderSurface:
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				switch u.Curve.(type) {
				case unionArcUV:
					found = true
				case nil, EllipseUV:
				default:
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
		}
	}
	if !found {
		return mesh.Mesh{}, geom.ErrDomain
	}
	if m, ok, e := s.compactCylinderMesh(ctx, o); ok {
		return m, e
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}

// Remove unused donor stock entities in original index order, preserving all
// retained saddle vertex/edge IDs while retaining stable face IDs.
func compactCrossedDonor(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tolerance float64) (Solid, error) {
	usedE, usedV := make([]bool, len(edges)), make([]bool, len(vertices))
	for _, f := range faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				usedE[u.Edge] = true
			}
		}
	}
	for i, e := range edges {
		if usedE[i] {
			usedV[e.Start], usedV[e.End] = true, true
		}
	}
	vm, em := make([]int, len(vertices)), make([]int, len(edges))
	var outV []geom.Vec3
	var outE []parametric.Edge
	for i, v := range vertices {
		if usedV[i] {
			vm[i] = len(outV)
			outV = append(outV, v)
		}
	}
	for i, e := range edges {
		if usedE[i] {
			em[i] = len(outE)
			e.Start, e.End = vm[e.Start], vm[e.End]
			outE = append(outE, e)
		}
	}
	for i := range faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		for _, loop := range append([][]Use{faces[i].Outer}, faces[i].Holes...) {
			for j := range loop {
				loop[j].Edge = em[loop[j].Edge]
			}
		}
	}
	return newSolidContext(ctx, outV, outE, faces, tolerance, true)
}
