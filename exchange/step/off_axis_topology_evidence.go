package step

import (
	"bytes"
	"fmt"
	"io"
)

// OffAxisTopologyEvidence certifies the selected three-face source incidence
// graph, not geometric embedding, cap coverage or material validity. Face/bound
// source order and normalized coedge direction are preserved explicitly.
type OffAxisTopologyEvidence struct {
	BodyID, ShellID                      int64
	Faces                                []OffAxisTopologyFace
	Vertices, Edges, EulerCharacteristic int
	Intersection                         CylinderPCurveEvidence
	EdgeCurves                           []OffAxisTopologyEdge
}

// OffAxisTopologyEdge records both owner pcurves, including the two branches
// of a same-face seam. It does not assign a seam branch to a particular use.
type OffAxisTopologyEdge struct {
	EdgeID, SurfaceCurveID, ModelCurveID     int64
	Seam                                     bool
	PCurveIDs, SupportIDs, ParameterCurveIDs [2]int64
}
type OffAxisTopologyFace struct {
	FaceID, SurfaceID int64
	SurfaceKind       string
	SameSense         bool
	Bounds            []OffAxisTopologyBound
}
type OffAxisTopologyBound struct {
	BoundID, LoopID int64
	Orientation     bool
	Uses            []OffAxisTopologyUse
}
type OffAxisTopologyUse struct {
	CoedgeID, EdgeID, StartVertex, EndVertex int64
	Forward                                  bool
}

// ValidateOffAxisBoreTopology admits one MANIFOLD_SOLID_BREP with one closed
// shell of sphere, cylinder and planar faces, one loop each, four edges and
// three vertices. Two edges are same-face seams; the others join the spherical
// rim and floor rim. Ordered vertex closure, exclusive ownership, opposite uses,
// connected vertex links and Euler characteristic 2 are required. The spline
// rim maps to its verified two-owner pcurves; no spatial coverage is inferred.
func ValidateOffAxisBoreTopology(r io.Reader, l Limits, bodyID int64) (out OffAxisTopologyEvidence, err error) {
	fail := func() (OffAxisTopologyEvidence, error) {
		return OffAxisTopologyEvidence{}, fmt.Errorf("step: unsupported or invalid off-axis topology")
	}
	l, e := l.defaults()
	if e != nil || r == nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	body, e := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	shellID, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faceIDs, e := listRefs(shell.Args[1], 3)
	if e != nil || len(faceIDs) != 3 || l.MaxFaces < 3 || l.MaxVertices < 3 {
		return fail()
	}
	type edgeInfo struct {
		a, b  int64
		uses  []bool
		faces []int64
	}
	edges := map[int64]*edgeInfo{}
	vertices := map[int64]bool{}
	owned := map[int64]bool{bodyID: true, shellID: true}
	kinds := map[string]int64{}
	faceKinds := map[int64]string{}
	type endpoint struct {
		edge int64
		end  bool
	}
	links := map[int64]map[endpoint][]endpoint{}
	mark := func(id int64) bool {
		if owned[id] {
			return false
		}
		owned[id] = true
		return true
	}
	out.BodyID, out.ShellID = bodyID, shellID
	totalUses := 0
	for _, fid := range faceIDs {
		if !mark(fid) {
			return fail()
		}
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		support, ok := f.Entities[sid]
		if !ok || len(support.Records) != 1 {
			return fail()
		}
		kind := support.Records[0].Type
		if kind != "SPHERICAL_SURFACE" && kind != "CYLINDRICAL_SURFACE" && kind != "PLANE" || kinds[kind] != 0 {
			return fail()
		}
		arity := 3
		if kind == "PLANE" {
			arity = 2
		}
		primitive, e := simple(f, sid, kind, arity)
		if e != nil {
			return fail()
		}
		axisID, e := ref(primitive.Args[1])
		if e != nil {
			return fail()
		}
		if _, e = placement(f, axisID); e != nil {
			return fail()
		}
		if arity == 3 {
			radius, e := numeric(primitive.Args[2])
			if e != nil || radius <= 0 {
				return fail()
			}
		}
		kinds[kind] = fid
		faceKinds[fid] = kind
		same, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		if (kind == "CYLINDRICAL_SURFACE") == same {
			return fail()
		}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil || len(bounds) != 1 {
			return fail()
		}
		of := OffAxisTopologyFace{FaceID: fid, SurfaceID: sid, SurfaceKind: kind, SameSense: same}
		for _, bid := range bounds {
			if !mark(bid) {
				return fail()
			}
			bound, e := simple(f, bid, "FACE_BOUND", 3)
			if e != nil {
				bound, e = simple(f, bid, "FACE_OUTER_BOUND", 3)
			}
			if e != nil {
				return fail()
			}
			sense, e := boolean(bound.Args[2])
			if e != nil {
				return fail()
			}
			lid, e := ref(bound.Args[1])
			if e != nil || !mark(lid) {
				return fail()
			}
			loop, e := simple(f, lid, "EDGE_LOOP", 2)
			if e != nil {
				return fail()
			}
			ids, e := listRefs(loop.Args[1], 1)
			if e != nil {
				return fail()
			}
			totalUses += len(ids)
			if totalUses > l.MaxEdgeUses {
				return fail()
			}
			ob := OffAxisTopologyBound{BoundID: bid, LoopID: lid, Orientation: sense}
			for _, oid := range ids {
				if !mark(oid) {
					return fail()
				}
				oe, e := simple(f, oid, "ORIENTED_EDGE", 5)
				if e != nil || oe.Args[1].Kind != Derived || oe.Args[2].Kind != Derived {
					return fail()
				}
				eid, e := ref(oe.Args[3])
				if e != nil {
					return fail()
				}
				forward, e := boolean(oe.Args[4])
				if e != nil {
					return fail()
				}
				info := edges[eid]
				if info == nil {
					rec, e := simple(f, eid, "EDGE_CURVE", 5)
					if e != nil {
						return fail()
					}
					if forward, e := boolean(rec.Args[4]); e != nil || !forward {
						return fail()
					}
					a, e := ref(rec.Args[1])
					if e != nil {
						return fail()
					}
					b, e := ref(rec.Args[2])
					if e != nil {
						return fail()
					}
					for _, v := range []int64{a, b} {
						if _, e := analyticVertex(f, v); e != nil {
							return fail()
						}
						vertices[v] = true
					}
					info = &edgeInfo{a: a, b: b}
					edges[eid] = info
					owned[eid] = true
				}
				a, b := info.a, info.b
				if !forward {
					a, b = b, a
				}
				ob.Uses = append(ob.Uses, OffAxisTopologyUse{oid, eid, a, b, forward})
			}
			// Normalize FACE_BOUND direction while retaining source coedge IDs.
			if !sense {
				for i, j := 0, len(ob.Uses)-1; i < j; i, j = i+1, j-1 {
					ob.Uses[i], ob.Uses[j] = ob.Uses[j], ob.Uses[i]
				}
				for i := range ob.Uses {
					u := &ob.Uses[i]
					u.Forward = !u.Forward
					u.StartVertex, u.EndVertex = u.EndVertex, u.StartVertex
				}
			}
			for i, u := range ob.Uses {
				next := ob.Uses[(i+1)%len(ob.Uses)]
				if u.EndVertex != next.StartVertex {
					return fail()
				}
				info := edges[u.EdgeID]
				info.uses = append(info.uses, u.Forward)
				info.faces = append(info.faces, fid)
				at := u.EndVertex
				if links[at] == nil {
					links[at] = map[endpoint][]endpoint{}
				}
				a, b := endpoint{u.EdgeID, u.Forward}, endpoint{next.EdgeID, !next.Forward}
				links[at][a] = append(links[at][a], b)
				links[at][b] = append(links[at][b], a)
			}
			of.Bounds = append(of.Bounds, ob)
		}
		out.Faces = append(out.Faces, of)
	}
	if len(edges) != 4 || len(vertices) != 3 {
		return fail()
	}
	curveEntity := func(id int64) bool {
		en, ok := f.Entities[id]
		if !ok || len(en.Records) != 1 {
			return false
		}
		r := en.Records[0]
		switch r.Type {
		case "B_SPLINE_CURVE_WITH_KNOTS":
			return len(r.Args) == 9
		case "LINE", "CIRCLE":
			return len(r.Args) == 3
		}
		return false
	}
	pcurveOwners := map[int64]int64{}
	for _, id := range f.Order {
		ed := edges[id]
		if ed == nil {
			continue
		}
		if len(ed.faces) != 2 {
			return fail()
		}
		rec, e := simple(f, id, "EDGE_CURVE", 5)
		if e != nil {
			return fail()
		}
		cid, e := ref(rec.Args[3])
		if e != nil {
			return fail()
		}
		kind := "SURFACE_CURVE"
		seam := ed.faces[0] == ed.faces[1]
		if seam {
			kind = "SEAM_CURVE"
		}
		curve, e := simple(f, cid, kind, 4)
		if e != nil {
			return fail()
		}
		model, e := ref(curve.Args[1])
		if e != nil {
			return fail()
		}
		if !curveEntity(model) {
			return fail()
		}
		pcs, e := listRefs(curve.Args[2], 2)
		if e != nil || len(pcs) != 2 || pcs[0] == pcs[1] {
			return fail()
		}
		if curve.Args[3].Kind != Enumeration || curve.Args[3].Text != "CURVE_3D" && curve.Args[3].Text != "PCURVE_S1" && curve.Args[3].Text != "PCURVE_S2" {
			return fail()
		}
		descriptor := OffAxisTopologyEdge{EdgeID: id, SurfaceCurveID: cid, ModelCurveID: model, Seam: seam}
		expected := map[int64]int{}
		for _, fid := range ed.faces {
			face, _ := simple(f, fid, "ADVANCED_FACE", 4)
			sid, _ := ref(face.Args[2])
			expected[sid]++
		}
		for i, pid := range pcs {
			if previous := pcurveOwners[pid]; previous != 0 {
				return fail()
			}
			pcurveOwners[pid] = id
			pc, e := simple(f, pid, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			sid, e := ref(pc.Args[1])
			if e != nil || expected[sid] == 0 {
				return fail()
			}
			expected[sid]--
			rid, e := ref(pc.Args[2])
			if e != nil {
				return fail()
			}
			rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
			items, e := listRefs(rep.Args[1], 1)
			if e != nil || len(items) != 1 {
				return fail()
			}
			if !curveEntity(items[0]) {
				return fail()
			}
			contextID, e := ref(rep.Args[2])
			if e != nil {
				return fail()
			}
			context, ok := f.Entities[contextID]
			if !ok {
				return fail()
			}
			dimension, ok := component(context, "GEOMETRIC_REPRESENTATION_CONTEXT")
			if !ok || len(dimension.Args) != 1 {
				return fail()
			}
			n, e := numeric(dimension.Args[0])
			if e != nil || n != 2 {
				return fail()
			}
			descriptor.PCurveIDs[i] = pid
			descriptor.SupportIDs[i] = sid
			descriptor.ParameterCurveIDs[i] = items[0]
		}
		out.EdgeCurves = append(out.EdgeCurves, descriptor)
	}
	adjacency := map[int64][]int64{}
	seams := map[string]int{}
	var splineRim int64
	floorRims := 0
	for id, ed := range edges {
		if len(ed.uses) != 2 || ed.uses[0] == ed.uses[1] {
			return fail()
		}
		a, b := faceKinds[ed.faces[0]], faceKinds[ed.faces[1]]
		if a == b {
			seams[a]++
			if ed.a == ed.b {
				return fail()
			}
		} else {
			adjacency[ed.faces[0]] = append(adjacency[ed.faces[0]], ed.faces[1])
			adjacency[ed.faces[1]] = append(adjacency[ed.faces[1]], ed.faces[0])
			if ed.a != ed.b {
				return fail()
			}
			if a == "SPHERICAL_SURFACE" && b == "CYLINDRICAL_SURFACE" || b == "SPHERICAL_SURFACE" && a == "CYLINDRICAL_SURFACE" {
				if splineRim != 0 {
					return fail()
				}
				splineRim = id
			} else if a == "PLANE" && b == "CYLINDRICAL_SURFACE" || b == "PLANE" && a == "CYLINDRICAL_SURFACE" {
				floorRims++
			} else {
				return fail()
			}
		}
	}
	if seams["SPHERICAL_SURFACE"] != 1 || seams["CYLINDRICAL_SURFACE"] != 1 || seams["PLANE"] != 0 || splineRim == 0 || floorRims != 1 {
		return fail()
	}
	seen := map[int64]bool{}
	queue := []int64{faceIDs[0]}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		queue = append(queue, adjacency[id]...)
	}
	if len(seen) != 3 {
		return fail()
	}
	for vid := range vertices {
		graph := links[vid]
		if len(graph) == 0 {
			return fail()
		}
		var start endpoint
		for node, neighbors := range graph {
			if len(neighbors) != 2 {
				return fail()
			}
			start = node
		}
		visited := map[endpoint]bool{}
		todo := []endpoint{start}
		for len(todo) > 0 {
			node := todo[0]
			todo = todo[1:]
			if visited[node] {
				continue
			}
			visited[node] = true
			todo = append(todo, graph[node]...)
		}
		if len(visited) != len(graph) {
			return fail()
		}
	}
	// Detached/reused topological records would hide extra material or owners.
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			switch rec.Type {
			case "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE":
				if len(en.Records) != 1 || !owned[id] {
					return fail()
				}
			case "VERTEX_POINT":
				if len(en.Records) != 1 || !vertices[id] {
					return fail()
				}
			case "OPEN_SHELL", "BREP_WITH_VOIDS", "ORIENTED_CLOSED_SHELL", "FACETED_BREP", "SHELL_BASED_SURFACE_MODEL":
				return fail()
			}
		}
	}

	intersection, e := InspectCylinderPCurve(bytes.NewReader(raw), l, splineRim)
	if e != nil || intersection.BodyID != bodyID || intersection.ShellID != shellID {
		return fail()
	}
	out.Intersection = intersection
	out.Vertices, out.Edges = 3, 4
	out.EulerCharacteristic = 2
	return out, nil
}
