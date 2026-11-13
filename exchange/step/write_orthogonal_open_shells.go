package step

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/geom"
	"sort"
	"unicode/utf8"
)

// WriteOrthogonalSplineOpenShells emits explicitly shared topology, never
// positional welding. Uses for each face and loop must follow source physical traversal order, with
// canonical vertices and Forward matching increasing iso-curve parameters.
// The staged STEP graph is independently decoded and certified before output.
// This preserves the reader's local topology/continuity guarantee, not global
// embedding or solid validity. Source numeric entity IDs are not preserved.
func WriteOrthogonalSplineOpenShells(w io.Writer, m OrthogonalSplineOpenShellModel) error {
	return writeOrthogonalSplineShells(w, m, false, supportAxis)
}

func writeOrthogonalSplineShells(w io.Writer, m OrthogonalSplineOpenShellModel, closed bool, mode affineSupportMode) error {
	fail := func() error { return errors.New("step: invalid orthogonal open-shell writer graph") }
	if w == nil || len(m.Shells) == 0 {
		return fail()
	}
	flat := OrthogonalSplineFaceModel{LengthUnit: m.LengthUnit}
	seenFaces := map[int64]bool{}
	sizes := []int{}
	faceCount := 0
	for _, s := range m.Shells {
		if !utf8.ValidString(s.Name) || len(s.Faces) < 2 || len(s.FaceIDs) != len(s.Faces) || faceCount+len(s.Faces) > MaxOrthogonalShellFaces || len(s.FaceHoles) != 0 && len(s.FaceHoles) != len(s.Faces) {
			return fail()
		}
		counts := make([]map[int]int, len(s.Faces))
		for i := range counts {
			counts[i] = map[int]int{}
		}
		for _, u := range s.Uses {
			if u.Face < 0 || u.Face >= len(s.Faces) || u.EdgeID <= 0 || u.StartVertexID <= 0 || u.EndVertexID <= 0 || u.StartVertexID == u.EndVertexID {
				return fail()
			}
			if u.Loop < 0 {
				return fail()
			}
			counts[u.Face][u.Loop]++
		}
		for i, id := range s.FaceIDs {
			if id <= 0 || seenFaces[id] || counts[i][0] != len(s.Faces[i].Boundary) {
				return fail()
			}
			seenFaces[id] = true
			holes := [][]geom.Vec2{}
			if len(s.FaceHoles) > 0 {
				holes = s.FaceHoles[i]
			}
			if len(holes) > 0 {
				if e := validateOrthogonalHoles(s.Faces[i].Boundary, holes); e != nil {
					return e
				}
			}
			if len(counts[i]) != 1+len(holes) {
				return fail()
			}
			for j, h := range holes {
				if counts[i][j+1] != len(h) {
					return fail()
				}
			}
			flat.Faces = append(flat.Faces, s.Faces[i])
			for _, h := range holes {
				flat.Faces = append(flat.Faces, OrthogonalSplineFace{Geometry: s.Faces[i].Geometry, Boundary: h})
			}
			sizes = append(sizes, 1+len(holes))
			faceCount++

		}
		if e := orthogonalShellIncidence(s, closed); e != nil {
			return e
		}
	}
	var staged bytes.Buffer
	if e := writeOrthogonalSplineFaces(&staged, flat, sizes); e != nil {
		return e
	}
	stageLimits, _ := (Limits{}).defaults()
	f, e := Parse(&staged, stageLimits)
	if e != nil {
		return e
	}
	faceIDs := []int64{}
	next := int64(1)
	for _, id := range f.Order {
		next = max(next, id+1)
		if _, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
			faceIDs = append(faceIDs, id)
		}
	}
	if len(faceIDs) != faceCount {
		return fail()
	}
	aliases := map[int64]int64{}
	vertices := map[int64]int64{}
	generatedVertexOwner := map[int64]int64{}
	edges := map[int64]int64{}
	edgeEnds := map[int64][2]int64{}
	edgeShell := map[int64]int{}
	roots := []int64{}
	offset := 0
	for si, s := range m.Shells {
		uses := make([][]OrthogonalShellUse, len(s.Faces))
		for _, u := range s.Uses {
			uses[u.Face] = append(uses[u.Face], u)
		}
		for i := range s.Faces {
			fid := faceIDs[offset+i]
			sort.SliceStable(uses[i], func(a, b int) bool { return uses[i][a].Loop < uses[i][b].Loop })
			_, _, generated, e := decodeOrthogonalShellFace(f, fid, stageLimits.MaxVertices, stageLimits.MaxEdgeUses)
			if e != nil {
				return e
			}
			for j, source := range uses[i] {
				g := generated[j]
				if source.Forward != g.Forward || source.Loop != g.Loop {
					return fail()
				}
				start, end, scid, e := analyticEdge(f, g.EdgeID)
				if e != nil {
					return e
				}
				for k, vid := range []int64{start, end} {
					topo := source.StartVertexID
					if k == 1 {
						topo = source.EndVertexID
					}
					if old, ok := generatedVertexOwner[vid]; ok && old != topo {
						return fail()
					}
					generatedVertexOwner[vid] = topo
					if canonical, ok := vertices[topo]; ok {
						a, e := analyticVertex(f, canonical)
						if e != nil {
							return e
						}
						b, e := analyticVertex(f, vid)
						if e != nil || a != b {
							return fail()
						}
						aliases[vid] = canonical
					} else {
						vertices[topo] = vid
					}
				}
				if canonical, ok := edges[source.EdgeID]; ok {
					if edgeShell[source.EdgeID] != si || edgeEnds[source.EdgeID] != [2]int64{source.StartVertexID, source.EndVertexID} {
						return fail()
					}
					aliases[g.EdgeID] = canonical
					_, _, firstSC, e := analyticEdge(f, canonical)
					if e != nil {
						return e
					}
					first, e := simple(f, firstSC, "SURFACE_CURVE", 4)
					if e != nil {
						return e
					}
					second, e := simple(f, scid, "SURFACE_CURVE", 4)
					if e != nil {
						return e
					}
					first.Args = append([]Value(nil), first.Args...)
					first.Args[2].Items = append(append([]Value(nil), first.Args[2].Items...), second.Args[2].Items...)
					f.Entities[firstSC] = Entity{ID: firstSC, Records: []Record{first}}
				} else {
					edges[source.EdgeID] = g.EdgeID
					edgeEnds[source.EdgeID] = [2]int64{source.StartVertexID, source.EndVertexID}
					edgeShell[source.EdgeID] = si
				}
			}
		}
		values := []Value{}
		for _, fid := range faceIDs[offset : offset+len(s.Faces)] {
			values = append(values, Value{Kind: Reference, Ref: fid})
		}
		shellID := next
		next++
		modelID := next
		next++
		f.Entities[shellID] = Entity{ID: shellID, Records: []Record{{Type: "OPEN_SHELL", Args: []Value{{Kind: String, Text: s.Name}, {Kind: Aggregate, Items: values}}}}}
		f.Entities[modelID] = Entity{ID: modelID, Records: []Record{{Type: "SHELL_BASED_SURFACE_MODEL", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: shellID}}}}}}}
		if closed {
			f.Entities[shellID] = Entity{ID: shellID, Records: []Record{{Type: "CLOSED_SHELL", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: values}}}}}
			f.Entities[modelID] = Entity{ID: modelID, Records: []Record{{Type: "MANIFOLD_SOLID_BREP", Args: []Value{{Kind: String, Text: s.Name}, {Kind: Reference, Ref: shellID}}}}}
		}

		roots = append(roots, modelID)
		offset += len(s.Faces)
	}
	var rewrite func(Value) Value
	rewrite = func(v Value) Value {
		if v.Kind == Reference {
			if id, ok := aliases[v.Ref]; ok {
				v.Ref = id
			}
		}
		if v.Items != nil {
			v.Items = append([]Value(nil), v.Items...)
			for i := range v.Items {
				v.Items[i] = rewrite(v.Items[i])
			}
		}
		return v
	}
	for id, ent := range f.Entities {
		for i := range ent.Records {
			ent.Records[i].Args = append([]Value(nil), ent.Records[i].Args...)
			for j := range ent.Records[i].Args {
				ent.Records[i].Args[j] = rewrite(ent.Records[i].Args[j])
			}
		}
		f.Entities[id] = ent
	}
	var output bytes.Buffer
	representation := "MANIFOLD_SURFACE_SHAPE_REPRESENTATION"
	if closed {
		representation = "ADVANCED_BREP_SHAPE_REPRESENTATION"
	}
	if e := writeStandalone(&output, m.LengthUnit, "", representation, func(emit func(string) int64) []int64 { return copySplineGeometry(emit, f, roots) }); e != nil {
		return e
	}
	if closed {
		if mode == supportAffine {
			if _, e := ReadAffineSplineShells(bytes.NewReader(output.Bytes()), Limits{}); e != nil {
				return e
			}
		} else if mode == supportOrthogonal {
			if _, e := ReadOrthogonalAffineSplineShells(bytes.NewReader(output.Bytes()), Limits{}); e != nil {
				return e
			}
		} else {
			if _, e := ReadAxisAlignedSplineShells(bytes.NewReader(output.Bytes()), Limits{}); e != nil {
				return e
			}
		}

	} else {
		if _, e := ReadOrthogonalSplineOpenShells(bytes.NewReader(output.Bytes()), Limits{}); e != nil {
			return e
		}
	}
	n, e := w.Write(output.Bytes())
	if e == nil && n != output.Len() {
		e = io.ErrShortWrite
	}
	return e
}
