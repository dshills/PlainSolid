package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
	"unicode/utf8"
)

// WriteSplineLensBody proves material orientation and emits four shared exact
// ellipse rim edges. Cached source IDs and Solid are ignored.
func WriteSplineLensBody(w io.Writer, m SplineLensBody, o CurvedOvalOptions) error {
	if w == nil || !validLengthUnit(m.LengthUnit) || !utf8.ValidString(m.Name) {
		return geom.ErrInvalid
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return e
	}
	if _, e = certifySplineLens(m, o); e != nil {
		return e
	}
	files := []*File{}
	roots := []int64{}
	q := QuadraticOval{Center: m.Center, Radii: m.Radii, QuarterWeight: .5}
	for _, face := range []CurvedOvalFace{{Name: m.FaceNames[0], Geometry: m.Upper, Outer: q, ellipseShear: m.Shear}, {Name: m.FaceNames[1], Geometry: m.Lower, Outer: q, ellipseShear: m.Shear}} {
		face.LengthUnit = m.LengthUnit
		var b bytes.Buffer
		if e = writeCurvedOvalFace(&b, face, o, true); e != nil {
			return e
		}
		f, e := Parse(&b, Limits{})
		if e != nil {
			return e
		}
		var root int64
		for _, id := range f.Order {
			if _, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
				root = id
			}
		}
		files = append(files, f)
		roots = append(roots, root)
	}
	var initial bytes.Buffer
	e = writeStandalone(&initial, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		faces := []int64{}
		for i, f := range files {
			faces = append(faces, copySplineGeometry(emit, f, []int64{roots[i]})[0])
		}
		shell := emit("OPEN_SHELL(''," + references(faces) + ")")
		return []int64{emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell))}
	})
	if e != nil {
		return e
	}
	f, e := Parse(&initial, Limits{})
	if e != nil {
		return e
	}
	faces := []int64{}
	for _, id := range f.Order {
		if _, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
			faces = append(faces, id)
		}
	}
	if len(faces) != 2 {
		return geom.ErrInvalid
	}
	a, e := curvedOvalFaceUses(f, faces[0])
	if e != nil {
		return e
	}
	b, e := curvedOvalFaceUses(f, faces[1])
	if e != nil || len(a) != 4 || len(b) != 4 {
		return geom.ErrInvalid
	}
	for i := 0; i < 4; i++ {
		hole, disk := a[i], b[i]
		hsc, e := ovalRecord(f, hole.curve, "SURFACE_CURVE", 4)
		if e != nil {
			return e
		}
		dsc, e := simple(f, disk.curve, "SURFACE_CURVE", 4)
		if e != nil {
			return e
		}
		pcs, e := listRefs(dsc.Args[2], 1)
		if e != nil || len(pcs) != 1 {
			return geom.ErrInvalid
		}
		pc, e := simple(f, pcs[0], "PCURVE", 3)
		if e != nil {
			return e
		}
		rid, _ := ref(pc.Args[2])
		rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return e
		}
		curves, e := listRefs(rep.Args[1], 1)
		if e != nil || len(curves) != 1 {
			return geom.ErrInvalid
		}

		hsc.Args[2].Items = append(append([]Value(nil), hsc.Args[2].Items...), Value{Kind: Reference, Ref: pcs[0]})
		ovalSet(f, hole.curve, hsc)
		use, e := ovalRecord(f, disk.use, "ORIENTED_EDGE", 5)
		if e != nil {
			return e
		}
		use.Args[3] = Value{Kind: Reference, Ref: hole.edge}
		use.Args[4] = Value{Kind: Enumeration, Text: "T"}
		ovalSet(f, disk.use, use)
	}
	fr, e := ovalRecord(f, faces[1], "ADVANCED_FACE", 4)
	if e != nil {
		return e
	}
	fr.Args[3] = Value{Kind: Enumeration, Text: "F"}
	ovalSet(f, faces[1], fr)
	for _, u := range b {
		bound, e := ovalRecord(f, u.bound, "FACE_OUTER_BOUND", 3)
		if e != nil {
			return e
		}
		bound.Args[2] = Value{Kind: Enumeration, Text: "F"}
		ovalSet(f, u.bound, bound)
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := copySplineGeometry(emit, f, faces)
		shell := emit("CLOSED_SHELL(''," + references(ids) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(m.Name), shell))}
	})
	if e != nil {
		return e
	}
	if _, e = ReadSplineLensBody(bytes.NewReader(out.Bytes()), Limits{}, o); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
