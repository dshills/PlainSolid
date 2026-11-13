package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
)

// WriteCurvedOvalOpenShell shares the four hole/disk edges, retaining a separate
// exact owner pcurve for each support. Cached source IDs and curves are ignored.
func WriteCurvedOvalOpenShell(w io.Writer, m CurvedOvalOpenShell, o CurvedOvalOptions) error {
	if w == nil || !validLengthUnit(m.LengthUnit) {
		return geom.ErrInvalid
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return e
	}
	if e = certifyCurvedOvalShell(m, o); e != nil {
		return e
	}
	files := []*File{}
	roots := []int64{}
	for _, face := range []CurvedOvalFace{m.Annulus, m.Disk} {
		face.LengthUnit = m.LengthUnit
		var b bytes.Buffer
		if e = WriteCurvedOvalFace(&b, face, o); e != nil {
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
	if e != nil || len(a) != 8 || len(b) != 4 {
		return geom.ErrInvalid
	}
	for i := 0; i < 4; i++ {
		hole, disk := a[4+i], b[3-i]
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
		if e = reverseOvalCurve(f, curves[0]); e != nil {
			return e
		}
		hsc.Args[2].Items = append(append([]Value(nil), hsc.Args[2].Items...), Value{Kind: Reference, Ref: pcs[0]})
		ovalSet(f, hole.curve, hsc)
		use, e := ovalRecord(f, disk.use, "ORIENTED_EDGE", 5)
		if e != nil {
			return e
		}
		use.Args[3] = Value{Kind: Reference, Ref: hole.edge}
		use.Args[4] = Value{Kind: Enumeration, Text: "F"}
		ovalSet(f, disk.use, use)
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := copySplineGeometry(emit, f, faces)
		shell := emit("OPEN_SHELL(''," + references(ids) + ")")
		return []int64{emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell))}
	})
	if e != nil {
		return e
	}
	if _, e = ReadCurvedOvalOpenShell(bytes.NewReader(out.Bytes()), Limits{}, o); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
