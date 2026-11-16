package step

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

func readBoreRimSTEPFaces(f *File, ids []int64) ([]boreRimSTEPFace, error) {
	faces := make([]boreRimSTEPFace, 0, len(ids))
	owned := map[int64]bool{}
	claim := func(id int64) bool {
		if owned[id] {
			return false
		}
		owned[id] = true
		return true
	}
	for _, id := range ids {
		rec, err := simple(f, id, "ADVANCED_FACE", 4)
		if err != nil || !claim(id) {
			return nil, errBoreRimFilletSTEP
		}
		p := boreRimSTEPFace{id: id}
		p.sense, err = boolean(rec.Args[3])
		if err != nil {
			return nil, err
		}
		support, err := ref(rec.Args[2])
		if err != nil || !claim(support) {
			return nil, errBoreRimFilletSTEP
		}
		s := f.Entities[support].Records
		if len(s) != 1 {
			return nil, errBoreRimFilletSTEP
		}
		arity := 2
		switch s[0].Type {
		case "PLANE":
			p.kind = borefillet.Plane
		case "CYLINDRICAL_SURFACE":
			p.kind, arity = borefillet.Cylinder, 3
		case "TOROIDAL_SURFACE":
			p.kind, arity = borefillet.Torus, 4
		default:
			return nil, errBoreRimFilletSTEP
		}
		if len(s[0].Args) != arity {
			return nil, errBoreRimFilletSTEP
		}
		placement, err := ref(s[0].Args[1])
		if err != nil {
			return nil, err
		}
		p.center, p.axis, err = boreRimSTEPFrame(f, placement)
		if err != nil || p.axis.X != 0 || p.axis.Y != 0 || p.axis.Z != 1 && p.axis.Z != -1 {
			return nil, errBoreRimFilletSTEP
		}
		if arity >= 3 {
			p.major, err = cylinderSupportNumber(s[0].Args[2])
			if err != nil || p.major <= 0 {
				return nil, errBoreRimFilletSTEP
			}
		}
		if arity == 4 {
			p.minor, err = cylinderSupportNumber(s[0].Args[3])
			if err != nil || p.minor <= 0 || p.minor >= p.major {
				return nil, errBoreRimFilletSTEP
			}
		}
		bounds, err := listRefs(rec.Args[1], 1)
		if err != nil || len(bounds) > 2 {
			return nil, errBoreRimFilletSTEP
		}
		for _, boundID := range bounds {
			b := f.Entities[boundID].Records
			if len(b) != 1 || len(b[0].Args) != 3 || (b[0].Type != "FACE_BOUND" && b[0].Type != "FACE_OUTER_BOUND") || !claim(boundID) {
				return nil, errBoreRimFilletSTEP
			}
			direction, err := boolean(b[0].Args[2])
			if err != nil {
				return nil, err
			}
			loopID, err := ref(b[0].Args[1])
			if err != nil || !claim(loopID) {
				return nil, errBoreRimFilletSTEP
			}
			loop, err := simple(f, loopID, "EDGE_LOOP", 2)
			if err != nil {
				return nil, err
			}
			uses, err := listRefs(loop.Args[1], 1)
			if err != nil || len(uses) > 10 {
				return nil, errBoreRimFilletSTEP
			}
			wire := make([]analyticUse, len(uses))
			for j, uid := range uses {
				u, err := simple(f, uid, "ORIENTED_EDGE", 5)
				if err != nil || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived || !claim(uid) {
					return nil, errBoreRimFilletSTEP
				}
				e, err := ref(u.Args[3])
				if err != nil {
					return nil, err
				}
				sense, err := boolean(u.Args[4])
				if err != nil {
					return nil, err
				}
				k := j
				if !direction {
					k = len(uses) - 1 - j
				}
				wire[k] = analyticUse{id: e, sense: sense == direction}
			}
			p.loops = append(p.loops, wire)
			p.outerBounds = append(p.outerBounds, b[0].Type == "FACE_OUTER_BOUND")
		}
		faces = append(faces, p)
	}
	return faces, nil
}

type boreRimSTEPIncidence struct {
	graph             borefillet.Graph
	nativeByPoint     map[geom.Vec3]int
	nativeByEnds      map[[2]int]int
	edgeIDs, pointIDs []int64
}

func proveBoreRimSTEPIncidence(ctx context.Context, f *File, faces []boreRimSTEPFace, s trimmed.Solid,
	faceSources []int64, o geom.SurfacePatchOptions) ([]int64, []int64, error) {
	g, err := borefillet.Extract(ctx, s, o)
	if err != nil {
		return nil, nil, err
	}
	p := boreRimSTEPIncidence{
		graph: g, nativeByPoint: make(map[geom.Vec3]int, len(g.Vertices)), nativeByEnds: make(map[[2]int]int, len(g.Edges)),
		edgeIDs: make([]int64, len(g.Edges)), pointIDs: make([]int64, len(g.Vertices)),
	}
	for i, v := range g.Vertices {
		q, err := geom.RepresentedExactPoint(v)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := p.nativeByPoint[q]; exists {
			return nil, nil, errBoreRimFilletSTEP
		}
		p.nativeByPoint[q] = i
	}
	for i, edge := range g.Edges {
		key := boreRimEdgePair(edge.Start, edge.End)
		if _, exists := p.nativeByEnds[key]; exists {
			return nil, nil, errBoreRimFilletSTEP
		}
		p.nativeByEnds[key] = i
	}
	known := map[int64][]analyticUse{}
	directions := map[int64][]bool{}
	for _, f := range faces {
		for _, loop := range f.loops {
			for _, u := range loop {
				directions[u.id] = append(directions[u.id], u.sense)
			}
		}
	}
	for id, senses := range directions {
		if len(senses) != 2 || senses[0] == senses[1] {
			return nil, nil, errBoreRimFilletSTEP
		}
		known[id], err = p.readEdge(f, id)
		if err != nil {
			return nil, nil, err
		}
	}
	for _, id := range p.edgeIDs {
		if id == 0 {
			return nil, nil, errBoreRimFilletSTEP
		}
	}
	byID := make(map[int64]boreRimSTEPFace, len(faces))
	for _, face := range faces {
		byID[face.id] = face
	}
	for i, id := range faceSources {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if face, ok := byID[id]; !ok || !matchBoreRimSTEPWires(face, g.Faces[i], known) {
			return nil, nil, errBoreRimFilletSTEP
		}
	}
	return p.edgeIDs, p.pointIDs, nil
}

func boreRimEdgePair(a, b int) [2]int { return [2]int{min(a, b), max(a, b)} }

func (p *boreRimSTEPIncidence) point(f *File, id int64) (int, geom.Vec3, error) {
	r, err := simple(f, id, "VERTEX_POINT", 2)
	if err != nil {
		return 0, geom.Vec3{}, err
	}
	pid, err := ref(r.Args[1])
	if err != nil {
		return 0, geom.Vec3{}, err
	}
	q, err := mappedExactCoordinate(f, pid, "CARTESIAN_POINT", 3)
	if err != nil {
		return 0, geom.Vec3{}, err
	}
	i, ok := p.nativeByPoint[q]
	if !ok || p.pointIDs[i] != 0 && p.pointIDs[i] != id {
		return 0, geom.Vec3{}, errBoreRimFilletSTEP
	}
	p.pointIDs[i] = id
	return i, q, nil
}

func (p *boreRimSTEPIncidence) readEdge(f *File, id int64) ([]analyticUse, error) {
	r, err := simple(f, id, "EDGE_CURVE", 5)
	if err != nil {
		return nil, err
	}
	t, err := namedReferences(r, 1, 2, 3)
	if err != nil {
		return nil, err
	}
	direction, err := boolean(r.Args[4])
	if err != nil {
		return nil, err
	}
	a, start, err := p.point(f, t[0])
	if err != nil {
		return nil, err
	}
	b, end, err := p.point(f, t[1])
	if err != nil {
		return nil, err
	}
	if a == b {
		return p.fullCircle(f, id, t[2], a, direction)
	}
	i, exists := p.nativeByEnds[boreRimEdgePair(a, b)]
	if !exists || p.edgeIDs[i] != 0 {
		return nil, errBoreRimFilletSTEP
	}
	e := p.graph.Edges[i]
	forward := e.Start == a
	if e.Circle.Radius == nil {
		if !matchBoreRimSTEPLine(f, t[2], start, end, direction) {
			return nil, errBoreRimFilletSTEP
		}
	} else {
		c, axis, r, err := readBoreRimSTEPCircle(f, t[2])
		if err != nil || !matchBoreRimSTEPCircle(e.Circle, c, r) {
			return nil, errBoreRimFilletSTEP
		}
		normal := geom.Vec3{Z: 1}
		if e.Circle.Meridian {
			normal = geom.Vec3{Y: -1}
		}
		if (axis != normal && axis != normal.Scale(-1)) || direction != ((forward == e.Forward) == (axis == normal)) {
			return nil, errBoreRimFilletSTEP
		}
	}
	p.edgeIDs[i] = id
	return []analyticUse{{id: int64(i), sense: forward}}, nil
}

func (p *boreRimSTEPIncidence) fullCircle(f *File, edge, geometry int64, start int, sense bool) ([]analyticUse, error) {
	c, axis, r, err := readBoreRimSTEPCircle(f, geometry)
	if err != nil || axis.X != 0 || axis.Y != 0 || axis.Z != 1 && axis.Z != -1 {
		return nil, errBoreRimFilletSTEP
	}
	counterclockwise := sense == (axis.Z > 0)
	out := make([]analyticUse, 0, 4)
	v := start
	for range 4 {
		next := -1
		for i, e := range p.graph.Edges {
			if e.Circle.Radius == nil || e.Circle.Meridian || !matchBoreRimSTEPCircle(e.Circle, c, r) {
				continue
			}
			a, b := e.Start, e.End
			forward := e.Forward == counterclockwise
			if !forward {
				a, b = b, a
			}
			if a != v {
				continue
			}
			if next >= 0 || p.edgeIDs[i] != 0 {
				return nil, errBoreRimFilletSTEP
			}
			next = b
			out = append(out, analyticUse{id: int64(i), sense: forward})
			p.edgeIDs[i] = edge
		}
		if next < 0 {
			return nil, errBoreRimFilletSTEP
		}
		v = next
	}
	if v != start {
		return nil, errBoreRimFilletSTEP
	}
	return out, nil
}

func readBoreRimSTEPCircle(f *File, id int64) (geom.Vec3, geom.Vec3, float64, error) {
	r, err := simple(f, id, "CIRCLE", 3)
	if err != nil {
		return geom.Vec3{}, geom.Vec3{}, 0, err
	}
	p, err := ref(r.Args[1])
	if err != nil {
		return geom.Vec3{}, geom.Vec3{}, 0, err
	}
	c, n, err := boreRimSTEPFrame(f, p)
	if err != nil {
		return c, n, 0, err
	}
	radius, err := cylinderSupportNumber(r.Args[2])
	return c, n, radius, err
}

func matchBoreRimSTEPCircle(want borefillet.Circle, center geom.Vec3, radius float64) bool {
	c := borefillet.Vec(center.X, center.Y, center.Z)
	return want.Radius.Cmp(sideRat(radius)) == 0 && want.Center[0].Cmp(c[0]) == 0 &&
		want.Center[1].Cmp(c[1]) == 0 && want.Center[2].Cmp(c[2]) == 0
}

func matchBoreRimSTEPLine(f *File, id int64, start, end geom.Vec3, direction bool) bool {
	r, err := simple(f, id, "LINE", 3)
	if err != nil {
		return false
	}
	t, err := namedReferences(r, 1, 2)
	if err != nil {
		return false
	}
	p, err := mappedExactCoordinate(f, t[0], "CARTESIAN_POINT", 3)
	if err != nil || p.X != start.X || p.Y != start.Y {
		return false
	}
	v, err := simple(f, t[1], "VECTOR", 3)
	if err != nil {
		return false
	}
	di, err := ref(v.Args[1])
	if err != nil {
		return false
	}
	d, err := mappedExactCoordinate(f, di, "DIRECTION", 3)
	if err != nil || d.X != 0 || d.Y != 0 || d.Z != 1 && d.Z != -1 {
		return false
	}
	m, err := cylinderSupportNumber(v.Args[2])
	return err == nil && m > 0 && direction == ((end.Z > start.Z) == (d.Z > 0))
}

func matchBoreRimSTEPWires(face boreRimSTEPFace, native borefillet.Face, edges map[int64][]analyticUse) bool {
	want := append([][]trimmed.Use{native.Outer}, native.Holes...)
	if len(want) != len(face.loops) {
		return false
	}
	matched := make([]bool, len(want))
	for k, loop := range face.loops {
		var have []analyticUse
		for _, u := range loop {
			parts := edges[u.id]
			if u.sense {
				have = append(have, parts...)
				continue
			}
			for i := len(parts) - 1; i >= 0; i-- {
				have = append(have, analyticUse{id: parts[i].id, sense: !parts[i].sense})
			}
		}
		for i, wire := range want {
			if !matched[i] && (!face.outerBounds[k] || i == 0) && sameBoreRimSTEPWire(have, wire, native.Reversed) {
				matched[i] = true
				break
			}
		}
	}
	for _, ok := range matched {
		if !ok {
			return false
		}
	}
	return true
}

func sameBoreRimSTEPWire(have []analyticUse, want []trimmed.Use, reverse bool) bool {
	if len(have) != len(want) {
		return false
	}
	for start := range want {
		ok := true
		for i, u := range have {
			k := (start + i) % len(want)
			if reverse {
				k = (start + len(want) - i) % len(want)
			}
			v := want[k]
			if u.id != int64(v.Edge) || u.sense != (v.Reversed == reverse) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
