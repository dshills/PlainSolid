package step

import (
	"bytes"
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CrossedObliqueBoreBody retains a cardinal Z bore and a concurrent
// equal-radius 3–4–5 bore crossing opposite X stock caps.
type CrossedObliqueBoreBody struct {
	Name                 string
	Frame                geom.Frame
	Dimensions, Crossing geom.Vec3
	Radius               float64
	// AxisRatio zero preserves the legacy 4:3 file representation.
	// The additional supported ratio is (3,4).
	AxisRatio geom.Vec2
}

type CrossedObliqueBoreModel struct {
	Body       CrossedObliqueBoreBody
	LengthUnit float64
}

func WriteCrossedObliqueBore(w io.Writer, m CrossedObliqueBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var buf bytes.Buffer
	if err := writeCrossedObliqueBore(&buf, m); err != nil {
		return err
	}
	if _, err := ReadCrossedObliqueBore(bytes.NewReader(buf.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &buf)
	return err
}

func writeCrossedObliqueBore(w io.Writer, m CrossedObliqueBoreModel) error {
	p := m.Body
	var r trimmed.BoxCrossedBoresResult
	var err error
	switch p.AxisRatio {
	case geom.Vec2{}:
		r, err = trimmed.BoxCrossedBores345(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, 1e-9)
	case geom.Vec2{X: 3, Y: 4}:
		r, err = trimmed.BoxCrossedObliqueBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.AxisRatio, p.Radius, 1e-9)
	default:
		return geom.ErrDomain
	}
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopologyConics(w, m.LengthUnit, p.Name, r.Solid, nativeLongEdgeLimits{22, 96, 64, 2, 4}, false, 0, nil, false, true)
}

func ReadCrossedObliqueBore(r io.Reader, l Limits) (CrossedObliqueBoreModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil || len(ids) != 1 {
		return CrossedObliqueBoreModel{}, circularSourceFailure()
	}
	body, err := recoverCrossedObliqueBore(f, ids[0])
	if err != nil {
		return CrossedObliqueBoreModel{}, err
	}
	m := CrossedObliqueBoreModel{Body: body, LengthUnit: unit}
	var buf bytes.Buffer
	if err := writeCrossedObliqueBore(&buf, m); err != nil {
		return CrossedObliqueBoreModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &buf); err != nil {
		return CrossedObliqueBoreModel{}, err
	}
	return m, nil
}

func recoverCrossedObliqueBore(f *File, id int64) (CrossedObliqueBoreBody, error) {
	fail := func() (CrossedObliqueBoreBody, error) { return CrossedObliqueBoreBody{}, circularSourceFailure() }
	body, err := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return fail()
	}
	name, err := textValue(body.Args[0])
	if err != nil {
		return fail()
	}
	sid, err := ref(body.Args[1])
	if err != nil {
		return fail()
	}
	shell, err := simple(f, sid, "CLOSED_SHELL", 2)
	if err != nil {
		return fail()
	}
	ids, err := listRefs(shell.Args[1], 22)
	if err != nil || len(ids) != 22 {
		return fail()
	}
	lo, err := readCircularSourceFaceBounded(f, ids[0], 2, 4)
	if err != nil || lo.kind != "PLANE" {
		return fail()
	}
	right, err := readCircularSourceFaceBounded(f, ids[3], 2, 4)
	if err != nil || right.kind != "PLANE" {
		return fail()
	}
	z, err := readCircularSourceFaceBounded(f, ids[6], 1, 4)
	if err != nil || z.kind != "CYLINDER" || z.sense {
		return fail()
	}
	b, err := readCircularSourceFaceBounded(f, ids[14], 1, 4)
	if err != nil || b.kind != "CYLINDER" || b.sense || b.radius != z.radius || b.frame.Origin() != z.frame.Origin() {
		return fail()
	}
	frame, err := geom.NewFrame(lo.frame.Origin(), right.frame.ZAxis(), z.frame.ZAxis())
	if err != nil {
		return fail()
	}
	basis := frame.XAxis().Scale(.8).Add(frame.ZAxis().Scale(.6))
	ratio := geom.Vec2{}
	if b.frame.ZAxis().Distance(basis) > 8*0x1p-52 {
		basis = frame.XAxis().Scale(.6).Add(frame.ZAxis().Scale(.8))
		if b.frame.ZAxis().Distance(basis) > 8*0x1p-52 {
			return fail()
		}
		ratio = geom.Vec2{X: 3, Y: 4}
	}
	var vertices []geom.Vec3
	for _, faceID := range ids[:6] {
		face, err := readCircularSourceFaceBounded(f, faceID, 2, 4)
		if err != nil || face.kind != "PLANE" || !face.sense || len(face.loops[0]) != 4 {
			return fail()
		}
		for _, use := range face.loops[0] {
			start, _, _, err := analyticEdgeGeometry(f, use.id, true)
			if err != nil {
				return fail()
			}
			point, err := analyticVertex(f, start)
			if err != nil {
				return fail()
			}
			vertices = append(vertices, frame.ToLocal(point))
		}
	}
	bounds, err := geom.Bounds(vertices)
	if err != nil || bounds.Min != (geom.Vec3{}) {
		return fail()
	}
	return CrossedObliqueBoreBody{Name: name, Frame: frame, Dimensions: bounds.Max, Crossing: frame.ToLocal(z.frame.Origin()), Radius: z.radius, AxisRatio: ratio}, nil
}
