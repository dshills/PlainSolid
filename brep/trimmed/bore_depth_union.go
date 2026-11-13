package trimmed

import (
	"fmt"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// LayeredBoreHistory records exact analytic slab measures and retained face IDs.
// FloorAreas corresponds to FloorFaces, including annular or crescent floors.
type LayeredBoreHistory struct {
	Entry, Exit                                                        int
	RemovedVolume, SurfaceAreaDelta, EntryRemovedArea, ExitRemovedArea float64
	FloorFaces                                                         []int
	FloorAreas                                                         []float64
}

// BoreCylinderDepthUnion removes parallel cylinders entering either of two
// opposite caps of convex stock. A zero depth reaches the other cap; positive
// depths stop inside. It preserves analytic arcs and cylinder surfaces across
// axial depth layers.
func BoreCylinderDepthUnion(base brep.Solid, cuts []CylinderCut, depths []float64, tolerance float64) (Solid, LayeredBoreHistory, error) {
	fail := func(s string) (Solid, LayeredBoreHistory, error) {
		return Solid{}, LayeredBoreHistory{}, fmt.Errorf("trimmed: layered cylinder bore: %s", s)
	}
	if len(cuts) == 0 || len(cuts) > 32 || len(depths) != len(cuts) || !finite(tolerance) || tolerance <= 0 {
		return fail("invalid inputs")
	}
	entry, exit := cuts[0].Entry, cuts[0].Exit
	span := 0.
	var disks []depthDisk
	for i, c := range cuts {
		opposed := c.Entry == exit && c.Exit == entry
		if opposed {
			// Qualify the original axis against its authored entry before
			// using the common chart for this tool's circular footprint.
			if _, _, err := BoreCylinderAlong(base, c.Entry, c.Exit, c.AxisPoint, c.Direction, c.Radius, tolerance); err != nil {
				return Solid{}, LayeredBoreHistory{}, err
			}
			c.Direction = c.Direction.Scale(-1)
		} else if c.Entry != entry || c.Exit != exit {
			return fail("different opening faces")
		}
		if !finite(depths[i]) || depths[i] < 0 {
			return fail("invalid depth")
		}
		proof, h, err := BoreCylinderAlong(base, entry, exit, c.AxisPoint, c.Direction, c.Radius, tolerance)
		if err != nil {
			return Solid{}, LayeredBoreHistory{}, err
		}
		wall := proof.Faces()[h.WallFaces[0]]
		surface := wall.Surface.(geom.CylinderSurface)
		f, _ := surface.Parameters()
		for _, u := range wall.Outer {
			if u.Curve != nil {
				return fail("oblique openings")
			}
		}
		for _, id := range []int{entry, exit} {
			p, e := base.FacePlane(id)
			if e != nil {
				return Solid{}, LayeredBoreHistory{}, e
			}
			if unionDotSign(f.XAxis(), p.Normal()) != 0 || unionDotSign(f.YAxis(), p.Normal()) != 0 {
				return fail("unproved perpendicular caps")
			}
		}
		length := wall.Outer[2].UVStart.Y - wall.Outer[0].UVStart.Y
		if len(disks) > 0 && (f.ZAxis() != disks[0].frame.ZAxis() || length != span) {
			return fail("unequal represented axes or spans")
		}
		span = length
		depth := depths[i]
		if depth == 0 {
			depth = span
		} else if depth <= tolerance || span-depth <= tolerance {
			return fail("unresolved depth or remaining wall")
		}
		start := 0.
		if opposed {
			start, depth = span-depth, span
		}
		disks = append(disks, depthDisk{unionDisk: unionDisk{radius: c.Radius, frame: f}, start: start, depth: depth})
	}
	sort.Slice(disks, func(i, j int) bool {
		a, b := disks[i].frame.Origin(), disks[j].frame.Origin()
		if a.X != b.X {
			return a.X < b.X
		}
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		if a.Z != b.Z {
			return a.Z < b.Z
		}
		return disks[i].radius > disks[j].radius
	})
	frame := disks[0].frame
	for i := range disks {
		p := frame.ToLocal(disks[i].frame.Origin())
		p.Z = 0
		if frame.ToWorld(p) != disks[i].frame.Origin() {
			return fail("unresolved represented disk coordinates")
		}
		disks[i].center = geom.Vec2{X: p.X, Y: p.Y}
	}
	arrangement, err := newDepthArrangement(disks, tolerance)
	if err != nil {
		return Solid{}, LayeredBoreHistory{}, err
	}
	levels := []float64{0, span}
	for _, d := range arrangement.disks {
		levels = append(levels, d.start, d.depth)
	}
	sort.Float64s(levels)
	unique := levels[:0]
	for _, z := range levels {
		if len(unique) == 0 || z != unique[len(unique)-1] {
			unique = append(unique, z)
		}
	}
	levels = unique
	for i := 1; i < len(levels); i++ {
		if levels[i]-levels[i-1] <= tolerance {
			return fail("unresolved adjacent depth levels")
		}
	}
	stock, err := FromPlanar(base, tolerance)
	if err != nil {
		return Solid{}, LayeredBoreHistory{}, err
	}
	vertices, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	history := LayeredBoreHistory{Entry: entry, Exit: exit}
	visible := make([][]bool, len(levels)-1)
	boundaries := make([]depthBoundary, len(visible))
	wallArea := 0.
	for i := range visible {
		visible[i], err = arrangement.exposed(levels[i], levels[i+1], tolerance)
		if err != nil {
			return Solid{}, history, err
		}
		var list []directedUnionArc
		for id, on := range visible[i] {
			if on {
				list = append(list, directedUnionArc{id: id})
			}
		}
		boundaries[i], err = arrangement.boundary(list, false, tolerance)
		if err != nil {
			return Solid{}, history, err
		}
		dz := levels[i+1] - levels[i]
		history.RemovedVolume += boundaries[i].area * dz
		wallArea += boundaries[i].perimeter * dz
	}
	history.EntryRemovedArea = boundaries[0].area
	history.ExitRemovedArea = boundaries[len(boundaries)-1].area
	type pointKey struct{ node, level int }
	type arcKey struct{ arc, level int }
	type seamKey struct{ node, slab int }
	pointIDs := map[pointKey]int{}
	arcIDs := map[arcKey]int{}
	seamIDs := map[seamKey]int{}
	point := func(node, level int) int {
		k := pointKey{node, level}
		if id, ok := pointIDs[k]; ok {
			return id
		}
		p := arrangement.points[node]
		id := len(vertices)
		vertices = append(vertices, frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y, Z: levels[level]}))
		pointIDs[k] = id
		return id
	}
	arcEdge := func(id, level int) (int, error) {
		k := arcKey{id, level}
		if e, ok := arcIDs[k]; ok {
			return e, nil
		}
		a := arrangement.arcs[id]
		d := arrangement.disks[a.disk]
		surface, e := geom.NewCylinderSurface(d.frame, d.radius)
		if e != nil {
			return 0, e
		}
		curve := surfaceTrimCurve{surface: surface, curve: EllipseUV{Center: geom.Vec2{Y: levels[level]}, Linear: geom.Vec2{X: 1}, From: a.from, To: a.to}}
		start, end := point(a.start, level), point(a.end, level)
		p, e0 := curve.Eval(0)
		q, e1 := curve.Eval(1)
		if e0 != nil || e1 != nil || !closePoint(p, vertices[start], tolerance) || !closePoint(q, vertices[end], tolerance) {
			return 0, fmt.Errorf("trimmed: layer arc endpoints exceed model tolerance")
		}
		eid := len(edges)
		edges = append(edges, parametric.Edge{Start: start, End: end, Curve: curve, Range: parametric.Interval{Max: 1}})
		arcIDs[k] = eid
		return eid, nil
	}
	seamEdge := func(node, slab int) (int, error) {
		k := seamKey{node, slab}
		if e, ok := seamIDs[k]; ok {
			return e, nil
		}
		start, end := point(node, slab), point(node, slab+1)
		line, e := geom.NewLine(vertices[start], frame.ZAxis())
		if e != nil {
			return 0, e
		}
		id := len(edges)
		edges = append(edges, parametric.Edge{Start: start, End: end, Curve: line, Range: parametric.Interval{Max: levels[slab+1] - levels[slab]}})
		seamIDs[k] = id
		return id, nil
	}
	planeLoop := func(loop []directedUnionArc, level int, plane geom.PlaneSurface) ([]Use, error) {
		uses := make([]Use, 0, len(loop))
		for _, directed := range loop {
			a := arrangement.arcs[directed.id]
			d := arrangement.disks[a.disk]
			eid, e := arcEdge(directed.id, level)
			if e != nil {
				return nil, e
			}
			from, to := a.from, a.to
			start, end := a.start, a.end
			if directed.reverse {
				from, to = to, from
				start, end = end, start
			}
			center := d.frame.Origin().Add(frame.ZAxis().Scale(levels[level]))
			c := plane.Frame().ToLocal(center)
			x, y := d.frame.XAxis().Scale(d.radius), d.frame.YAxis().Scale(d.radius)
			arc := EllipseUV{Center: geom.Vec2{X: c.X, Y: c.Y}, Cos: geom.Vec2{X: x.Dot(plane.Frame().XAxis()), Y: x.Dot(plane.Frame().YAxis())}, Sin: geom.Vec2{X: y.Dot(plane.Frame().XAxis()), Y: y.Dot(plane.Frame().YAxis())}, From: from, To: to}
			p, q := plane.Frame().ToLocal(vertices[point(start, level)]), plane.Frame().ToLocal(vertices[point(end, level)])
			u, v := geom.Vec2{X: p.X, Y: p.Y}, geom.Vec2{X: q.X, Y: q.Y}
			raw, e0 := arc.EvalUV(0)
			last, e1 := arc.EvalUV(1)
			p0, e2 := plane.Eval(raw.X, raw.Y)
			p1, e3 := plane.Eval(last.X, last.Y)
			if e0 != nil || e1 != nil || e2 != nil || e3 != nil || !closePoint(p0, vertices[point(start, level)], tolerance) || !closePoint(p1, vertices[point(end, level)], tolerance) {
				return nil, fmt.Errorf("trimmed: layer UV endpoints exceed model tolerance")
			}
			uses = append(uses, Use{Edge: eid, Reversed: directed.reverse, UVStart: u, UVEnd: v, Curve: unionArcUV{EllipseUV: arc, Start: u, End: v}})
		}
		return uses, nil
	}
	// Each global arrangement arc uses the same endpoints at every depth level.
	for slab, on := range visible {
		for id, exposed := range on {
			if !exposed {
				continue
			}
			if len(faces) > 4096 {
				return fail("layered face work limit")
			}
			a := arrangement.arcs[id]
			d := arrangement.disks[a.disk]
			bottom, e := arcEdge(id, slab)
			if e != nil {
				return Solid{}, history, e
			}
			top, e := arcEdge(id, slab+1)
			if e != nil {
				return Solid{}, history, e
			}
			right, e := seamEdge(a.end, slab)
			if e != nil {
				return Solid{}, history, e
			}
			left, e := seamEdge(a.start, slab)
			if e != nil {
				return Solid{}, history, e
			}
			surface, e := geom.NewCylinderSurface(d.frame, d.radius)
			if e != nil {
				return Solid{}, history, e
			}
			p := [4]geom.Vec2{{X: a.from, Y: levels[slab]}, {X: a.to, Y: levels[slab]}, {X: a.to, Y: levels[slab+1]}, {X: a.from, Y: levels[slab+1]}}
			ids := [4]int{bottom, right, top, left}
			uses := make([]Use, 4)
			for j, id := range ids {
				uses[j] = Use{Edge: id, Reversed: j >= 2, UVStart: p[j], UVEnd: p[(j+1)%4]}
			}
			faces = append(faces, Face{Surface: surface, Outer: uses, Reversed: true})
		}
	}
	for _, loop := range boundaries[0].loops {
		uses, e := planeLoop(loop, 0, faces[entry].Surface.(geom.PlaneSurface))
		if e != nil {
			return Solid{}, history, e
		}
		faces[entry].Holes = append(faces[entry].Holes, uses)
	}
	for _, loop := range boundaries[len(boundaries)-1].loops {
		uses, e := planeLoop(loop, len(levels)-1, faces[exit].Surface.(geom.PlaneSurface))
		if e != nil {
			return Solid{}, history, e
		}
		faces[exit].Holes = append(faces[exit].Holes, reverseUnionLoop(uses))
	}
	floorArea, signedFloorArea := 0., 0.
	for level := 1; level < len(levels)-1; level++ {
		starting, stopping := false, false
		for _, disk := range arrangement.disks {
			starting = starting || disk.start == levels[level]
			stopping = stopping || disk.depth == levels[level]
		}
		var floors, ceilings []directedUnionArc
		for id, before := range visible[level-1] {
			after := visible[level][id]
			if before == after {
				continue
			}
			growing, reverse := starting, before == starting
			if starting && stopping {
				// A disappearing arc bounds a stopped floor when the new
				// slab is empty at the arc. If the new slab covers it, the
				// arc instead bounds a growing ceiling on its right.
				lo, hi := levels[level-1], levels[level]
				if before {
					lo, hi = levels[level], levels[level+1]
				}
				covered, e := arrangement.coveredArc(id, lo, hi, tolerance)
				if e != nil {
					return Solid{}, history, e
				}
				growing, reverse = before == covered, covered
			}
			arc := directedUnionArc{id: id, reverse: reverse}
			if growing {
				ceilings = append(ceilings, arc)
			} else {
				floors = append(floors, arc)
			}
		}
		if len(floors)+len(ceilings) == 0 {
			continue
		}
		f, e := geom.NewFrame(frame.Origin().Add(frame.ZAxis().Scale(levels[level])), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, history, e
		}
		plane, e := geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, history, e
		}
		for _, change := range []struct {
			arcs    []directedUnionArc
			growing bool
		}{{floors, false}, {ceilings, true}} {
			boundary, e := arrangement.boundary(change.arcs, true, tolerance)
			if e != nil {
				return Solid{}, history, e
			}
			groups := map[int][]int{}
			for i, area := range boundary.areas {
				if area > 0 {
					groups[i] = nil
				}
			}
			for i, area := range boundary.areas {
				if area >= 0 {
					continue
				}
				arc := arrangement.arcs[boundary.loops[i][0].id]
				disk := arrangement.disks[arc.disk]
				owner := -1
				for _, fraction := range []float64{.5, .25, .75, .375, .625, .125, .875} {
					angle := arc.from + (arc.to-arc.from)*fraction
					point := geom.Vec2{X: disk.center.X + disk.radius*math.Cos(angle), Y: disk.center.Y + disk.radius*math.Sin(angle)}
					candidate := -1
					resolved := true
					for j := range groups {
						inside, e := arrangement.insideLoop(point, boundary.loops[j], tolerance)
						if e != nil {
							resolved = false
							break
						}
						if inside {
							if candidate >= 0 {
								return fail("ambiguous nested floor")
							}
							candidate = j
						}
					}
					if resolved {
						owner = candidate
						break
					}
				}
				if owner < 0 {
					return fail("unresolved or uncontained floor hole")
				}

				groups[owner] = append(groups[owner], i)
			}
			// Iterate original loop order, not map order, for stable face IDs.
			for i := range boundary.loops {
				holes, ok := groups[i]
				if !ok {
					continue
				}
				outer, e := planeLoop(boundary.loops[i], level, plane)
				if e != nil {
					return Solid{}, history, e
				}
				face := Face{Surface: plane, Outer: outer, Reversed: !change.growing}
				area := boundary.areas[i]
				for _, h := range holes {
					loop, e := planeLoop(boundary.loops[h], level, plane)
					if e != nil {
						return Solid{}, history, e
					}
					face.Holes = append(face.Holes, loop)
					area += boundary.areas[h]
				}
				if area <= tolerance*tolerance {
					return fail("unresolved floor material")
				}
				history.FloorFaces = append(history.FloorFaces, len(faces))
				history.FloorAreas = append(history.FloorAreas, area)
				floorArea += area
				if change.growing {
					signedFloorArea -= area
				} else {
					signedFloorArea += area
				}
				faces = append(faces, face)
				if len(faces) > 4096 {
					return fail("layered face work limit")
				}
			}
		}
	}
	history.SurfaceAreaDelta = wallArea + floorArea - history.EntryRemovedArea - history.ExitRemovedArea
	if !finite(history.RemovedVolume) || history.RemovedVolume <= 0 || !finite(history.SurfaceAreaDelta) || !finite(history.EntryRemovedArea) || !finite(history.ExitRemovedArea) {
		return fail("invalid layered material measures")
	}
	if math.Abs(signedFloorArea-(history.EntryRemovedArea-history.ExitRemovedArea)) > 1e-9*math.Max(1, math.Max(history.EntryRemovedArea, history.ExitRemovedArea)) {
		return fail("inconsistent layered floor partition")
	}
	solid, err := NewSolid(vertices, edges, faces, tolerance)
	if err != nil {
		return Solid{}, LayeredBoreHistory{}, err
	}
	return solid, history, nil
}
