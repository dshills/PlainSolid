package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
)

// WriteWrappedStationBore stages two certified lofts, reverses the inner walls
// and joins exactly matching planar caps into annuli. Complete readback and
// constructive material proof precede any write to the caller.
func WriteWrappedStationBore(w io.Writer, m WrappedStationBoreBody) error {
	return writeWrappedAxialBore(w, m, false)
}
func writeWrappedAxialBore(w io.Writer, m WrappedStationBoreBody, single bool) error {
	return writeWrappedProfileBore(w, m, single, false)
}

func writeWrappedProfileBore(w io.Writer, m WrappedStationBoreBody, single, rational bool, anisotropicMode ...bool) error {
	anisotropic := len(anisotropicMode) == 1 && anisotropicMode[0]
	if w == nil {
		return geom.ErrInvalid
	}
	if !rational && (len(m.Outer.Stations) < 2 || len(m.Inner.Stations) < 2 || (!single && len(m.Outer.Stations)+len(m.Inner.Stations) < 5) || (single && (len(m.Outer.Stations) != 2 || len(m.Inner.Stations) != 2 || !wrappedAxialBoreTapered(m)))) {
		return geom.ErrInvalid
	}
	sources := [2]*File{}
	roots := [2]int64{}
	for i, b := range []WrappedStationLoftBody{m.Outer, m.Inner} {
		b.LengthUnit = m.LengthUnit
		b.Name = m.Name
		b.CapNames = m.CapNames
		var raw bytes.Buffer
		var err error
		if anisotropic {
			err = WriteWrappedAnisotropicStationLoft(&raw, WrappedAnisotropicStationLoftBody{Name: b.Name, Walls: b.Walls, CapNames: b.CapNames, LengthUnit: b.LengthUnit})
		} else if rational {
			err = WriteWrappedRationalStationLoft(&raw, WrappedRationalStationLoftBody{Name: b.Name, Walls: b.Walls, CapNames: b.CapNames, LengthUnit: b.LengthUnit})
		} else {
			err = writeStationBoreLoft(&raw, b)
		}
		if err != nil {
			return err
		}
		f, e := Parse(&raw, Limits{})
		if e != nil {
			return e
		}
		sources[i] = f
		for _, id := range f.Order {
			if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
				if roots[i] != 0 {
					return geom.ErrInvalid
				}
				roots[i] = id
			}
		}
	}
	var staged bytes.Buffer
	e := writeStandalone(&staged, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(put func(string) int64) []int64 {
		ids := []int64{}
		for i, f := range sources {
			ids = append(ids, copySplineGeometry(put, f, []int64{roots[i]})...)
		}
		return ids
	})
	if e != nil {
		return e
	}
	f, e := Parse(&staged, Limits{})
	if e != nil {
		return e
	}
	bodies := []int64{}
	for _, id := range f.Order {
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			bodies = append(bodies, id)
		}
	}
	if len(bodies) != 2 {
		return geom.ErrInvalid
	}
	shells := [2]int64{}
	walls := [2][]int64{}
	caps := [2][]int64{}
	for i, id := range bodies {
		body, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
		shells[i], _ = ref(body.Args[1])
		faces, e := bodyFaceIDs(f, id)
		if e != nil {
			return e
		}
		for _, fid := range faces {
			face, _ := simple(f, fid, "ADVANCED_FACE", 4)
			sid, _ := ref(face.Args[2])
			if _, e := simple(f, sid, "PLANE", 2); e == nil {
				caps[i] = append(caps[i], fid)
			} else {
				walls[i] = append(walls[i], fid)
			}
		}
		if len(caps[i]) != 2 || len(walls[i]) < 1 {
			return geom.ErrInvalid
		}
	}
	for _, fid := range walls[1] {
		face, _ := simple(f, fid, "ADVANCED_FACE", 4)
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil || len(bounds) != 1 {
			return geom.ErrInvalid
		}
		bound := f.Entities[bounds[0]].Records[0]
		face.Args = append([]Value(nil), face.Args...)
		bound.Args = append([]Value(nil), bound.Args...)
		face.Args[3] = Value{Kind: Enumeration, Text: "F"}
		bound.Args[2] = face.Args[3]
		ovalSet(f, fid, face)
		ovalSet(f, bounds[0], bound)
	}
	for k := 0; k < 2; k++ {
		outer, _ := simple(f, caps[0][k], "ADVANCED_FACE", 4)
		inner, _ := simple(f, caps[1][k], "ADVANCED_FACE", 4)
		op, _ := ref(outer.Args[2])
		ip, _ := ref(inner.Args[2])
		of, e := planeFrame(f, op)
		if e != nil {
			return e
		}
		inf, e := planeFrame(f, ip)
		if e != nil {
			return e
		}
		if of != inf {
			return fmt.Errorf("step: station bore cap frames differ")
		}
		ib, e := listRefs(inner.Args[1], 1)
		if e != nil || len(ib) != 1 {
			return geom.ErrInvalid
		}
		bound := f.Entities[ib[0]].Records[0]
		bound.Type = "FACE_BOUND"
		ovalSet(f, ib[0], bound)
		lid, _ := ref(bound.Args[1])
		loop, _ := simple(f, lid, "EDGE_LOOP", 2)
		uses, e := listRefs(loop.Args[1], 1)
		if e != nil || len(uses) != 1 {
			return geom.ErrInvalid
		}
		use, _ := simple(f, uses[0], "ORIENTED_EDGE", 5)
		use.Args = append([]Value(nil), use.Args...)
		use.Args[4] = Value{Kind: Enumeration, Text: "F"}
		ovalSet(f, uses[0], use)
		outer.Args = append([]Value(nil), outer.Args...)
		outer.Args[1].Items = append(append([]Value(nil), outer.Args[1].Items...), Value{Kind: Reference, Ref: ib[0]})
		ovalSet(f, caps[0][k], outer)
		for id, en := range f.Entities {
			if len(en.Records) == 1 && en.Records[0].Type == "PCURVE" {
				p := en.Records[0]
				owner, _ := ref(p.Args[1])
				if owner == ip {
					p.Args = append([]Value(nil), p.Args...)
					p.Args[1] = Value{Kind: Reference, Ref: op}
					ovalSet(f, id, p)
				}
			}
		}
	}
	kept := append(append(append([]int64(nil), walls[0]...), walls[1]...), caps[0]...)
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(put func(string) int64) []int64 {
		ids := copySplineGeometry(put, f, kept)
		sh := put("CLOSED_SHELL(''," + references(ids) + ")")
		return []int64{put(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(m.Name), sh))}
	})
	if e != nil {
		return e
	}
	if _, e = readWrappedProfileBore(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{}, single, rational, anisotropic); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
