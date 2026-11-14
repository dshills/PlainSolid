package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// TripleCrossedBoreBody retains three equal-radius concurrent cardinal
// through bores on rectangular stock. Crossing is in stock coordinates.
type TripleCrossedBoreBody struct {
	Name                 string
	Frame                geom.Frame
	Dimensions, Crossing geom.Vec3
	Radius               float64
}
type TripleCrossedBoreModel struct {
	Body       TripleCrossedBoreBody
	LengthUnit float64
}

func WriteTripleCrossedBore(w io.Writer, m TripleCrossedBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if e := writeTripleCrossedBore(&out, m); e != nil {
		return e
	}
	if _, e := ReadTripleCrossedBore(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &out)
	return e
}
func writeTripleCrossedBore(w io.Writer, m TripleCrossedBoreModel) error {
	p := m.Body
	r, e := trimmed.BoxTripleCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, 1e-9)
	if e != nil {
		return e
	}
	return writeNativeCrossedBoreTopology(w, m.LengthUnit, p.Name, r.Solid)
}

// ReadTripleCrossedBore verifies the entire graph against exact regeneration.
func ReadTripleCrossedBore(r io.Reader, limits Limits) (TripleCrossedBoreModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, limits)
	if e != nil || len(ids) != 1 {
		return TripleCrossedBoreModel{}, circularSourceFailure()
	}
	p, e := recoverTripleCrossedBore(f, ids[0])
	if e != nil {
		return TripleCrossedBoreModel{}, e
	}
	m := TripleCrossedBoreModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if e = writeTripleCrossedBore(&out, m); e != nil {
		return TripleCrossedBoreModel{}, e
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &out); e != nil {
		return TripleCrossedBoreModel{}, e
	}
	return m, nil
}
func recoverTripleCrossedBore(f *File, id int64) (TripleCrossedBoreBody, error) {
	fail := func() (TripleCrossedBoreBody, error) { return TripleCrossedBoreBody{}, circularSourceFailure() }
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], 54)
	if e != nil || len(faces) != 54 {
		return fail()
	}
	lower, e := readCircularSourceFace(f, faces[0])
	if e != nil || lower.kind != "PLANE" {
		return fail()
	}
	var supports [3]circularSourceFace
	for i, fi := range []int{6, 22, 38} {
		supports[i], e = readCircularSourceFace(f, faces[fi])
		if e != nil || supports[i].kind != "CYLINDER" || supports[i].sense {
			return fail()
		}
	}
	z, x, y := supports[0], supports[1], supports[2]
	if z.radius != x.radius || z.radius != y.radius || z.frame.Origin() != x.frame.Origin() || z.frame.Origin() != y.frame.Origin() {
		return fail()
	}
	frame, e := geom.NewFrame(lower.frame.Origin(), x.frame.ZAxis(), z.frame.ZAxis())
	if e != nil || frame.YAxis() != y.frame.ZAxis() {
		return fail()
	}
	var vertices []geom.Vec3
	for _, id := range faces[:6] {
		face, e := readCircularSourceFace(f, id)
		if e != nil || face.kind != "PLANE" || !face.sense || len(face.loops) != 2 || len(face.loops[0]) != 4 || len(face.loops[1]) != 8 {
			return fail()
		}
		for _, use := range face.loops[0] {
			start, _, _, e := analyticEdgeGeometry(f, use.id, true)
			if e != nil {
				return fail()
			}
			p, e := analyticVertex(f, start)
			if e != nil {
				return fail()
			}
			vertices = append(vertices, frame.ToLocal(p))
		}
	}
	bounds, e := geom.Bounds(vertices)
	if e != nil || bounds.Min != (geom.Vec3{}) {
		return fail()
	}
	return TripleCrossedBoreBody{Name: name, Frame: frame, Dimensions: bounds.Max, Crossing: frame.ToLocal(z.frame.Origin()), Radius: z.radius}, nil
}
