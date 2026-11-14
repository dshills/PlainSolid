package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteWrappedStationLoft builds canonical circular bands from explicit stations,
// retains provided wall/cap names, and emits shared full station rim topology.
func WriteWrappedStationLoft(w io.Writer, m WrappedStationLoftBody) error {
	if w == nil || len(m.Stations) < 3 || len(m.Stations) > 65 || len(m.Walls) != 0 && len(m.Walls) != len(m.Stations)-1 {
		return geom.ErrInvalid
	}
	solid, h, e := trimmed.LoftWrappedCircularStations(context.Background(), m.Center, m.Stations, 1e-10, geom.SurfacePatchOptions{})
	if e != nil {
		return e
	}
	faces := solid.Faces()
	parts := WrappedFrustumModel{LengthUnit: m.LengthUnit}
	for i, fid := range h.WallFaces {
		support, ok := faces[fid].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return geom.ErrInvalid
		}
		if len(m.Walls) != 0 && m.Walls[i].Geometry != nil {
			support = m.Walls[i].Geometry
			proof, e := geom.CertifyWrappedFrustumEmbedding(context.Background(), support, geom.SurfacePatchOptions{})
			if e != nil {
				return e
			}
			equal := func(v *big.Rat, x float64) bool { return v != nil && v.Cmp(new(big.Rat).SetFloat64(x)) == 0 }
			if !proof.Complete || !proof.Circular || !equal(proof.CenterX, m.Center.X) || !equal(proof.CenterY, m.Center.Y) || !equal(proof.LowerZ, m.Stations[i].Z) || !equal(proof.UpperZ, m.Stations[i+1].Z) || !equal(proof.LowerRadius, m.Stations[i].Radius) || !equal(proof.UpperRadius, m.Stations[i+1].Radius) {
				return geom.ErrConvergence
			}
		}
		name := ""
		if len(m.Walls) != 0 {
			name = m.Walls[i].Name
		}
		parts.Bodies = append(parts.Bodies, WrappedFrustumBody{Face: WrappedSplineFace{Name: name, Geometry: support}, CapNames: m.CapNames})
	}
	return writeWrappedStationGraph(w, m.Name, parts, true)
}

func writeWrappedStationGraph(w io.Writer, name string, parts WrappedFrustumModel, circular bool) error {
	var e error
	var raw bytes.Buffer
	if e = WriteWrappedFrusta(&raw, parts); e != nil {
		return e
	}
	source := append([]byte(nil), raw.Bytes()...)
	decoded, e := ReadWrappedFrusta(bytes.NewReader(source), Limits{}, WrappedSurfaceOptions{})
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
	e = writeStandalone(&out, parts.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := copySplineGeometry(emit, f, kept)
		shell := emit("CLOSED_SHELL(''," + references(ids) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), shell))}
	})
	if e != nil {
		return e
	}
	if _, e = readWrappedStationLoft(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{}, circular); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}

// WrappedStationLoftFromSolid validates native canonical topology before
// recovering exact representable stations. Original band supports are retained.
func WrappedStationLoftFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedStationLoftBody, error) {
	if !validLengthUnit(unit) {
		return WrappedStationLoftBody{}, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCircularStationLoft(ctx, o)
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	if !p.Complete || len(p.Heights) < 3 || len(p.Heights) > 65 {
		return WrappedStationLoftBody{}, geom.ErrConvergence
	}
	x, e := geom.RepresentedExactScalar(p.Center[0])
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	y, e := geom.RepresentedExactScalar(p.Center[1])
	if e != nil {
		return WrappedStationLoftBody{}, e
	}
	out := WrappedStationLoftBody{Name: name, Center: geom.Vec2{X: x, Y: y}, Solid: s, LengthUnit: unit}
	for i, z := range p.Heights {
		h, e := geom.RepresentedExactScalar(z)
		if e != nil {
			return WrappedStationLoftBody{}, e
		}
		r, e := geom.RepresentedExactScalar(p.Radii[i])
		if e != nil {
			return WrappedStationLoftBody{}, e
		}
		out.Stations = append(out.Stations, trimmed.WrappedLoftStation{Z: h, Radius: r})
	}
	faces := s.Faces()
	for _, id := range p.History.WallFaces {
		w, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return WrappedStationLoftBody{}, geom.ErrConvergence
		}
		out.Walls = append(out.Walls, WrappedSplineFace{Geometry: w})
	}
	return out, nil
}
