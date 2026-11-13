package step

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// TubeBody specifies a full concentric annular cylinder with two planar caps.
type TubeBody struct {
	Name                             string
	Frame                            geom.Frame
	OuterRadius, InnerRadius, Height float64
}
type TubeModel struct {
	Bodies     []TubeBody
	LengthUnit float64
}

func WriteTubes(w io.Writer, m TubeModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing tube writer/model")
	}
	flat := CylinderModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		if b.InnerRadius <= 0 {
			return errors.New("step: tube inner radius must be positive")
		}
		if _, e := parametric.Tube(b.Frame, b.OuterRadius, b.Height, b.OuterRadius-b.InnerRadius); e != nil {
			return e
		}
		flat.Bodies = append(flat.Bodies, CylinderBody{b.Name, b.Frame, b.OuterRadius, b.Height}, CylinderBody{"", b.Frame, b.InnerRadius, b.Height})
	}
	var raw bytes.Buffer
	if e := WriteCylinders(&raw, flat); e != nil {
		return e
	}
	return writeAnnularWalls(w, raw.Bytes(), m.LengthUnit, len(m.Bodies))
}

// writeAnnularWalls merges two validated full walls per body into shared annular
// caps, preserving each wall's exact rim and p-curve definitions.
func writeAnnularWalls(w io.Writer, raw []byte, lengthUnit float64, bodyCount int) error {
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		return e
	}
	ids, _, e := colorBodyIDs(f)
	if e != nil {
		return e
	}
	aliases := map[int64]int64{}
	roots := []int64{}
	for i := 0; i < bodyCount; i++ {
		outer, inner := ids[2*i], ids[2*i+1]
		ob := f.Entities[outer]
		ib := f.Entities[inner]
		osh := ob.Records[0].Args[1].Ref
		ish := ib.Records[0].Args[1].Ref
		of, _ := listRefs(f.Entities[osh].Records[0].Args[1], 3)
		inf, _ := listRefs(f.Entities[ish].Records[0].Args[1], 3)
		// Staged cylinder writer emits side,bottom,top. Merge each pair of caps and
		// alias the inner cap support used by its exact pcurves to the outer support.
		for k := 1; k <= 2; k++ {
			a, b := f.Entities[of[k]], f.Entities[inf[k]]
			aliases[b.Records[0].Args[2].Ref] = a.Records[0].Args[2].Ref
			bounds, _ := listRefs(b.Records[0].Args[1], 1)
			bound := f.Entities[bounds[0]]
			bound.Records[0].Args[2] = Value{Kind: Enumeration, Text: "F"}
			f.Entities[bounds[0]] = bound
			a.Records[0].Args[1].Items = append(a.Records[0].Args[1].Items, Value{Kind: Reference, Ref: bounds[0]})
			f.Entities[of[k]] = a
		}
		side := f.Entities[inf[0]]
		side.Records[0].Args[3] = Value{Kind: Enumeration, Text: "F"}
		bounds, _ := listRefs(side.Records[0].Args[1], 1)
		bound := f.Entities[bounds[0]]
		bound.Records[0].Args[2] = Value{Kind: Enumeration, Text: "F"}
		f.Entities[bounds[0]] = bound
		f.Entities[inf[0]] = side
		shell := f.Entities[osh]
		shell.Records[0].Args[1].Items = append(shell.Records[0].Args[1].Items, Value{Kind: Reference, Ref: inf[0]})
		f.Entities[osh] = shell
		roots = append(roots, outer)
	}
	var output bytes.Buffer
	e = writeStandalone(&output, lengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
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
			if alias, ok := aliases[id]; ok {
				n := copyEntity(alias)
				copied[id] = n
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
		for _, id := range roots {
			result = append(result, copyEntity(id))
		}
		return result
	})
	if e != nil {
		return e
	}
	if _, e = ReadAnalyticBodies(bytes.NewReader(output.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(output.Bytes())
	if e == nil && n != output.Len() {
		return io.ErrShortWrite
	}
	return e
}
