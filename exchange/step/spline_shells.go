package step

import (
	"context"
	"errors"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
)

// SplineShellBody retains its exact rectangular faces and a globally certified
// embedded outward solid. It is not a general STEP shell decoder.
type SplineShellBody struct {
	Name  string
	Faces []BoundedSplineFace
	Solid trimmed.Solid
}
type SplineShellModel struct {
	Bodies     []SplineShellBody
	LengthUnit float64
}

func splineShellSolid(faces []BoundedSplineFace) (trimmed.Solid, error) {
	surfaces := make([]*geom.BSplineSurface, len(faces))
	domains := make([]geom.ParameterRectangle, len(faces))
	reversed := make([]bool, len(faces))
	for i, f := range faces {
		surfaces[i] = f.Geometry
		domains[i] = f.Domain
	}
	s, _, e := trimmed.SewSplineRectangles(context.Background(), surfaces, domains, reversed, trimmed.SewOptions{Tolerance: 1e-10, MaxFaces: 64, MaxUses: 256})
	if e != nil {
		return s, e
	}
	proof, e := s.ValidateSplineRectangleEmbedding(context.Background(), trimmed.SplineRectangleEmbeddingOptions{})
	if e != nil {
		return s, e
	}
	if !proof.Complete {
		return trimmed.Solid{}, errors.New("step: unresolved spline shell embedding")
	}
	return s, nil
}
func decodeSplineShell(f *File, id int64, l Limits) (SplineShellBody, int, error) {
	fail := func() (SplineShellBody, int, error) {
		return SplineShellBody{}, 0, errors.New("step: unsupported or invalid rectangular spline shell")
	}
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	ids, e := listRefs(shell.Args[1], 4)
	if e != nil || len(ids) < 4 || len(ids) > 64 || len(ids) > l.MaxFaces || len(ids) > l.MaxEdgeUses/4 {
		return fail()
	}
	type owner struct {
		surface int64
		sense   bool
	}
	owners := map[int64][]owner{}
	surfaces := map[int64]bool{}
	out := SplineShellBody{Name: name}
	points := 0
	vertices := map[int64]geom.Vec3{}
	pointIDs := map[geom.Vec3]int64{}
	for _, fid := range ids {
		face, uses, e := analyticFaceLoop(f, fid, 4)
		if e != nil {
			return fail()
		}
		support, e := ref(face.Args[2])
		if e != nil || surfaces[support] {
			return fail()
		}
		surfaces[support] = true
		local := *f
		local.Entities = make(map[int64]Entity, len(f.Entities))
		for id, ent := range f.Entities {
			local.Entities[id] = ent
		}
		for _, u := range uses {
			start, end, g, e := analyticEdge(f, u.id)
			if e != nil {
				return fail()
			}
			owners[u.id] = append(owners[u.id], owner{support, u.sense})
			for _, vid := range []int64{start, end} {
				p, e := analyticVertex(f, vid)
				if e != nil {
					return fail()
				}
				if old, ok := pointIDs[p]; ok && old != vid {
					return fail()
				}
				pointIDs[p] = vid
				vertices[vid] = p
			}
			sc, e := simple(f, g, "SURFACE_CURVE", 4)
			if e != nil || sc.Args[3].Kind != Enumeration || (sc.Args[3].Text != "PCURVE_S1" && sc.Args[3].Text != "PCURVE_S2") {
				return fail()
			}
			pcs, e := listRefs(sc.Args[2], 2)
			if e != nil || len(pcs) != 2 {
				return fail()
			}
			selected := int64(0)
			for _, pcid := range pcs {
				pc, e := simple(f, pcid, "PCURVE", 3)
				if e != nil {
					return fail()
				}
				ownerID, e := ref(pc.Args[1])
				if e != nil {
					return fail()
				}
				if ownerID == support {
					if selected != 0 {
						return fail()
					}
					selected = pcid
				}
			}
			if selected == 0 {
				return fail()
			}
			sc.Args = append([]Value(nil), sc.Args...)
			sc.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: selected}}}
			sc.Args[3] = Value{Kind: Enumeration, Text: "PCURVE_S1"}
			local.Entities[g] = Entity{ID: g, Records: []Record{sc}}
		}
		decoded, e := decodeBoundedSplineFace(&local, fid, l.MaxVertices-points)
		if e != nil {
			return fail()
		}
		points += len(decoded.Geometry.Data().Poles)
		out.Faces = append(out.Faces, decoded)
	}
	if len(vertices) > l.MaxVertices {
		return fail()
	}
	for eid, os := range owners {
		if len(os) != 2 || os[0].surface == os[1].surface || os[0].sense == os[1].sense {
			return fail()
		}
		_, _, g, _ := analyticEdge(f, eid)
		sc, _ := simple(f, g, "SURFACE_CURVE", 4)
		pcs, _ := listRefs(sc.Args[2], 2)
		seen := map[int64]bool{}
		for _, pcid := range pcs {
			pc, _ := simple(f, pcid, "PCURVE", 3)
			support, _ := ref(pc.Args[1])
			if seen[support] || (support != os[0].surface && support != os[1].surface) {
				return fail()
			}
			seen[support] = true
		}
	}
	out.Solid, e = splineShellSolid(out.Faces)
	if e != nil {
		return SplineShellBody{}, 0, e
	}
	if len(out.Solid.Vertices()) != len(vertices) || len(out.Solid.Edges()) != len(owners) {
		return fail()
	}
	return out, points, nil
}
func ReadSplineShells(r io.Reader, l Limits) (SplineShellModel, error) {
	l, e := l.defaults()
	if e != nil {
		return SplineShellModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return SplineShellModel{}, e
	}
	m := SplineShellModel{}
	fail := func() (SplineShellModel, error) {
		return SplineShellModel{}, errors.New("step: unsupported spline shell representation")
	}
	used := map[int64]bool{}
	usedFaces := map[int64]bool{}
	points, faces, uses := 0, 0, 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" {
				return fail()
			}
			if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
				return fail()
			}
		}
		if len(ent.Records) != 1 || ent.Records[0].Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		rep, e := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return fail()
		}
		unit, e := contextUnits(f, ctx)
		if e != nil || m.LengthUnit != 0 && m.LengthUnit != unit {
			return fail()
		}
		m.LengthUnit = unit
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, item := range items {
			if _, e := simple(f, item, "AXIS2_PLACEMENT_3D", 4); e == nil {
				if _, e := placement(f, item); e != nil {
					return fail()
				}
				continue
			}
			if used[item] || len(m.Bodies) >= l.MaxSolids {
				return fail()
			}
			remaining := l
			remaining.MaxVertices -= points
			remaining.MaxFaces -= faces
			remaining.MaxEdgeUses -= uses
			b, n, e := decodeSplineShell(f, item, remaining)
			if e != nil {
				return SplineShellModel{}, e
			}
			points += n
			faces += len(b.Faces)
			uses += 4 * len(b.Faces)
			used[item] = true
			m.Bodies = append(m.Bodies, b)
			fr, e := bodyFaceIDs(f, item)
			if e != nil {
				return fail()
			}
			for _, fid := range fr {
				if usedFaces[fid] {
					return fail()
				}
				usedFaces[fid] = true
			}
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if r.Type == "MANIFOLD_SOLID_BREP" && !used[id] || r.Type == "ADVANCED_FACE" && !usedFaces[id] {
				return fail()
			}
		}
	}
	if len(m.Bodies) == 0 {
		return fail()
	}
	return m, nil
}
