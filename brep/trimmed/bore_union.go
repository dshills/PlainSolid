package trimmed

import (
	"fmt"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// CylinderUnionHistory retains original stock face IDs and analytic union
// footprint measures. Depth is the material depth, for both through and blind
// cuts. Distinct blind footprint components have separate planar floor faces.
type CylinderUnionHistory struct {
	Entry, Exit            int
	FloorFaces             []int
	Area, Perimeter, Depth float64
	Blind                  bool
}

// BoreCylinderUnion removes the union of up to32 parallel circular footprints
// between the same perpendicular cap planes of convex planar stock. depth==0
// cuts through; positive depth creates a common blind floor from the entry cap.
// Every complete disk must clear the stock sides. Duplicate/contained disks
// simplify; external tangencies, unresolved triple intersections and footprint
// holes (which can leave material islands) reject explicitly. Original stock
// face/edge/vertex IDs are retained, and input geometry is never modified.
func BoreCylinderUnion(base brep.Solid, cuts []CylinderCut, depth, tolerance float64) (Solid, CylinderUnionHistory, error) {
	fail := func(reason string) (Solid, CylinderUnionHistory, error) {
		return Solid{}, CylinderUnionHistory{}, fmt.Errorf("trimmed: cylindrical union bore: %s", reason)
	}
	if len(cuts) == 0 || len(cuts) > 32 || !finite(depth) || depth < 0 || !finite(tolerance) || tolerance <= 0 {
		return fail("invalid cuts, depth or tolerance")
	}
	entry, exit := cuts[0].Entry, cuts[0].Exit
	var disks []unionDisk
	span := 0.
	for _, c := range cuts {
		if c.Entry != entry || c.Exit != exit {
			return fail("different stock opening faces")
		}
		proved, h, err := BoreCylinderAlong(base, c.Entry, c.Exit, c.AxisPoint, c.Direction, c.Radius, tolerance)
		if err != nil {
			return Solid{}, CylinderUnionHistory{}, err
		}
		wall := proved.Faces()[h.WallFaces[0]]
		surface, ok := wall.Surface.(geom.CylinderSurface)
		if !ok {
			return fail("missing cylindrical proof")
		}
		for _, u := range wall.Outer {
			if u.Curve != nil {
				return fail("oblique cylinder openings")
			}
		}
		cframe, _ := surface.Parameters()
		for _, face := range []int{entry, exit} {
			plane, e := base.FacePlane(face)
			if e != nil {
				return Solid{}, CylinderUnionHistory{}, e
			}
			if unionDotSign(cframe.XAxis(), plane.Normal()) != 0 || unionDotSign(cframe.YAxis(), plane.Normal()) != 0 {
				return fail("represented cap planes are not exactly perpendicular to the axis")
			}
		}
		length := wall.Outer[2].UVStart.Y - wall.Outer[0].UVStart.Y
		if len(disks) > 0 && (cframe.ZAxis() != disks[0].frame.ZAxis() || length != span) {
			return fail("unequal represented axes or spans")
		}
		span = length
		disks = append(disks, unionDisk{radius: c.Radius, frame: cframe})
	}
	if !finite(span) || span <= 2*tolerance {
		return fail("invalid material span")
	}
	blind := depth != 0
	if blind {
		if depth <= tolerance || span-depth <= tolerance {
			return fail("unresolved blind depth or remaining wall")
		}
	} else {
		depth = span
	}
	// A geometry-based anchor makes the arrangement independent of caller order.
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
			return fail("unresolved represented planar disk coordinates")
		}
		disks[i].center = geom.Vec2{X: p.X, Y: p.Y}
	}
	profile, err := diskUnionProfile(disks, tolerance)
	if err != nil {
		return Solid{}, CylinderUnionHistory{}, err
	}
	stock, err := FromPlanar(base, tolerance)
	if err != nil {
		return Solid{}, CylinderUnionHistory{}, err
	}
	vertices, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	ep, eok := faces[entry].Surface.(geom.PlaneSurface)
	xp, xok := faces[exit].Surface.(geom.PlaneSurface)
	if !eok || !xok {
		return fail("stock caps are not planes")
	}
	endPlane := xp
	if blind {
		f, e := geom.NewFrame(xp.Frame().Origin().Add(frame.ZAxis().Scale(depth-span)), xp.Frame().XAxis(), xp.Frame().ZAxis())
		if e != nil {
			return Solid{}, CylinderUnionHistory{}, e
		}
		endPlane, e = geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, CylinderUnionHistory{}, e
		}
	}
	history := CylinderUnionHistory{Entry: entry, Exit: exit, Area: profile.area, Perimeter: profile.perimeter, Depth: depth, Blind: blind}
	pointIDs := map[int][2]int{}
	addPoint := func(id int) [2]int {
		if pair, ok := pointIDs[id]; ok {
			return pair
		}
		p := profile.points[id]
		pair := [2]int{len(vertices), len(vertices) + 1}
		vertices = append(vertices, frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y}), frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y, Z: depth}))
		pointIDs[id] = pair
		return pair
	}
	for _, loop := range profile.loops {
		entryLoop := make([]Use, 0, len(loop))
		exitLoop := make([]Use, 0, len(loop))
		seams := make(map[int]int)
		for _, a := range loop {
			ids := addPoint(a.start)
			line, e := geom.NewLine(vertices[ids[0]], frame.ZAxis())
			if e != nil {
				return Solid{}, CylinderUnionHistory{}, e
			}
			seams[a.start] = len(edges)
			edges = append(edges, parametric.Edge{Start: ids[0], End: ids[1], Curve: line, Range: parametric.Interval{Max: depth}})
		}
		for _, a := range loop {
			disk := profile.disks[a.disk]
			start, end := addPoint(a.start), addPoint(a.end)
			rim := [2]int{}
			for k, plane := range []geom.PlaneSurface{ep, endPlane} {
				center := disk.frame.Origin().Add(frame.ZAxis().Scale(float64(k) * depth))
				local := plane.Frame().ToLocal(center)
				x, y := disk.frame.XAxis().Scale(disk.radius), disk.frame.YAxis().Scale(disk.radius)
				arc := EllipseUV{Center: geom.Vec2{X: local.X, Y: local.Y}, Cos: geom.Vec2{X: x.Dot(plane.Frame().XAxis()), Y: x.Dot(plane.Frame().YAxis())}, Sin: geom.Vec2{X: y.Dot(plane.Frame().XAxis()), Y: y.Dot(plane.Frame().YAxis())}, From: a.from, To: a.to}
				first, e := arc.EvalUV(0)
				if e != nil {
					return Solid{}, CylinderUnionHistory{}, e
				}
				last, e := arc.EvalUV(1)
				if e != nil {
					return Solid{}, CylinderUnionHistory{}, e
				}
				rim[k] = len(edges)
				edges = append(edges, parametric.Edge{Start: start[k], End: end[k], Curve: surfaceTrimCurve{surface: plane, curve: arc}, Range: parametric.Interval{Max: 1}})
				startUV, endUV := plane.Frame().ToLocal(vertices[start[k]]), plane.Frame().ToLocal(vertices[end[k]])
				anchorStart, anchorEnd := geom.Vec2{X: startUV.X, Y: startUV.Y}, geom.Vec2{X: endUV.X, Y: endUV.Y}
				if !closePoint(geom.Vec3{X: first.X, Y: first.Y}, geom.Vec3{X: anchorStart.X, Y: anchorStart.Y}, tolerance) || !closePoint(geom.Vec3{X: last.X, Y: last.Y}, geom.Vec3{X: anchorEnd.X, Y: anchorEnd.Y}, tolerance) {
					return fail("union arc endpoint exceeds model tolerance")
				}
				firstWorld, e0 := plane.Eval(first.X, first.Y)
				lastWorld, e1 := plane.Eval(last.X, last.Y)
				if e0 != nil || e1 != nil || !closePoint(firstWorld, vertices[start[k]], tolerance) || !closePoint(lastWorld, vertices[end[k]], tolerance) {
					return fail("union model arc endpoint exceeds shared vertex tolerance")
				}
				use := Use{Edge: rim[k], UVStart: anchorStart, UVEnd: anchorEnd, Curve: unionArcUV{EllipseUV: arc, Start: anchorStart, End: anchorEnd}}
				if k == 0 {
					entryLoop = append(entryLoop, use)
				} else {
					exitLoop = append(exitLoop, use)
				}
			}
			surface, e := geom.NewCylinderSurface(disk.frame, disk.radius)
			if e != nil {
				return Solid{}, CylinderUnionHistory{}, e
			}
			points := [4]geom.Vec2{{X: a.from}, {X: a.to}, {X: a.to, Y: depth}, {X: a.from, Y: depth}}
			ids := [4]int{rim[0], seams[a.end], rim[1], seams[a.start]}
			uses := make([]Use, 4)
			for j, id := range ids {
				uses[j] = Use{Edge: id, Reversed: j >= 2, UVStart: points[j], UVEnd: points[(j+1)%4]}
			}
			faces = append(faces, Face{Surface: surface, Outer: uses, Reversed: true})
		}
		faces[entry].Holes = append(faces[entry].Holes, entryLoop)
		if blind {
			history.FloorFaces = append(history.FloorFaces, len(faces))
			faces = append(faces, Face{Surface: endPlane, Outer: exitLoop, Reversed: true})
		} else {
			faces[exit].Holes = append(faces[exit].Holes, reverseUnionLoop(exitLoop))
		}
	}
	result, err := NewSolid(vertices, edges, faces, tolerance)
	if err != nil {
		return Solid{}, CylinderUnionHistory{}, err
	}
	return result, history, nil
}

func reverseUnionLoop(loop []Use) []Use {
	out := make([]Use, len(loop))
	for i, u := range loop {
		u.Reversed = !u.Reversed
		u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
		arc := u.Curve.(unionArcUV)
		arc.From, arc.To = arc.To, arc.From
		arc.Start, arc.End = arc.End, arc.Start
		u.Curve = arc
		out[len(loop)-1-i] = u
	}
	return out
}

// unionArcUV keeps analytic ellipse support and derivative while sharing one
// floating endpoint representative at circle-circle intersections. Construction
// proves both representatives agree with the analytic rim within model tolerance.
// Interior evaluation is never displaced or replaced by a chord.
type unionArcUV struct {
	EllipseUV
	Start, End geom.Vec2
}

func (u unionArcUV) EvalUV(t float64) (geom.Vec2, error) {
	if t == 0 {
		return u.Start, nil
	}
	if t == 1 {
		return u.End, nil
	}
	return u.EllipseUV.EvalUV(t)
}
