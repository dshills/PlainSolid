package step

import (
	"bytes"
	"errors"
	"io"
)

type planarCavitySTEPRecipe struct {
	name     string
	reversed []bool
}

// WriteCavities writes convex planar BREP_WITH_VOIDS bodies. Solid is derived
// from Outer and Voids and is ignored on input; source void orientation is kept.
func WriteCavities(w io.Writer, m CavityModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing cavity writer/model")
	}
	flat := Model{LengthUnit: m.LengthUnit}
	recipes := make([]planarCavitySTEPRecipe, len(m.Bodies))
	for i, body := range m.Bodies {
		b := body
		if e := validateCavity(&b); e != nil {
			return e
		}
		recipes[i].name = b.Name
		flat.Bodies = append(flat.Bodies, Body{b.Name, b.Outer})
		for _, v := range b.Voids {
			shell := v.Shell
			if !v.Reversed {
				shell = shell.Reversed()
			}
			flat.Bodies = append(flat.Bodies, Body{"", shell})
			recipes[i].reversed = append(recipes[i].reversed, v.Reversed)
		}
	}
	var out bytes.Buffer
	if e := writePlanarCavitySTEPGraph(&out, flat, recipes, Limits{}, false); e != nil {
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

func writePlanarCavitySTEPGraph(w io.Writer, flat Model, recipes []planarCavitySTEPRecipe, limits Limits, exact bool) error {
	l, e := limits.defaults()
	if e != nil {
		return e
	}
	raw := vertexContactSTEPBuffer{max: l.MaxBytes}
	if e := writePlanarModel(&raw, flat, true, exact); e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), l)
	if e != nil {
		return e
	}
	ids, _, e := colorBodyIDs(f)
	if e != nil {
		return e
	}
	next := int64(1)
	for id := range f.Entities {
		if id >= next {
			next = id + 1
		}
	}
	roots := []Value{}
	offset := 0
	for _, b := range recipes {
		if offset >= len(ids) || len(b.reversed) >= len(ids)-offset {
			return errors.New("step: missing composed planar shell records")
		}
		root := ids[offset]
		offset++
		roots = append(roots, Value{Kind: Reference, Ref: root})
		if len(b.reversed) == 0 {
			continue
		}
		outer := f.Entities[root].Records[0].Args[1]
		voids := []Value{}
		for _, reversed := range b.reversed {
			id := ids[offset]
			offset++
			shell := f.Entities[id].Records[0].Args[1]

			delete(f.Entities, id)
			sense := "T"
			if reversed {
				sense = "F"
			}
			f.Entities[next] = Entity{ID: next, Records: []Record{{Type: "ORIENTED_CLOSED_SHELL", Args: []Value{{Kind: String}, {Kind: Derived}, shell, {Kind: Enumeration, Text: sense}}}}}
			f.Order = append(f.Order, next)
			voids = append(voids, Value{Kind: Reference, Ref: next})
			next++
		}
		f.Entities[root] = Entity{ID: root, Records: []Record{{Type: "BREP_WITH_VOIDS", Args: []Value{{Kind: String, Text: b.name}, outer, {Kind: Aggregate, Items: voids}}}}}
	}
	if offset != len(ids) {
		return errors.New("step: unclaimed composed planar shell records")
	}
	emitUnit := emitLengthUnit
	if exact {
		emitUnit = emitExactPlanarLengthUnit
	}
	return writeStandaloneWithUnit(w, flat.LengthUnit, 0, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
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
	}, emitUnit)
}
