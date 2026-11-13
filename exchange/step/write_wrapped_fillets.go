package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
)

// WriteWrappedRimFillet constructs exact quarter-toroid supports and shared
// full circular rims. Provided wall charts must match the constructive model.
func WriteWrappedRimFillet(w io.Writer, m WrappedRimFilletBody) error {
	if w == nil {
		return geom.ErrInvalid
	}
	solid, h, e := filletNative(m)
	if e != nil {
		return e
	}
	ids := []int{}
	if h.Blends[0] >= 0 {
		ids = append(ids, h.Blends[0])
	}
	ids = append(ids, h.Lateral)
	if h.Blends[1] >= 0 {
		ids = append(ids, h.Blends[1])
	}
	if len(m.Walls) != 0 && len(m.Walls) != len(ids) {
		return geom.ErrInvalid
	}
	parts := WrappedCylindricalModel{LengthUnit: m.LengthUnit}
	faces := solid.Faces()
	for i, id := range ids {
		support, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return geom.ErrInvalid
		}
		name := ""
		if len(m.Walls) > 0 {
			name = m.Walls[i].Name
			if m.Walls[i].Geometry != nil {
				same, e := geom.CompareExactSurfaces(context.Background(), support.FundamentalPatch(), m.Walls[i].Geometry.FundamentalPatch(), geom.ExactSurfaceComparisonOptions{})
				if e != nil {
					return e
				}
				if !same.Complete || !same.Equal {
					return geom.ErrConvergence
				}
				support = m.Walls[i].Geometry
			}
		}
		parts.Bodies = append(parts.Bodies, WrappedCylindricalBody{Face: WrappedSplineFace{Name: name, Geometry: support}, CapNames: m.CapNames})
	}
	var raw bytes.Buffer
	if e = stageWrappedCappedBodies(&raw, parts, nil); e != nil {
		return e
	}
	source := append([]byte(nil), raw.Bytes()...)
	decoded, e := decodeStagedFilletWalls(source)
	if e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(source), Limits{})
	if e != nil {
		return e
	}
	bodyIDs := []int64{}
	for _, id := range f.Order {
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			bodyIDs = append(bodyIDs, id)
		}
	}
	if len(bodyIDs) != len(parts.Bodies) {
		return geom.ErrInvalid
	}
	sourceFaces := make([][]int64, len(bodyIDs))
	for i, id := range bodyIDs {
		sourceFaces[i], e = bodyFaceIDs(f, id)
		if e != nil || len(sourceFaces[i]) != 3 {
			return geom.ErrInvalid
		}
	}
	aliases := map[int64]int64{}
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	for i := 1; i < len(decoded.Bodies); i++ {
		prev, next := decoded.Bodies[i-1].Face, decoded.Bodies[i].Face
		keep, drop := prev.SideEdges[2], next.SideEdges[0]
		aliases[drop] = keep
		aliases[next.CornerVertices[0]] = prev.CornerVertices[2]
		_, _, kg, e := analyticEdge(f, keep)
		if e != nil {
			return e
		}
		_, _, dg, e := analyticEdge(f, drop)
		if e != nil {
			return e
		}
		left, e := simple(f, kg, "SURFACE_CURVE", 4)
		if e != nil {
			return e
		}
		right, e := simple(f, dg, "SURFACE_CURVE", 4)
		if e != nil {
			return e
		}
		lp, _ := listRefs(left.Args[2], 2)
		rp, _ := listRefs(right.Args[2], 2)
		wallPC := func(ids []int64) int64 {
			for _, id := range ids {
				pc, e := simple(f, id, "PCURVE", 3)
				if e != nil {
					continue
				}
				sid, _ := ref(pc.Args[1])
				if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
					return id
				}
			}
			return 0
		}
		a, b := wallPC(lp), wallPC(rp)
		if a == 0 || b == 0 {
			return geom.ErrInvalid
		}
		left.Args = append([]Value(nil), left.Args...)
		left.Args[2] = Value{Kind: Aggregate, Items: []Value{refv(a), refv(b)}}
		f.Entities[kg] = Entity{ID: kg, Records: []Record{left}}
	}
	var rewrite func(Value) Value
	rewrite = func(v Value) Value {
		if v.Kind == Reference {
			if id, ok := aliases[v.Ref]; ok {
				v.Ref = id
			}
		}
		v.Items = append([]Value(nil), v.Items...)
		for i := range v.Items {
			v.Items[i] = rewrite(v.Items[i])
		}
		return v
	}
	for id, ent := range f.Entities {
		ent.Records = append([]Record(nil), ent.Records...)
		for i, r := range ent.Records {
			r.Args = append([]Value(nil), r.Args...)
			for j := range r.Args {
				r.Args[j] = rewrite(r.Args[j])
			}
			ent.Records[i] = r
		}
		f.Entities[id] = ent
	}
	kept := []int64{}
	for _, fs := range sourceFaces {
		kept = append(kept, fs[0])
	}
	kept = append(kept, sourceFaces[0][1], sourceFaces[len(sourceFaces)-1][2])
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := copySplineGeometry(emit, f, kept)
		shell := emit("CLOSED_SHELL(''," + references(ids) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(m.Name), shell))}
	})
	if e != nil {
		return e
	}
	if _, e = ReadWrappedRimFillet(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}

func decodeStagedFilletWalls(raw []byte) (WrappedCylindricalModel, error) {
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		return WrappedCylindricalModel{}, e
	}
	out := WrappedCylindricalModel{}
	for _, id := range f.Order {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			continue
		}
		sid, _ := ref(face.Args[2])
		if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); !ok {
			continue
		}
		view := cloneOvalFile(f)
		_, uses, e := analyticFaceLoop(f, id, 4)
		if e != nil {
			return out, e
		}
		seen := map[int64]bool{}
		for _, use := range uses {
			if seen[use.id] {
				continue
			}
			seen[use.id] = true
			_, _, gid, e := analyticEdge(f, use.id)
			if e != nil {
				return out, e
			}
			sc, e := ovalRecord(f, gid, "SURFACE_CURVE", 4)
			if e != nil {
				continue
			}
			pcs, e := listRefs(sc.Args[2], 2)
			if e != nil {
				return out, e
			}
			selected := int64(0)
			for _, pcid := range pcs {
				pc, e := simple(f, pcid, "PCURVE", 3)
				if e != nil {
					return out, e
				}
				owner, _ := ref(pc.Args[1])
				if owner == sid {
					selected = pcid
				}
			}
			if selected == 0 {
				return out, geom.ErrInvalid
			}
			sc.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: selected}}}
			ovalSet(view, gid, sc)
		}
		q, e := DecodeWrappedSplineFace(context.Background(), view, id, WrappedSurfaceOptions{})
		if e != nil {
			return out, e
		}
		out.Bodies = append(out.Bodies, WrappedCylindricalBody{Face: q})
	}
	return out, nil
}
