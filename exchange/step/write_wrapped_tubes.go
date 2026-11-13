package step

import (
	"bytes"
	"fmt"
	"io"
)

// WriteWrappedTube emits one explicit four-face solid. The outer and inner
// wrapped supports and names are authoritative; Solid is re-proved on readback.
func WriteWrappedTube(w io.Writer, m WrappedTubeBody) error { return writeWrappedBore(w, m, false) }

func writeWrappedBore(w io.Writer, m WrappedTubeBody, blind bool) error {
	if w == nil {
		return fmt.Errorf("step: nil wrapped tube writer")
	}
	var raw bytes.Buffer
	pair := WrappedCylindricalModel{LengthUnit: m.LengthUnit, Bodies: []WrappedCylindricalBody{{Name: m.Name, Face: m.Outer, CapNames: m.CapNames}, {Name: m.Name, Face: m.Inner, CapNames: m.CapNames}}}
	if blind {
		pair.Bodies[1].CapNames = [2]string{m.FloorName, m.FloorName}
	}
	if e := WriteWrappedCylindricalBodies(&raw, pair); e != nil {
		return e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return e
	}
	bodyIDs := []int64{}
	for _, id := range f.Order {
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			bodyIDs = append(bodyIDs, id)
		}
	}
	if len(bodyIDs) != 2 {
		return fmt.Errorf("step: staged tube bodies")
	}
	set := func(id int64, r Record) { f.Entities[id] = Entity{ID: id, Records: []Record{r}} }
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	shells := [2]int64{}
	faces := [2][]int64{}
	for i, id := range bodyIDs {
		body, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
		shells[i], _ = ref(body.Args[1])
		shell, _ := simple(f, shells[i], "CLOSED_SHELL", 2)
		faces[i], e = listRefs(shell.Args[1], 3)
		if e != nil || len(faces[i]) != 3 {
			return fmt.Errorf("step: staged cylinder faces")
		}
	}
	inner, _ := simple(f, faces[1][0], "ADVANCED_FACE", 4)
	bounds, _ := listRefs(inner.Args[1], 1)
	bound := f.Entities[bounds[0]].Records[0]
	inner.Args = append([]Value(nil), inner.Args...)
	inner.Args[3] = Value{Kind: Enumeration, Text: "F"}
	bound.Args = append([]Value(nil), bound.Args...)
	bound.Args[2] = Value{Kind: Enumeration, Text: "F"}
	set(faces[1][0], inner)
	set(bounds[0], bound)
	floors := []int64{}
	for k := 1; k <= 2; k++ {
		outer, _ := simple(f, faces[0][k], "ADVANCED_FACE", 4)
		inside, _ := simple(f, faces[1][k], "ADVANCED_FACE", 4)
		outerPlane, _ := ref(outer.Args[2])
		innerPlane, _ := ref(inside.Args[2])
		ib, _ := listRefs(inside.Args[1], 1)
		b := f.Entities[ib[0]].Records[0]
		if blind {
			op, _ := planeFrame(f, outerPlane)
			ip, _ := planeFrame(f, innerPlane)
			if op.Origin().Z != ip.Origin().Z {
				inside.Args = append([]Value(nil), inside.Args...)
				sense, _ := boolean(inside.Args[3])
				value := "T"
				if sense {
					value = "F"
				}
				inside.Args[3] = Value{Kind: Enumeration, Text: value}
				b.Args = append([]Value(nil), b.Args...)
				b.Args[2] = inside.Args[3]
				set(faces[1][k], inside)
				set(ib[0], b)
				floors = append(floors, faces[1][k])
				continue
			}
		}
		if !blind {
			b.Type = "FACE_BOUND"
			set(ib[0], b)
		}
		lid, _ := ref(b.Args[1])
		loop, _ := simple(f, lid, "EDGE_LOOP", 2)
		uses, _ := listRefs(loop.Args[1], 1)
		u, _ := simple(f, uses[0], "ORIENTED_EDGE", 5)
		u.Args = append([]Value(nil), u.Args...)
		u.Args[4] = Value{Kind: Enumeration, Text: "F"}
		set(uses[0], u)
		outer.Args = append([]Value(nil), outer.Args...)
		outer.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), outer.Args[1].Items...), refv(ib[0]))}
		set(faces[0][k], outer)
		delete(f.Entities, faces[1][k])
		for id, ent := range f.Entities {
			if len(ent.Records) == 1 && ent.Records[0].Type == "PCURVE" {
				p := ent.Records[0]
				owner, _ := ref(p.Args[1])
				if owner == innerPlane {
					p.Args = append([]Value(nil), p.Args...)
					p.Args[1] = refv(outerPlane)
					set(id, p)
				}
			}
		}
	}
	shell, _ := simple(f, shells[0], "CLOSED_SHELL", 2)
	shell.Args = append([]Value(nil), shell.Args...)
	shell.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), shell.Args[1].Items...), refv(faces[1][0]))}
	for _, floor := range floors {
		shell.Args[1].Items = append(shell.Args[1].Items, refv(floor))
	}
	set(shells[0], shell)
	delete(f.Entities, shells[1])
	delete(f.Entities, bodyIDs[1])
	for id, ent := range f.Entities {
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			r := ent.Records[0]
			items := []Value{}
			for _, v := range r.Args[1].Items {
				if v.Kind != Reference || v.Ref != bodyIDs[1] {
					items = append(items, v)
				}
			}
			r.Args = append([]Value(nil), r.Args...)
			r.Args[1] = Value{Kind: Aggregate, Items: items}
			set(id, r)
		}
	}
	out := serializeWrappedProofFile(f)
	if _, e = readWrappedBore(bytes.NewReader(out), Limits{}, WrappedSurfaceOptions{}, blind); e != nil {
		return e
	}
	n, e := w.Write(out)
	if e == nil && n != len(out) {
		e = io.ErrShortWrite
	}
	return e
}
