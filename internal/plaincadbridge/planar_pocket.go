package plaincadbridge

import "plainsolid/brep"

// openPlanarPocket sews an entry-open, strictly side-contained planar tool
// directly into the selected cap. This preserves represented coordinates
// for slightly rotated pocket profiles without splitting planar triangles.
func openPlanarPocket(stock, tool brep.Solid) (stored, bool) {
	a, aErr := stock.Bounds()
	b, bErr := tool.Bounds()
	if aErr != nil || bErr != nil {
		return stored{}, false
	}
	lo, hi := coordinates(a.Min), coordinates(a.Max)
	cutLo, cutHi := coordinates(b.Min), coordinates(b.Max)
	axis, level, direction := -1, 0.0, 0.0
	for i := range lo {
		if cutLo[i] < lo[i] || cutHi[i] > hi[i] || cutLo[i] == lo[i] && cutHi[i] == hi[i] {
			return stored{}, false
		}
		if cutLo[i] == lo[i] || cutHi[i] == hi[i] {
			if axis >= 0 {
				return stored{}, false
			}
			axis, level, direction = i, lo[i], -1
			if cutHi[i] == hi[i] {
				level, direction = hi[i], 1
			}
		}
	}
	if axis < 0 {
		return stored{}, false
	}
	n := vector{}
	n[axis] = direction
	findCap := func(s brep.Solid, faces []brep.Face) int {
		id := -1
		for i := range faces {
			plane, err := s.FacePlane(i)
			if err != nil || plane.Normal() != n.vec() || coordinates(plane.Origin())[axis] != level {
				continue
			}
			if id >= 0 {
				return -1
			}
			id = i
		}
		return id
	}
	faces, toolFaces := stock.Faces(), tool.Faces()
	cap, entry := findCap(stock, faces), findCap(tool, toolFaces)
	if cap < 0 || entry < 0 || len(toolFaces[entry].Holes) != 0 {
		return stored{}, false
	}
	vertices, edges := stock.Vertices(), stock.Edges()
	vertexOffset, edgeOffset := len(vertices), len(edges)
	vertices = append(vertices, tool.Vertices()...)
	for _, edge := range tool.Edges() {
		edges = append(edges, brep.Edge{Start: edge.Start + vertexOffset, End: edge.End + vertexOffset})
	}
	invert := func(wire []brep.EdgeUse) []brep.EdgeUse {
		out := make([]brep.EdgeUse, len(wire))
		for i, use := range wire {
			out[len(wire)-1-i] = brep.EdgeUse{Edge: use.Edge + edgeOffset, Reversed: !use.Reversed}
		}
		return out
	}
	faces[cap].Holes = append(faces[cap].Holes, invert(toolFaces[entry].Boundary))
	for i, face := range toolFaces {
		if i != entry {
			for j, hole := range face.Holes {
				face.Holes[j] = invert(hole)
			}
			face.Boundary = invert(face.Boundary)
			faces = append(faces, face)
		}
	}
	solid, err := brep.NewSolid(vertices, edges, faces)
	if err != nil {
		return stored{}, false
	}
	result, err := provePlanar(solid)
	before, stockErr := stock.MassProperties()
	removed, toolErr := tool.MassProperties()
	tolerance := 1e-10 * before.Volume
	delta := before.Volume - result.GeometryAssertions.Volume - removed.Volume
	if err != nil || stockErr != nil || toolErr != nil || result.GeometryAssertions.SolidCount != 1 ||
		delta < -tolerance || delta > tolerance {
		return stored{}, false
	}
	return stored{solid: solid, result: result}, true
}
