package step

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/geom"
)

// AxialSection describes a cylinder (equal radii) or narrowing conical frustum.
type AxialSection struct{ BaseRadius, TopRadius, Height float64 }
type MixedAxialBody struct {
	Name     string
	Frame    geom.Frame
	Sections []AxialSection
}
type MixedAxialModel struct {
	Bodies     []MixedAxialBody
	LengthUnit float64
}

// WriteMixedAxialProfiles requires an annular shoulder between every section.
func WriteMixedAxialProfiles(w io.Writer, m MixedAxialModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing mixed axial model/writer")
	}
	f := &File{Entities: map[int64]Entity{}}
	ids := []int64{}
	for _, body := range m.Bodies {
		if len(body.Sections) < 3 || len(body.Sections) > 64 {
			return errors.New("step: mixed profile requires 3 to 64 sections")
		}
		z := 0.
		for i, s := range body.Sections {
			if !finiteNumber(s.BaseRadius) || !finiteNumber(s.TopRadius) || !finiteNumber(s.Height) || s.TopRadius <= 0 || s.BaseRadius < s.TopRadius || s.Height <= 0 || (i > 0 && body.Sections[i-1].TopRadius == s.BaseRadius) {
				return errors.New("step: invalid mixed axial section or zero shoulder")
			}
			frame, e := geom.NewFrame(body.Frame.ToWorld(geom.Vec3{Z: z}), body.Frame.XAxis(), body.Frame.ZAxis())
			if e != nil {
				return e
			}
			var raw bytes.Buffer
			if s.BaseRadius == s.TopRadius {
				e = WriteCylinders(&raw, CylinderModel{[]CylinderBody{{body.Name, frame, s.BaseRadius, s.Height}}, m.LengthUnit})
			} else {
				e = WriteCones(&raw, ConeModel{[]ConeBody{{body.Name, frame, s.BaseRadius, s.TopRadius, s.Height}}, m.LengthUnit})
			}
			if e != nil {
				return e
			}
			part, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
			if e != nil {
				return e
			}
			roots, _, e := colorBodyIDs(part)
			if e != nil || len(roots) != 1 {
				return errors.New("step: invalid staged mixed axial wall")
			}
			remap := map[int64]int64{}
			for _, id := range part.Order {
				remap[id] = int64(len(f.Order) + len(remap) + 1)
			}
			var value func(Value) Value
			value = func(v Value) Value {
				if v.Kind == Reference {
					v.Ref = remap[v.Ref]
				}
				if v.Items != nil {
					items := make([]Value, len(v.Items))
					for j, x := range v.Items {
						items[j] = value(x)
					}
					v.Items = items
				}
				return v
			}
			for _, id := range part.Order {
				ent := part.Entities[id]
				ent.ID = remap[id]
				for j, r := range ent.Records {
					for k, v := range r.Args {
						r.Args[k] = value(v)
					}
					ent.Records[j] = r
				}
				f.Entities[ent.ID] = ent
				f.Order = append(f.Order, ent.ID)
			}
			ids = append(ids, remap[roots[0]])
			z += s.Height
			if !finiteNumber(z) {
				return errors.New("step: mixed axial height overflow")
			}
		}
	}
	var e error
	aliases := map[int64]int64{}
	roots := []int64{}
	offset := 0
	for _, body := range m.Bodies {
		root := ids[offset]
		first := f.Entities[root]
		shellID := first.Records[0].Args[1].Ref
		previous, _ := listRefs(f.Entities[shellID].Records[0].Args[1], 3)
		offset++
		for i := 1; i < len(body.Sections); i++ {
			id := ids[offset]
			offset++
			nextBody := f.Entities[id]
			nextShell := nextBody.Records[0].Args[1].Ref
			nextFaces, _ := listRefs(f.Entities[nextShell].Records[0].Args[1], 3)
			a, b := f.Entities[previous[2]], f.Entities[nextFaces[1]]
			aliases[b.Records[0].Args[2].Ref] = a.Records[0].Args[2].Ref
			if body.Sections[i].BaseRadius > body.Sections[i-1].TopRadius {
				a.Records[0].Args[3] = Value{Kind: Enumeration, Text: "F"}
				a.Records[0].Args[1].Items = append(b.Records[0].Args[1].Items, a.Records[0].Args[1].Items...)
			} else {
				a.Records[0].Args[1].Items = append(a.Records[0].Args[1].Items, b.Records[0].Args[1].Items...)
			}
			f.Entities[previous[2]] = a
			shell := f.Entities[shellID]
			shell.Records[0].Args[1].Items = append(shell.Records[0].Args[1].Items, Value{Kind: Reference, Ref: nextFaces[0]}, Value{Kind: Reference, Ref: nextFaces[2]})
			f.Entities[shellID] = shell
			previous = nextFaces
		}
		roots = append(roots, root)
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
