package plaincadbridge

import (
	"context"
	"math"
	"slices"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// treatOrthogonalAxialEdges reconstructs a plain orthogonal extrusion with
// the selected convex and concave corner contours replaced as one operation.
func (b *Bridge) treatOrthogonalAxialEdges(stock brep.Solid, q request, edges [][2]geom.Vec3, size float64) (Shape, error) {
	fail := func() (Shape, error) {
		return Shape{}, unsupportedEdge("axial treatment requires a single orthogonal prism and disjoint corner clearances")
	}
	if q.Direction == nil {
		return fail()
	}
	axis := q.Direction.vec()
	frame, err := geom.FrameFromNormal(edges[0][0], axis)
	if err != nil || !cardinalFrame(frame) {
		return fail()
	}
	axis = frame.ZAxis()
	vertices := stock.Vertices()
	lo, hi := 0., 0.
	for _, v := range vertices {
		z := frame.ToLocal(v).Z
		lo, hi = min(lo, z), max(hi, z)
	}
	height := hi - lo
	if height <= 0 {
		return fail()
	}
	frame, err = geom.NewFrame(frame.ToWorld(geom.Vec3{Z: lo}), frame.XAxis(), axis)
	if err != nil {
		return fail()
	}
	cap, err := matchingPlanarFace(stock, frame.Origin(), axis)
	if err != nil {
		return fail()
	}
	faces, nativeEdges := stock.Faces(), stock.Edges()
	wire := faces[cap.Face].Boundary
	if len(wire) < 4 || len(wire) > MaxProfileVertices || len(faces) != len(wire)+2 || len(vertices) != 2*len(wire) || len(faces[cap.Face].Holes) != 0 {
		return fail()
	}
	profile := make([]geom.Vec2, 0, len(wire))
	upper := make(map[geom.Vec3]bool, len(wire))
	for _, v := range vertices {
		if p := frame.ToLocal(v); p.Z == height {
			upper[v] = true
		}
	}
	for _, u := range wire {
		index := nativeEdges[u.Edge].Start
		if u.Reversed {
			index = nativeEdges[u.Edge].End
		}
		v := vertices[index]
		p := frame.ToLocal(v)
		if p.Z != 0 || frame.ToWorld(p) != v || !upper[v.Add(axis.Scale(height))] {
			return fail()
		}
		profile = append(profile, geom.Vec2{X: p.X, Y: p.Y})
	}
	if cap.Normal.vec().Dot(axis) < 0 {
		slices.Reverse(profile)
	}
	for _, face := range faces {
		if len(face.Holes) != 0 {
			return fail()
		}
		if len(face.Boundary) != 4 && len(face.Boundary) != len(wire) {
			return fail()
		}
	}
	reference := make([]geom.Vec3, len(profile))
	for i, p := range profile {
		reference[i] = frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y})
	}
	prism, err := brep.Prism(reference, axis.Scale(height))
	if err != nil || !samePlanarFaces(stock, prism) {
		return fail()
	}
	selected := make(map[geom.Vec2]bool, len(edges))
	for _, edge := range edges {
		a, c := frame.ToLocal(edge[0]), frame.ToLocal(edge[1])
		if a.X != c.X || a.Y != c.Y || min(a.Z, c.Z) != 0 || max(a.Z, c.Z) != height {
			return fail()
		}
		selected[geom.Vec2{X: a.X, Y: a.Y}] = true
	}
	segments, err := orthogonalAxialProfile(profile, selected, size, q.Op == "fillet")
	if err != nil {
		return fail()
	}
	if q.Op == "chamfer" {
		corners := make([]geom.Vec3, len(segments))
		for i, s := range segments {
			corners[i] = frame.ToWorld(geom.Vec3{X: s.Start.X, Y: s.Start.Y})
		}
		solid, err := brep.Prism(corners, axis.Scale(height))
		if err != nil {
			return Shape{}, err
		}
		return b.put(solid)
	}
	solid, err := trimmed.ExtrudeArcLoop(context.Background(), frame, segments, height, max(1, height, size)*1e-10)
	if err != nil {
		return Shape{}, err
	}
	return b.putArcLoop(solid, frame, segments, height)
}

func orthogonalAxialProfile(profile []geom.Vec2, selected map[geom.Vec2]bool, size float64, rounded bool) ([]trimmed.ExtrusionSegment, error) {
	n := len(profile)
	before := append([]geom.Vec2(nil), profile...)
	after := append([]geom.Vec2(nil), profile...)
	blends := make([]trimmed.ExtrusionSegment, n)
	clearance := make([]geom.Box, n)
	used := 0
	for i, p := range profile {
		prev, next := profile[(i+n-1)%n], profile[(i+1)%n]
		a, c := geom.Vec2{X: prev.X - p.X, Y: prev.Y - p.Y}, geom.Vec2{X: next.X - p.X, Y: next.Y - p.Y}
		if (a.X == 0) == (a.Y == 0) || (c.X == 0) == (c.Y == 0) || (a.X == 0) == (c.X == 0) {
			return nil, geom.ErrDomain
		}
		if !selected[p] {
			continue
		}
		used++
		la, lc := math.Abs(a.X+a.Y), math.Abs(c.X+c.Y)
		if min(la, lc) <= size {
			return nil, geom.ErrDomain
		}
		ax, ay, cx, cy := a.X/la, a.Y/la, c.X/lc, c.Y/lc
		before[i] = geom.Vec2{X: p.X + size*ax, Y: p.Y + size*ay}
		after[i] = geom.Vec2{X: p.X + size*cx, Y: p.Y + size*cy}
		center := geom.Vec2{X: p.X + size*(ax+cx), Y: p.Y + size*(ay+cy)}
		clearance[i] = axialCornerBox(p, center)
		blends[i] = trimmed.ExtrusionSegment{Start: before[i], End: after[i]}
		if rounded {
			blends[i].Center = center
			blends[i].Radius = size
			blends[i].StartAngle = math.Atan2(-cy, -cx)
			blends[i].Sweep = math.Copysign(math.Pi/2, ay*cx-ax*cy)
		}
	}
	if used == 0 || used != len(selected) {
		return nil, geom.ErrDomain
	}
	segments := make([]trimmed.ExtrusionSegment, 0, n+used)
	for i, p := range profile {
		j := (i + 1) % n
		if selected[p] {
			for k, a := range profile {
				m := (k + 1) % n
				if k != i && m != i {
					if _, overlaps := clearance[i].Intersect(axialCornerBox(a, profile[m])); overlaps {
						return nil, geom.ErrDomain
					}
				}
				if k > i && selected[a] {
					if _, overlaps := clearance[i].Intersect(clearance[k]); overlaps {
						return nil, geom.ErrDomain
					}
				}
			}
			segments = append(segments, blends[i])
		}
		edge := profile[j]
		a, c := after[i], before[j]
		if (edge.X-p.X)*(c.X-a.X)+(edge.Y-p.Y)*(c.Y-a.Y) <= 0 {
			return nil, geom.ErrDomain
		}
		segments = append(segments, trimmed.ExtrusionSegment{Start: a, End: c})
	}
	return segments, nil
}

func axialCornerBox(a, b geom.Vec2) geom.Box {
	return geom.Box{
		Min: geom.Vec3{X: min(a.X, b.X), Y: min(a.Y, b.Y)},
		Max: geom.Vec3{X: max(a.X, b.X), Y: max(a.Y, b.Y)},
	}
}

func samePlanarFaces(a, b brep.Solid) bool {
	loops := func(s brep.Solid) [][]geom.Vec3 {
		v, e := s.Vertices(), s.Edges()
		f := s.Faces()
		out := make([][]geom.Vec3, len(f))
		for i, face := range f {
			for _, u := range face.Boundary {
				index := e[u.Edge].Start
				if u.Reversed {
					index = e[u.Edge].End
				}
				out[i] = append(out[i], v[index])
			}
		}
		return out
	}
	want, have := loops(a), loops(b)
	if len(want) != len(have) {
		return false
	}
	for _, loop := range want {
		matches := 0
		for _, candidate := range have {
			if len(loop) != len(candidate) {
				continue
			}
			j := slices.Index(candidate, loop[0])
			if j < 0 {
				continue
			}
			equal := true
			for i, p := range loop {
				if candidate[(j+i)%len(candidate)] != p {
					equal = false
					break
				}
			}
			if equal {
				matches++
			}
		}
		if matches != 1 {
			return false
		}
	}
	return true
}
