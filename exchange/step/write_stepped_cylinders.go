package step

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SteppedCylinderBody specifies a two coaxial cylinder sections joined by an annular shoulder.
type SteppedCylinderBody struct {
	Name                                               string
	Frame                                              geom.Frame
	LowerRadius, UpperRadius, LowerHeight, UpperHeight float64
}
type SteppedCylinderModel struct {
	Bodies     []SteppedCylinderBody
	LengthUnit float64
}

func WriteSteppedCylinders(w io.Writer, m SteppedCylinderModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing stepped cylinder writer/model")
	}
	flat := CylinderModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		if b.UpperRadius <= 0 || b.UpperRadius >= b.LowerRadius || b.LowerHeight <= 0 || b.UpperHeight <= 0 {
			return errors.New("step: invalid stepped cylinder dimensions")
		}
		if _, e := parametric.Cylinder(b.Frame, b.LowerRadius, b.LowerHeight); e != nil {
			return e
		}
		upper, e := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{Z: b.LowerHeight}), b.Frame.XAxis(), b.Frame.ZAxis())
		if e != nil {
			return e
		}
		flat.Bodies = append(flat.Bodies, CylinderBody{b.Name, b.Frame, b.LowerRadius, b.LowerHeight}, CylinderBody{"", upper, b.UpperRadius, b.UpperHeight})
	}

	var raw bytes.Buffer
	if e := WriteCylinders(&raw, flat); e != nil {
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
	aliases := map[int64]int64{}
	roots := []int64{}
	for i := range m.Bodies {
		outer, inner := ids[2*i], ids[2*i+1]
		ob := f.Entities[outer]
		ib := f.Entities[inner]
		osh := ob.Records[0].Args[1].Ref
		ish := ib.Records[0].Args[1].Ref
		of, _ := listRefs(f.Entities[osh].Records[0].Args[1], 3)
		inf, _ := listRefs(f.Entities[ish].Records[0].Args[1], 3)
		// Lower top and upper bottom meet on one annular shoulder.
		a, b := f.Entities[of[2]], f.Entities[inf[1]]
		aliases[b.Records[0].Args[2].Ref] = a.Records[0].Args[2].Ref
		a.Records[0].Args[1].Items = append(a.Records[0].Args[1].Items, b.Records[0].Args[1].Items...)
		f.Entities[of[2]] = a
		shell := f.Entities[osh]
		shell.Records[0].Args[1].Items = append(shell.Records[0].Args[1].Items, Value{Kind: Reference, Ref: inf[0]}, Value{Kind: Reference, Ref: inf[2]})
		f.Entities[osh] = shell

		roots = append(roots, outer)
	}
	var output bytes.Buffer
	e = writeStandalone(&output, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
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
