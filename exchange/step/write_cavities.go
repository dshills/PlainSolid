package step

import (
	"bytes"
	"errors"
	"io"
)

// WriteCavities writes convex planar BREP_WITH_VOIDS bodies. Solid is derived
// from Outer and Voids and is ignored on input; source void orientation is kept.
func WriteCavities(w io.Writer, m CavityModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing cavity writer/model")
	}
	flat := Model{LengthUnit: m.LengthUnit}
	for _, body := range m.Bodies {
		b := body
		if e := validateCavity(&b); e != nil {
			return e
		}
		flat.Bodies = append(flat.Bodies, Body{b.Name, b.Outer})
		for _, v := range b.Voids {
			shell := v.Shell
			if !v.Reversed {
				shell = shell.Reversed()
			}
			flat.Bodies = append(flat.Bodies, Body{"", shell})
		}
	}
	var raw bytes.Buffer
	if e := WriteAdvanced(&raw, flat); e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		return e
	}
	ids, _, e := colorBodyIDs(f)
	if e != nil {
		return e
	}
	next := int64(1)
	mapping := map[int64]int64{}
	for id := range f.Entities {
		mapping[id] = id
		if id >= next {
			next = id + 1
		}
	}
	roots := []Value{}
	offset := 0
	for _, b := range m.Bodies {
		root := ids[offset]
		offset++
		outer := f.Entities[root].Records[0].Args[1]
		voids := []Value{}
		for _, v := range b.Voids {
			id := ids[offset]
			offset++
			shell := f.Entities[id].Records[0].Args[1]

			delete(f.Entities, id)
			sense := "T"
			if v.Reversed {
				sense = "F"
			}
			f.Entities[next] = Entity{ID: next, Records: []Record{{Type: "ORIENTED_CLOSED_SHELL", Args: []Value{{Kind: String}, {Kind: Derived}, shell, {Kind: Enumeration, Text: sense}}}}}
			mapping[next] = next
			f.Order = append(f.Order, next)
			voids = append(voids, Value{Kind: Reference, Ref: next})
			next++
		}
		f.Entities[root] = Entity{ID: root, Records: []Record{{Type: "BREP_WITH_VOIDS", Args: []Value{{Kind: String, Text: b.Name}, outer, {Kind: Aggregate, Items: voids}}}}}
		roots = append(roots, Value{Kind: Reference, Ref: root})
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		copied := map[int64]int64{}
		var copyEntity func(int64) int64
		var walk func(Value)
		walk = func(v Value) {
			if v.Kind == Reference {
				copyEntity(v.Ref)
			}
			for _, x := range v.Items {
				walk(x)
			}
		}
		copyEntity = func(id int64) int64 {
			if n, ok := copied[id]; ok {
				return n
			}
			ent := f.Entities[id]
			for _, r := range ent.Records {
				for _, v := range r.Args {
					walk(v)
				}
			}
			n := emit(assemblyEntity(ent, copied))
			copied[id] = n
			return n
		}
		result := []int64{}
		for _, root := range roots {
			result = append(result, copyEntity(root.Ref))
		}
		return result
	})
	if e != nil {
		return e
	}

	if _, e := ReadCavities(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
