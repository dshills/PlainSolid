package step

import (
	"errors"
	"io"
	"plainsolid/brep"
	"plainsolid/geom"
	"strings"
)

// OrientedVoidShell preserves the STEP source shell and its orientation flag.
// Reversed=true corresponds to ORIENTED_CLOSED_SHELL orientation .F.
type OrientedVoidShell struct {
	Shell    brep.Solid
	Reversed bool
}
type CavityBody struct {
	Name  string
	Outer brep.Solid
	Voids []OrientedVoidShell
	Solid brep.Solid
}
type CavityModel struct {
	Bodies     []CavityBody
	LengthUnit float64
}

func convexPlanes(s brep.Solid) ([][3]geom.Vec3, error) {
	fail := func() ([][3]geom.Vec3, error) {
		return nil, errors.New("step: cavity shells must be convex, closed and outward")
	}
	if e := s.Validate(); e != nil {
		return nil, e
	}
	if s.Topology().Shells != 1 || s.Tessellate().SignedVolume() <= 0 {
		return fail()
	}
	v, e := s.Vertices(), s.Edges()
	planes := [][3]geom.Vec3{}
	for _, f := range s.Faces() {
		if len(f.Holes) > 0 {
			return fail()
		}
		points := []geom.Vec3{}
		for _, u := range f.Boundary {
			x := e[u.Edge].Start
			if u.Reversed {
				x = e[u.Edge].End
			}
			points = append(points, v[x])
		}
		if len(points) < 3 {
			return fail()
		}
		var p [3]geom.Vec3
		found := false
		for i := 2; i < len(points); i++ {
			if points[1].Sub(points[0]).Cross(points[i].Sub(points[0])).Length() > 0 {
				p = [3]geom.Vec3{points[0], points[1], points[i]}
				found = true
				break
			}
		}
		if !found {
			return fail()
		}
		for _, q := range v {
			sign, err := geom.Orient3D(p[0], p[1], p[2], q)
			if err != nil || sign < 0 {
				return fail()
			}
		}
		planes = append(planes, p)
	}
	return planes, nil
}
func validateCavity(b *CavityBody) error {
	nv, nf := len(b.Outer.Vertices()), len(b.Outer.Faces())
	if len(b.Voids) > 256 {
		return errors.New("step: cavity proof shell limit")
	}
	for _, v := range b.Voids {
		nv += len(v.Shell.Vertices())
		nf += len(v.Shell.Faces())
	}
	if nv <= 0 || nf <= 0 || nv > 4000000/nf {
		return errors.New("step: cavity convex proof work limit")
	}

	outer, e := convexPlanes(b.Outer)
	if e != nil {
		return e
	}
	if len(b.Voids) == 0 {
		return errors.New("step: cavity requires void shells")
	}
	var positives []brep.Solid
	var planes [][][3]geom.Vec3
	for _, void := range b.Voids {
		if !void.Reversed {
			return errors.New("step: only reversed outward void shells supported")
		}
		positive := void.Shell
		if !void.Reversed {
			positive = positive.Reversed()
		}
		p, e := convexPlanes(positive)
		if e != nil {
			return e
		}
		for _, plane := range outer {
			for _, v := range positive.Vertices() {
				s, e := geom.Orient3D(plane[0], plane[1], plane[2], v)
				if e != nil || s <= 0 {
					return errors.New("step: void not strictly contained")
				}
			}
		}
		positives = append(positives, positive)
		planes = append(planes, p)
	}
	separated := func(p [][3]geom.Vec3, s brep.Solid) bool {
		for _, plane := range p {
			all := true
			for _, v := range s.Vertices() {
				sign, e := geom.Orient3D(plane[0], plane[1], plane[2], v)
				if e != nil || sign >= 0 {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
		return false
	}
	for i := range positives {
		for j := 0; j < i; j++ {
			if !separated(planes[i], positives[j]) && !separated(planes[j], positives[i]) {
				return errors.New("step: void separation cannot be certified")
			}
		}
	}
	vertices, edges, faces := b.Outer.Vertices(), b.Outer.Edges(), b.Outer.Faces()
	for _, v := range positives {
		s := v.Reversed()
		nv, ne := len(vertices), len(edges)
		vertices = append(vertices, s.Vertices()...)
		for _, edge := range s.Edges() {
			edges = append(edges, brep.Edge{Start: edge.Start + nv, End: edge.End + nv})
		}
		for _, f := range s.Faces() {
			for i := range f.Boundary {
				f.Boundary[i].Edge += ne
			}
			faces = append(faces, f)
		}
	}
	b.Solid, e = brep.NewSolid(vertices, edges, faces)
	return e
}
func readCavityBody(f *File, id int64, l Limits) (CavityBody, error) {
	r, e := simple(f, id, "BREP_WITH_VOIDS", 3)
	if e != nil {
		return CavityBody{}, e
	}
	name, e := textValue(r.Args[0])
	if e != nil {
		return CavityBody{}, e
	}
	outer, e := ref(r.Args[1])
	if e != nil {
		return CavityBody{}, e
	}
	voidIDs, e := listRefs(r.Args[2], 1)
	if e != nil || len(voidIDs) > l.MaxFaces {
		return CavityBody{}, errors.New("step: invalid void count")
	}
	local := *f
	local.Entities = make(map[int64]Entity, len(f.Entities)+1)
	for id, ent := range f.Entities {
		local.Entities[id] = ent
	}
	nv, nf, nu := 0, 0, 0
	shell := func(sid int64) (brep.Solid, error) {
		local.Entities[-1] = Entity{ID: -1, Records: []Record{{Type: "MANIFOLD_SOLID_BREP", Args: []Value{{Kind: String}, {Kind: Reference, Ref: sid}}}}}
		b, e := readBody(&local, -1, l.MaxVertices-nv, l.MaxFaces-nf, l.MaxEdgeUses-nu, true)
		if e != nil {
			return brep.Solid{}, e
		}
		nv += len(b.Solid.Vertices())
		nf += len(b.Solid.Faces())
		for _, f := range b.Solid.Faces() {
			nu += len(f.Boundary)
			for _, h := range f.Holes {
				nu += len(h)
			}
		}
		return b.Solid, nil
	}
	b := CavityBody{Name: name}
	b.Outer, e = shell(outer)
	if e != nil {
		return b, e
	}
	seen := map[int64]bool{outer: true}
	for _, vid := range voidIDs {
		v, e := simple(f, vid, "ORIENTED_CLOSED_SHELL", 4)
		if e != nil || v.Args[1].Kind != Derived {
			return CavityBody{}, errors.New("step: invalid oriented void shell")
		}
		if _, e = textValue(v.Args[0]); e != nil {
			return CavityBody{}, e
		}
		sid, e := ref(v.Args[2])
		if e != nil || seen[sid] {
			return CavityBody{}, errors.New("step: repeated cavity shell")
		}
		seen[sid] = true
		orientation, e := boolean(v.Args[3])
		if e != nil {
			return CavityBody{}, e
		}
		s, e := shell(sid)
		if e != nil {
			return CavityBody{}, e
		}
		b.Voids = append(b.Voids, OrientedVoidShell{s, !orientation})
	}
	e = validateCavity(&b)
	return b, e
}

// ReadCavities imports strictly contained convex planar void shells.
func ReadCavities(r io.Reader, l Limits) (CavityModel, error) {
	l, e := l.defaults()
	if e != nil {
		return CavityModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return CavityModel{}, e
	}
	m := CavityModel{}
	used := map[int64]bool{}
	totalVertices, totalFaces, totalUses := 0, 0, 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || rec.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" {
				return CavityModel{}, errors.New("step: cavity assembly placement unsupported")
			}
		}
		if len(ent.Records) != 1 {
			for _, r := range ent.Records {
				if strings.HasSuffix(r.Type, "SHAPE_REPRESENTATION") {
					return CavityModel{}, errors.New("step: complex cavity representation unsupported")
				}
			}
			continue
		}
		kind := ent.Records[0].Type
		if strings.HasSuffix(kind, "SHAPE_REPRESENTATION") && kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			return CavityModel{}, errors.New("step: unsupported cavity representation")
		}
		if kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		rep, e := simple(f, id, kind, 3)
		if e != nil {
			return CavityModel{}, e
		}
		if _, e = textValue(rep.Args[0]); e != nil {
			return CavityModel{}, e
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return CavityModel{}, e
		}
		unit, e := contextUnits(f, ctx)
		if e != nil {
			return CavityModel{}, e
		}
		if m.LengthUnit != 0 && m.LengthUnit != unit {
			return CavityModel{}, errors.New("step: mixed cavity units")
		}
		m.LengthUnit = unit
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return CavityModel{}, e
		}
		for _, bid := range items {
			if x := f.Entities[bid]; len(x.Records) == 1 && x.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				if _, e = placement(f, bid); e != nil {
					return CavityModel{}, e
				}
				continue
			}
			if used[bid] || len(m.Bodies) >= l.MaxSolids {
				return CavityModel{}, errors.New("step: repeated cavity body or resource limit")
			}
			body, e := readCavityBody(f, bid, Limits{MaxVertices: l.MaxVertices - totalVertices, MaxFaces: l.MaxFaces - totalFaces, MaxEdgeUses: l.MaxEdgeUses - totalUses})
			if e != nil {
				return CavityModel{}, e
			}
			nv, nf := len(body.Solid.Vertices()), len(body.Solid.Faces())
			if nv > l.MaxVertices-totalVertices || nf > l.MaxFaces-totalFaces || len(body.Solid.Edges()) > (l.MaxEdgeUses-totalUses)/2 {
				return CavityModel{}, errors.New("step: cavity topology limit")
			}
			totalVertices += nv
			totalFaces += nf
			totalUses += len(body.Solid.Edges()) * 2
			used[bid] = true
			m.Bodies = append(m.Bodies, body)
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if (strings.HasSuffix(r.Type, "_BREP") || r.Type == "BREP_WITH_VOIDS") && !used[id] {
				return CavityModel{}, errors.New("step: unrepresented cavity body")
			}
		}
	}
	if len(m.Bodies) == 0 {
		return CavityModel{}, errors.New("step: no supported cavity bodies")
	}
	return m, nil
}
