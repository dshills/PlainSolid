package step

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/geom"
	"unicode/utf8"
)

// WriteSplineShells uses Faces as authoritative geometry; Solid is derived and
// ignored. It emits shared edges only for aligned forward model parameters,
// and requires the same complete global embedding proof as ReadSplineShells.
func WriteSplineShells(w io.Writer, m SplineShellModel) error {
	fail := func() error { return errors.New("step: unsupported spline shell model or edge parameterization") }
	if w == nil || len(m.Bodies) == 0 {
		return fail()
	}
	flat := BoundedSplineFaceModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		if !utf8.ValidString(b.Name) || len(b.Faces) < 4 || len(b.Faces) > 64 {
			return fail()
		}
		flat.Faces = append(flat.Faces, b.Faces...)
	}
	var raw bytes.Buffer
	if e := WriteBoundedSplineFaces(&raw, flat); e != nil {
		return e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return e
	}
	allFaces := []int64{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_FACE" {
			allFaces = append(allFaces, id)
		}
	}
	if len(allFaces) != len(flat.Faces) {
		return fail()
	}
	aliases := map[int64]int64{}
	roots := []int64{}
	next := int64(-1)
	offset := 0
	for _, b := range m.Bodies {
		faces := allFaces[offset : offset+len(b.Faces)]
		offset += len(b.Faces)
		vertices := map[geom.Vec3]int64{}
		edges := map[[2]int64]int64{}
		counts := map[int64]int{}
		for _, fid := range faces {
			_, uses, e := analyticFaceLoop(f, fid, 4)
			if e != nil {
				return e
			}
			for _, u := range uses {
				start, end, g, e := analyticEdge(f, u.id)
				if e != nil {
					return e
				}
				ends := [2]int64{start, end}
				for i, vid := range ends {
					p, e := analyticVertex(f, vid)
					if e != nil {
						return e
					}
					if id, ok := vertices[p]; ok {
						aliases[vid] = id
						ends[i] = id
					} else {
						vertices[p] = vid
					}
				}
				if _, reverse := edges[[2]int64{ends[1], ends[0]}]; reverse {
					return fail()
				}
				if canonical, ok := edges[ends]; ok {
					counts[canonical]++
					aliases[u.id] = canonical
					_, _, originalGeometry, _ := analyticEdge(f, canonical)
					existing, _ := simple(f, originalGeometry, "SURFACE_CURVE", 4)
					sc, _ := simple(f, g, "SURFACE_CURVE", 4)
					existing.Args = append([]Value(nil), existing.Args...)
					existing.Args[2] = Value{Kind: Aggregate, Items: append(append([]Value(nil), existing.Args[2].Items...), sc.Args[2].Items...)}
					f.Entities[originalGeometry] = Entity{ID: originalGeometry, Records: []Record{existing}}
				} else {
					edges[ends] = u.id
					counts[u.id] = 1
				}
			}
		}
		for _, count := range counts {
			if count != 2 {
				return fail()
			}
		}
		shellID := next
		next--
		bodyID := next
		next--
		refs := []Value{}
		for _, id := range faces {
			refs = append(refs, Value{Kind: Reference, Ref: id})
		}
		f.Entities[shellID] = Entity{ID: shellID, Records: []Record{{Type: "CLOSED_SHELL", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: refs}}}}}
		f.Entities[bodyID] = Entity{ID: bodyID, Records: []Record{{Type: "MANIFOLD_SOLID_BREP", Args: []Value{{Kind: String, Text: b.Name}, {Kind: Reference, Ref: shellID}}}}}
		roots = append(roots, bodyID)
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := map[int64]int64{}
		var copyOne func(int64) int64
		var walk func(Value)
		walk = func(v Value) {
			if v.Kind == Reference {
				copyOne(v.Ref)
			}
			for _, x := range v.Items {
				walk(x)
			}
		}
		copyOne = func(id int64) int64 {
			if n, ok := ids[id]; ok {
				return n
			}
			if alias, ok := aliases[id]; ok && alias != id {
				n := copyOne(alias)
				ids[id] = n
				return n
			}
			ent := f.Entities[id]
			for _, r := range ent.Records {
				for _, v := range r.Args {
					walk(v)
				}
			}
			ids[id] = emit(assemblyEntity(ent, ids))
			return ids[id]
		}
		result := []int64{}
		for _, id := range roots {
			result = append(result, copyOne(id))
		}
		return result
	})
	if e != nil {
		return e
	}
	if _, e := ReadSplineShells(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
