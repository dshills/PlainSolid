package step

import (
	"bytes"
	"fmt"
	"io"
)

// WriteWrappedShoulder preserves exact wall supports and merges only the two
// coincident endpoint caps, retaining the larger disk's outward orientation.
// The staged graph is independently re-proved before any output is written.
func WriteWrappedShoulder(w io.Writer, m WrappedShoulderBody) error {
	if w == nil {
		return fmt.Errorf("step: nil shoulder writer")
	}
	var raw bytes.Buffer
	pair := WrappedCylindricalModel{LengthUnit: m.LengthUnit, Bodies: []WrappedCylindricalBody{{Name: m.Name, Face: m.Larger, CapNames: m.LargerCapNames}, {Name: m.Name, Face: m.Smaller, CapNames: [2]string{m.SmallerEndName, m.SmallerEndName}}}}
	if e := WriteWrappedCylindricalBodies(&raw, pair); e != nil {
		return e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return e
	}
	ids := []int64{}
	for _, id := range f.Order {
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) != 2 {
		return fmt.Errorf("step: staged shoulder bodies")
	}
	shells := [2]int64{}
	faces := [2][]int64{}
	for i, id := range ids {
		body, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
		shells[i], _ = ref(body.Args[1])
		shell, _ := simple(f, shells[i], "CLOSED_SHELL", 2)
		faces[i], e = listRefs(shell.Args[1], 3)
		if e != nil || len(faces[i]) != 3 {
			return fmt.Errorf("step: staged shoulder faces")
		}
	}
	sharedBig, sharedSmall := 0, 0
	for i := 1; i <= 2; i++ {
		a, _ := simple(f, faces[0][i], "ADVANCED_FACE", 4)
		as, _ := ref(a.Args[2])
		ap, _ := planeFrame(f, as)
		for j := 1; j <= 2; j++ {
			b, _ := simple(f, faces[1][j], "ADVANCED_FACE", 4)
			bs, _ := ref(b.Args[2])
			bp, _ := planeFrame(f, bs)
			if ap.Origin().Z == bp.Origin().Z {
				if sharedBig != 0 {
					return fmt.Errorf("step: shoulder requires one endpoint contact")
				}
				sharedBig, sharedSmall = i, j
			}
		}
	}
	if sharedBig == 0 || sharedBig == sharedSmall {
		return fmt.Errorf("step: shoulder intervals must touch without overlap")
	}
	set := func(id int64, r Record) { f.Entities[id] = Entity{ID: id, Records: []Record{r}} }
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	big, _ := simple(f, faces[0][sharedBig], "ADVANCED_FACE", 4)
	small, _ := simple(f, faces[1][sharedSmall], "ADVANCED_FACE", 4)
	bigPlane, _ := ref(big.Args[2])
	smallPlane, _ := ref(small.Args[2])
	bounds, _ := listRefs(small.Args[1], 1)
	bound := f.Entities[bounds[0]].Records[0]
	lid, _ := ref(bound.Args[1])
	loop, _ := simple(f, lid, "EDGE_LOOP", 2)
	uses, _ := listRefs(loop.Args[1], 1)
	use, _ := simple(f, uses[0], "ORIENTED_EDGE", 5)
	use.Args = append([]Value(nil), use.Args...)
	use.Args[4] = Value{Kind: Enumeration, Text: "F"}
	set(uses[0], use)
	bound.Args = append([]Value(nil), bound.Args...)
	bound.Args[2] = big.Args[3]
	set(bounds[0], bound)
	big.Args = append([]Value(nil), big.Args...)
	big.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), big.Args[1].Items...), refv(bounds[0]))}
	set(faces[0][sharedBig], big)
	delete(f.Entities, faces[1][sharedSmall])
	for id, ent := range f.Entities {
		if len(ent.Records) == 1 && ent.Records[0].Type == "PCURVE" {
			p := ent.Records[0]
			owner, _ := ref(p.Args[1])
			if owner == smallPlane {
				p.Args = append([]Value(nil), p.Args...)
				p.Args[1] = refv(bigPlane)
				set(id, p)
			}
		}
	}
	shell, _ := simple(f, shells[0], "CLOSED_SHELL", 2)
	shell.Args = append([]Value(nil), shell.Args...)
	shell.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), shell.Args[1].Items...), refv(faces[1][0]), refv(faces[1][3-sharedSmall]))}
	set(shells[0], shell)
	delete(f.Entities, shells[1])
	delete(f.Entities, ids[1])
	for id, ent := range f.Entities {
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			r := ent.Records[0]
			items := []Value{}
			for _, v := range r.Args[1].Items {
				if v.Kind != Reference || v.Ref != ids[1] {
					items = append(items, v)
				}
			}
			r.Args = append([]Value(nil), r.Args...)
			r.Args[1] = Value{Kind: Aggregate, Items: items}
			set(id, r)
		}
	}
	out := serializeWrappedProofFile(f)
	if _, e = ReadWrappedShoulder(bytes.NewReader(out), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(out)
	if e == nil && n != len(out) {
		e = io.ErrShortWrite
	}
	return e
}
