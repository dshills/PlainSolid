package trimmed

import (
	"context"
	"fmt"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AttachSteppedShaftHead replaces only the untouched top disk of an external
// grooved shaft. The native head, socket and their boundary edges are retained.
// Four analytic annular strips join an arbitrary helical seam phase to the
// cardinal head rim; no chord approximation or coincident duplicate cap remains.
func AttachSteppedShaftHead(ctx context.Context, p SteppedShaft, groove Solid) (Solid, error) {
	fail := func() (Solid, error) {
		return Solid{}, fmt.Errorf("trimmed: grooved shaft has no qualified untouched head interface")
	}
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	head, e := p.Solid(ctx)
	if e != nil {
		return Solid{}, e
	}
	topEdge, topFace := -1, -1
	phase := 0.
	wantOrigin := p.Frame.ToWorld(geom.Vec3{Z: -p.ShaftLength})
	for _, f := range groove.faces {
		wall, ok := f.Surface.(geom.TwistedCylinderSurface)
		if !ok {
			continue
		}
		frame, r, tw, ph := wall.Parameters()
		if r != p.ShaftRadius {
			continue
		}
		if frame.Origin() != wantOrigin || frame.XAxis() != p.Frame.XAxis() || frame.ZAxis() != p.Frame.ZAxis() {
			return fail()
		}
		for _, u := range f.Outer {
			if u.UVStart.Y == p.ShaftLength && u.UVEnd.Y == p.ShaftLength && math.Abs(u.UVEnd.X-u.UVStart.X) == 1 {
				if topEdge >= 0 {
					return fail()
				}
				topEdge = u.Edge
				phase = math.Remainder(tw*p.ShaftLength+ph, 1)
			}
		}
	}
	if topEdge < 0 {
		return fail()
	}
	edge := groove.edges[topEdge]
	if edge.Start != edge.End || edge.Range.Min != 0 || edge.Range.Max != 1 {
		return fail()
	}
	for i, f := range groove.faces {
		frame, ok := parametric.PolarPlaneFrame(f.Surface)
		if !ok {
			continue
		}
		if frame.Origin() != p.Frame.Origin() || frame.ZAxis() != p.Frame.ZAxis() {
			continue
		}
		found := false
		for _, u := range f.Outer {
			if u.Edge == topEdge {
				found = true
			}
		}
		if found {
			if topFace >= 0 || len(f.Holes) != 0 {
				return fail()
			}
			topFace = i
		}
	}
	if topFace < 0 {
		return fail()
	}
	// Split the same authoritative periodic rim; all its existing face uses move
	// together. The head attachment never changes the helical wall or its holes.
	current := topEdge
	innerEdges := [4]int{}
	for i := 0; i < 3; i++ {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		var h EdgeSplit
		groove, h, e = groove.splitEdgeDraft(current, float64(i+1)/4)
		if e != nil {
			return Solid{}, e
		}
		innerEdges[i] = h.First
		current = h.Second
	}
	innerEdges[3] = current
	vertices, edges, faces := groove.Vertices(), groove.Edges(), groove.Faces()
	faces = append(faces[:topFace], faces[topFace+1:]...)
	hv, he, hf := head.Vertices(), head.Edges(), head.Faces()
	vo, eo := len(vertices), len(edges)
	vertices = append(vertices, hv...)
	for _, edge := range he {
		edge.Start += vo
		edge.End += vo
		edges = append(edges, edge)
	}
	outerEdges := [4]int{}
	outerVertices := [4]int{}
	cardinal := [4]geom.Vec3{{X: p.HeadRadius}, {Y: p.HeadRadius}, {X: -p.HeadRadius}, {Y: -p.HeadRadius}}
	for i, point := range cardinal {
		world := p.Frame.ToWorld(point)
		found := -1
		for j, v := range hv {
			if v == world {
				if found >= 0 {
					return fail()
				}
				found = j + vo
			}
		}
		if found < 0 {
			return fail()
		}
		outerVertices[i] = found
	}
	for i := 0; i < 4; i++ {
		found := -1
		a, b := outerVertices[i], outerVertices[(i+1)%4]
		for j, edge := range edges[eo:] {
			if edge.Start == a && edge.End == b || edge.Start == b && edge.End == a {
				if found >= 0 {
					return fail()
				}
				found = j + eo
			}
		}
		if found < 0 {
			return fail()
		}
		outerEdges[i] = found
	}
	for i, f := range hf {
		if i == 0 || i == 1 {
			continue
		}
		if cyl, ok := f.Surface.(geom.CylinderSurface); ok {
			_, r := cyl.Parameters()
			if r == p.ShaftRadius {
				continue
			}
		}
		for j := range f.Outer {
			f.Outer[j].Edge += eo
		}
		for j := range f.Holes {
			for k := range f.Holes[j] {
				f.Holes[j][k].Edge += eo
			}
		}
		faces = append(faces, f)
	}
	shoulder, e := geom.NewTwistedAnnularSurface(p.Frame, p.ShaftRadius, p.HeadRadius, 0, phase)
	if e != nil {
		return Solid{}, e
	}
	radial := [4]int{}
	for i := 0; i < 4; i++ {
		u := float64(i) / 4
		innerVertex := edges[innerEdges[i]].Start
		radial[i] = len(edges)
		edges = append(edges, parametric.Edge{Start: innerVertex, End: outerVertices[i], Curve: surfaceTrimCurve{shoulder, geom.Vec2{X: u, Y: p.ShaftRadius}, geom.Vec2{X: u, Y: p.HeadRadius}, nil}, Range: parametric.Interval{Max: 1}})
	}
	for i := 0; i < 4; i++ {
		j := (i + 1) % 4
		u, v := float64(i)/4, float64(i+1)/4
		uv := [4]geom.Vec2{{X: u, Y: p.ShaftRadius}, {X: v, Y: p.ShaftRadius}, {X: v, Y: p.HeadRadius}, {X: u, Y: p.HeadRadius}}
		ids := [4]int{innerEdges[i], radial[j], outerEdges[i], radial[i]}
		rev := [4]bool{false, false, edges[outerEdges[i]].Start == outerVertices[i], true}
		f := Face{Surface: shoulder}
		for k, id := range ids {
			f.Outer = append(f.Outer, Use{Edge: id, Reversed: rev[k], UVStart: uv[k], UVEnd: uv[(k+1)%4]})
		}
		faces = append(faces, f)
	}
	return compactSteppedTopology(ctx, vertices, edges, faces, max(head.tolerance, groove.tolerance))
}
func compactSteppedTopology(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tol float64) (Solid, error) {
	return compactSteppedTopologyWithValidation(ctx, vertices, edges, faces, tol, true)
}

func compactSteppedTopologyWithValidation(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tol float64, validateDisplay bool) (Solid, error) {
	edgeMap := map[int]int{}
	vertexMap := map[int]int{}
	var outV []geom.Vec3
	var outE []parametric.Edge
	vertex := func(id int) int {
		if v, ok := vertexMap[id]; ok {
			return v
		}
		v := len(outV)
		vertexMap[id] = v
		outV = append(outV, vertices[id])
		return v
	}
	for i := range faces {
		loops := append([][]Use{faces[i].Outer}, faces[i].Holes...)
		for _, loop := range loops {
			for j := range loop {
				id := loop[j].Edge
				next, ok := edgeMap[id]
				if !ok {
					next = len(outE)
					edgeMap[id] = next
					edge := edges[id]
					edge.Start = vertex(edge.Start)
					edge.End = vertex(edge.End)
					outE = append(outE, edge)
				}
				loop[j].Edge = next
			}
		}
	}
	return newSolidContext(ctx, outV, outE, faces, tol, validateDisplay)
}
