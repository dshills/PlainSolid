package step

import (
	"bytes"
	"context"
	"errors"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedCavityBody is one strictly enclosed coaxial rational void. Both source
// support descriptors point outward from their own cylinder. STEP explicitly
// reverses the inner CLOSED_SHELL with ORIENTED_CLOSED_SHELL orientation F.
type WrappedCavityBody struct {
	Name                         string
	Outer, Inner                 WrappedSplineFace
	OuterCapNames, InnerCapNames [2]string
	Solid                        trimmed.Solid
	LengthUnit                   float64
}

// ReadWrappedCavity requires BREP_WITH_VOIDS with exactly one explicitly
// reversed outward inner shell. Strict radial/axial containment and exact
// normalized profile identity are proved before constructing material.
func ReadWrappedCavity(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedCavityBody, error) {
	fail := func() (WrappedCavityBody, error) {
		return WrappedCavityBody{}, errors.New("step: unsupported wrapped cavity graph/orientation")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedCavityBody{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedCavityBody{}, e
	}
	if l.MaxFaces < 6 || l.MaxEdgeUses < 12 {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedCavityBody{}, e
	}
	name, e := splitWrappedVoid(f, 3, true, o)
	if e != nil {
		return WrappedCavityBody{}, e
	}
	raw := serializeWrappedProofFile(f)
	internal := l
	internal.MaxSolids = max(2, l.MaxSolids)
	m, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), internal, o)
	if e != nil {
		return WrappedCavityBody{}, e
	}
	if len(m.Bodies) != 2 {
		return fail()
	}
	a, b := m.Bodies[0], m.Bodies[1]
	if a.Embedding.LowerZ.Cmp(b.Embedding.LowerZ) >= 0 || a.Embedding.UpperZ.Cmp(b.Embedding.UpperZ) <= 0 {
		return fail()
	}
	solid, _, e := trimmed.NewWrappedCylinderBore(context.Background(), a.Face.Geometry, b.Face.Geometry, 1e-10, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return WrappedCavityBody{}, e
	}
	if len(solid.Topology().Shells) != 2 {
		return fail()
	}
	return WrappedCavityBody{Name: name, Outer: a.Face, Inner: b.Face, OuterCapNames: a.CapNames, InnerCapNames: b.CapNames, Solid: solid, LengthUnit: m.LengthUnit}, nil
}

// WriteWrappedCavity emits outward source shells and an explicit reversed void
// occurrence. Cached Solid is ignored and material containment is re-proved.
func WriteWrappedCavity(w io.Writer, m WrappedCavityBody) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var raw bytes.Buffer
	pair := WrappedCylindricalModel{LengthUnit: m.LengthUnit, Bodies: []WrappedCylindricalBody{{Name: m.Name, Face: m.Outer, CapNames: m.OuterCapNames}, {Name: m.Name, Face: m.Inner, CapNames: m.InnerCapNames}}}
	if e := WriteWrappedCylindricalBodies(&raw, pair); e != nil {
		return e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return e
	}
	out, e := joinWrappedVoid(f)
	if e != nil {
		return e
	}
	if _, e = ReadWrappedCavity(bytes.NewReader(out), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(out)
	if e == nil && n != len(out) {
		e = io.ErrShortWrite
	}
	return e
}

func splitWrappedVoid(f *File, facesPerShell int, normalize bool, o WrappedSurfaceOptions) (string, error) {
	fail := func() (string, error) { return "", errors.New("step: unsupported wrapped void graph/orientation") }
	var e error
	bodyID := int64(0)
	next := int64(1)
	for _, id := range f.Order {
		if id >= next {
			next = id + 1
		}
		for _, rec := range f.Entities[id].Records {
			if rec.Type == "MANIFOLD_SOLID_BREP" {
				return fail()
			}
			if rec.Type == "BREP_WITH_VOIDS" {
				if bodyID != 0 {
					return fail()
				}
				bodyID = id
			}
		}
	}
	if bodyID == 0 {
		return fail()
	}
	body, e := simple(f, bodyID, "BREP_WITH_VOIDS", 3)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	outerID, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	voids, e := listRefs(body.Args[2], 1)
	if e != nil || len(voids) != 1 {
		return fail()
	}
	oriented, e := simple(f, voids[0], "ORIENTED_CLOSED_SHELL", 4)
	if e != nil || oriented.Args[1].Kind != Derived {
		return fail()
	}
	sense, e := boolean(oriented.Args[3])
	if e != nil || sense {
		return fail()
	}
	innerID, e := ref(oriented.Args[2])
	if e != nil || innerID == outerID {
		return fail()
	}
	set := func(id int64, r Record) { f.Entities[id] = Entity{ID: id, Records: []Record{r}} }
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	for _, shellID := range []int64{outerID, innerID} {
		shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
		if e != nil {
			return fail()
		}
		faces, e := listRefs(shell.Args[1], facesPerShell)
		if e != nil || len(faces) != facesPerShell {
			return fail()
		}
		for _, fid := range faces {
			face, e := simple(f, fid, "ADVANCED_FACE", 4)
			if e != nil {
				return fail()
			}
			sid, _ := ref(face.Args[2])
			if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
				if normalize {
					if e = normalizeWrappedIsoSplines(f, sid, o.MaxPoles); e != nil {
						return "", e
					}
				}
			}
		}
	}
	for id, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "ORIENTED_CLOSED_SHELL" && id != voids[0] {
				return fail()
			}
		}
	}
	delete(f.Entities, voids[0])
	set(bodyID, Record{Type: "MANIFOLD_SOLID_BREP", Args: []Value{body.Args[0], refv(outerID)}})
	newBody := next
	f.Order = append(f.Order, newBody)
	set(newBody, Record{Type: "MANIFOLD_SOLID_BREP", Args: []Value{body.Args[0], refv(innerID)}})
	reps := 0
	for _, id := range f.Order {
		rep, e := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			continue
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, item := range items {
			if item == bodyID {
				reps++
				rep.Args = append([]Value(nil), rep.Args...)
				rep.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), rep.Args[1].Items...), refv(newBody))}
				set(id, rep)
			}
		}
	}
	if reps != 1 {
		return fail()
	}

	return name, nil
}

func joinWrappedVoid(f *File) ([]byte, error) {
	bodies := []int64{}
	next := int64(1)
	for _, id := range f.Order {
		if id >= next {
			next = id + 1
		}
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			bodies = append(bodies, id)
		}
	}
	if len(bodies) != 2 {
		return nil, geom.ErrInvalid
	}
	a, _ := simple(f, bodies[0], "MANIFOLD_SOLID_BREP", 2)
	b, _ := simple(f, bodies[1], "MANIFOLD_SOLID_BREP", 2)
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	set := func(id int64, r Record) { f.Entities[id] = Entity{ID: id, Records: []Record{r}} }
	set(next, Record{Type: "ORIENTED_CLOSED_SHELL", Args: []Value{{Kind: String}, {Kind: Derived}, b.Args[1], {Kind: Enumeration, Text: "F"}}})
	f.Order = append(f.Order, next)
	set(bodies[0], Record{Type: "BREP_WITH_VOIDS", Args: []Value{a.Args[0], a.Args[1], {Kind: Aggregate, Items: []Value{refv(next)}}}})
	delete(f.Entities, bodies[1])
	for id, ent := range f.Entities {
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			rep := ent.Records[0]
			items := []Value{}
			for _, v := range rep.Args[1].Items {
				if v.Kind != Reference || v.Ref != bodies[1] {
					items = append(items, v)
				}
			}
			rep.Args = append([]Value(nil), rep.Args...)
			rep.Args[1] = Value{Kind: Aggregate, Items: items}
			set(id, rep)
		}
	}
	out := serializeWrappedProofFile(f)

	return out, nil
}
