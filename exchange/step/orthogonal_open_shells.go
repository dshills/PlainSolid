package step

import (
	"errors"
	"io"
	"plainsolid/geom"
)

const MaxOrthogonalShellFaces = 64

// OrthogonalShellUse records source topology, not an inferred geometric weld.
type OrthogonalShellUse struct {
	Face                               int
	EdgeID, StartVertexID, EndVertexID int64
	Forward                            bool
	Loop                               int // 0 is the outer loop; positive values index FaceHoles plus one.
}

// OrthogonalSplineOpenShell is a connected, consistently oriented open
// two-manifold incidence graph whose individual faces and shared curves pass
// exact correspondence checks. Cross-face global embedding is NOT certified.
// It must not be treated as a Solid or a globally intersection-free surface.
type OrthogonalSplineOpenShell struct {
	Name      string
	Faces     []OrthogonalSplineFace
	FaceIDs   []int64
	Uses      []OrthogonalShellUse
	FaceHoles [][][]geom.Vec2 // Indexed by face, then hole; polygons are counterclockwise.
}
type OrthogonalSplineOpenShellModel struct {
	Shells     []OrthogonalSplineOpenShell
	LengthUnit float64
}

func openShellIncidence(s OrthogonalSplineOpenShell) error { return orthogonalShellIncidence(s, false) }

func orthogonalShellIncidence(s OrthogonalSplineOpenShell, closed bool) error {
	fail := func() error { return errors.New("step: inconsistent open-shell incidence") }
	edges := map[int64][]OrthogonalShellUse{}
	vertices := map[int64]map[int]bool{}
	for _, u := range s.Uses {
		if u.Face < 0 || u.Face >= len(s.Faces) || u.StartVertexID == u.EndVertexID {
			return fail()
		}
		if old := edges[u.EdgeID]; len(old) > 0 && (old[0].StartVertexID != u.StartVertexID || old[0].EndVertexID != u.EndVertexID) {
			return fail()
		}
		edges[u.EdgeID] = append(edges[u.EdgeID], u)
		for _, v := range []int64{u.StartVertexID, u.EndVertexID} {
			if vertices[v] == nil {
				vertices[v] = map[int]bool{}
			}
			vertices[v][u.Face] = true
		}
	}
	adjacency := make([]map[int]bool, len(s.Faces))
	for i := range adjacency {
		adjacency[i] = map[int]bool{}
	}
	vertexEdges := map[int64][]int64{}
	for eid, uses := range edges {
		u := uses[0]
		vertexEdges[u.StartVertexID] = append(vertexEdges[u.StartVertexID], eid)
		vertexEdges[u.EndVertexID] = append(vertexEdges[u.EndVertexID], eid)
	}
	boundary := 0
	for _, uses := range edges {
		if len(uses) == 1 {
			boundary++
			continue
		}
		if len(uses) != 2 || uses[0].Face == uses[1].Face || uses[0].Forward == uses[1].Forward {
			return fail()
		}
		a, b := uses[0].Face, uses[1].Face
		adjacency[a][b] = true
		adjacency[b][a] = true
	}
	if (boundary == 0) != closed || len(s.Faces) < 2 {
		return fail()
	}
	connected := func(nodes map[int]bool, links map[int]map[int]bool) bool {
		todo := []int{}
		for k := range nodes {
			todo = append(todo, k)
			break
		}
		seen := map[int]bool{}
		for len(todo) > 0 {
			i := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[i] {
				continue
			}
			seen[i] = true
			for j := range links[i] {
				todo = append(todo, j)
			}
		}
		return len(seen) == len(nodes)
	}
	all := map[int]bool{}
	links := map[int]map[int]bool{}
	for i := range s.Faces {
		all[i] = true
		links[i] = adjacency[i]
	}
	if !connected(all, links) {
		return fail()
	}
	// A vertex link must be one cycle (interior) or one path (boundary).
	for vid, nodes := range vertices {
		links := map[int]map[int]bool{}
		degree := map[int]int{}
		ends := 0
		for n := range nodes {
			links[n] = map[int]bool{}
		}
		for _, eid := range vertexEdges[vid] {
			uses := edges[eid]
			if len(uses) == 1 {
				ends++
				continue
			}
			a, b := uses[0].Face, uses[1].Face
			degree[a]++
			degree[b]++
			links[a][b] = true
			links[b][a] = true
		}
		if ends != 0 && ends != 2 || !connected(nodes, links) {
			return fail()
		}
		for n := range nodes {
			if degree[n] > 2 {
				return fail()
			}
			if ends == 0 && degree[n] != 2 {
				return fail()
			}
		}
	}
	return nil
}

// ReadOrthogonalSplineOpenShells accepts connected OPEN_SHELLs with at least
// two orthogonal spline faces, optionally with strictly interior holes. Shared source EDGE_CURVEs must have
// exactly the two owning p-curves and opposite uses. Boundary edges have one.
// Global face-face nonintersection is deliberately not asserted.
func ReadOrthogonalSplineOpenShells(r io.Reader, l Limits) (OrthogonalSplineOpenShellModel, error) {
	l, e := l.defaults()
	if e != nil {
		return OrthogonalSplineOpenShellModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return OrthogonalSplineOpenShellModel{}, e
	}
	return readOrthogonalSplineShellFile(f, l, false)
}

func readOrthogonalSplineShellFile(f *File, l Limits, closed bool) (OrthogonalSplineOpenShellModel, error) {
	fail := func() (OrthogonalSplineOpenShellModel, error) {
		return OrthogonalSplineOpenShellModel{}, errors.New("step: unsupported connected orthogonal shell")
	}
	// Split only the graph view consumed by the strict standalone reader. Keep
	// original shell membership and IDs separately; no source geometry is welded.
	view := *f
	view.Entities = make(map[int64]Entity, len(f.Entities))
	for id, ent := range f.Entities {
		view.Entities[id] = ent
	}
	view.Order = append([]int64(nil), f.Order...)
	out := OrthogonalSplineOpenShellModel{}
	membership := map[int64][2]int{}
	split := map[int64][]int64{}
	next := int64(-1)
	for _, id := range f.Order {
		shell, e := simple(f, id, "OPEN_SHELL", 2)
		if e != nil {
			continue
		}
		ids, e := listRefs(shell.Args[1], 2)
		if e != nil || len(ids) < 2 || len(ids) > l.MaxFaces || len(membership)+len(ids) > MaxOrthogonalShellFaces || len(out.Shells) >= l.MaxSolids {
			return fail()
		}
		name, e := textValue(shell.Args[0])
		if e != nil {
			return fail()
		}
		si := len(out.Shells)
		out.Shells = append(out.Shells, OrthogonalSplineOpenShell{Name: name, FaceIDs: ids, Faces: make([]OrthogonalSplineFace, len(ids)), FaceHoles: make([][][]geom.Vec2, len(ids))})
		for i, fid := range ids {
			if _, ok := membership[fid]; ok {
				return fail()
			}
			membership[fid] = [2]int{si, i}
			sid := id
			if i > 0 {
				for {
					if _, ok := view.Entities[next]; !ok {
						break
					}
					next--
				}
				sid = next
				next--
				view.Order = append(view.Order, sid)
			}
			rec := shell
			rec.Args = append([]Value(nil), shell.Args...)
			rec.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: fid}}}
			view.Entities[sid] = Entity{ID: sid, Records: []Record{rec}}
			split[id] = append(split[id], sid)
		}
	}
	if len(out.Shells) == 0 {
		return fail()
	}
	for id, ent := range view.Entities {
		if len(ent.Records) != 1 || ent.Records[0].Type != "SHELL_BASED_SURFACE_MODEL" {
			continue
		}
		rec := ent.Records[0]
		if len(rec.Args) != 2 {
			return fail()
		}
		ids, e := listRefs(rec.Args[1], 1)
		if e != nil {
			return fail()
		}
		items := []Value{}
		for _, sid := range ids {
			if len(split[sid]) == 0 {
				return fail()
			}
			for _, newID := range split[sid] {
				items = append(items, Value{Kind: Reference, Ref: newID})
			}
		}
		rec.Args = append([]Value(nil), rec.Args...)
		rec.Args[1] = Value{Kind: Aggregate, Items: items}
		view.Entities[id] = Entity{ID: id, Records: []Record{rec}}
	}
	remaining := l.MaxEdgeUses
	supports := map[int64]int64{}
	edgeShell := map[int64]int{}
	vertexPoints := map[int64]geom.Vec3{}
	pointIDs := map[geom.Vec3]int64{}
	m, e := readSplineFaceGraph(&view, l, func(local *File, fid int64, maxPoints int) (BoundedSplineFace, error) {
		bad := func() (BoundedSplineFace, error) {
			return BoundedSplineFace{}, errors.New("step: invalid exact open-shell face")
		}
		where, ok := membership[fid]
		if !ok {
			return bad()
		}
		decoded, holes, uses, e := decodeOrthogonalShellFace(local, fid, maxPoints, remaining)
		if e != nil {
			return bad()
		}
		face, _ := simple(local, fid, "ADVANCED_FACE", 4)
		support, _ := ref(face.Args[2])
		supports[fid] = support
		for _, use := range uses {
			if old, ok := edgeShell[use.EdgeID]; ok && old != where[0] {
				return bad()
			}
			edgeShell[use.EdgeID] = where[0]
			for _, vid := range []int64{use.StartVertexID, use.EndVertexID} {
				p, e := analyticVertex(local, vid)
				if e != nil {
					return bad()
				}
				if old, ok := pointIDs[p]; ok && old != vid {
					return bad()
				}
				pointIDs[p] = vid
				vertexPoints[vid] = p
			}
			use.Face = where[1]
			out.Shells[where[0]].Uses = append(out.Shells[where[0]].Uses, use)
		}
		out.Shells[where[0]].Faces[where[1]] = decoded
		out.Shells[where[0]].FaceHoles[where[1]] = holes
		remaining -= len(uses)
		d, _ := orthogonalPolygon(decoded.Boundary)
		return BoundedSplineFace{Name: decoded.Name, Geometry: decoded.Geometry, Domain: d}, nil
	}, 0)
	if e != nil {
		return OrthogonalSplineOpenShellModel{}, e
	}
	if len(vertexPoints) > l.MaxVertices {
		return fail()
	}
	for _, shell := range out.Shells {
		if e := orthogonalShellIncidence(shell, closed); e != nil {
			return fail()
		}
		owners := map[int64][]int64{}
		for _, u := range shell.Uses {
			owners[u.EdgeID] = append(owners[u.EdgeID], supports[shell.FaceIDs[u.Face]])
		}
		for eid, expected := range owners {
			_, _, g, _ := analyticEdge(f, eid)
			sc, _ := simple(f, g, "SURFACE_CURVE", 4)
			pcs, _ := listRefs(sc.Args[2], 1)
			if len(pcs) != len(expected) {
				return fail()
			}
			for _, pcid := range pcs {
				pc, _ := simple(f, pcid, "PCURVE", 3)
				owner, _ := ref(pc.Args[1])
				found := false
				for _, want := range expected {
					found = found || owner == want
				}
				if !found {
					return fail()
				}
			}
		}
	}
	out.LengthUnit = m.LengthUnit
	return out, nil
}
