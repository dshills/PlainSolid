package step

import (
	"bytes"
	"context"
	"io"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// QuarterAnnularExtrusionBody is a quadrant of a tube, from frame X toward
// frame Y, with two cylindrical walls and four planar faces.
type QuarterAnnularExtrusionBody struct {
	Name                  string
	Frame                 geom.Frame
	Inner, Radius, Height float64
}

type QuarterAnnularExtrusionModel struct {
	Body       QuarterAnnularExtrusionBody
	LengthUnit float64
}

func writeQuarterAnnularExtrusion(w io.Writer, m QuarterAnnularExtrusionModel) error {
	p := m.Body
	if !finiteOblique(p.Inner) || p.Inner <= 0 || p.Inner >= p.Radius {
		return geom.ErrDomain
	}
	q := QuarterSectorExtrusionBody{Frame: p.Frame, Radius: p.Radius, Height: p.Height}
	outer, err := q.Segments()
	if err != nil {
		return err
	}
	a, b := geom.Vec2{X: p.Inner}, geom.Vec2{Y: p.Inner}
	segments := []trimmed.ExtrusionSegment{
		{Start: a, End: outer[1].Start}, outer[1], {Start: outer[1].End, End: b},
		{Start: b, End: a, Radius: p.Inner, StartAngle: math.Pi / 2, Sweep: -math.Pi / 2},
	}
	s, err := trimmed.ExtrudeArcLoop(context.Background(), p.Frame, segments, p.Height, max(1, p.Height, p.Radius)*1e-10)
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, p.Name, s.Solid, nativeLongEdgeLimits{6, 12, 8, 1, 4})
}

// WriteQuarterAnnularExtrusion writes the exact coaxially bored quadrant.
func WriteQuarterAnnularExtrusion(w io.Writer, m QuarterAnnularExtrusionModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeQuarterAnnularExtrusion(&out, m); err != nil {
		return err
	}
	if _, err := ReadQuarterAnnularExtrusion(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

// ReadQuarterAnnularExtrusion recovers the aligned heights and wall radii and
// proves every represented face, edge, and pcurve against the generated graph.
func ReadQuarterAnnularExtrusion(r io.Reader, l Limits) (QuarterAnnularExtrusionModel, error) {
	fail := func() (QuarterAnnularExtrusionModel, error) {
		return QuarterAnnularExtrusionModel{}, circularSourceFailure()
	}
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil || len(ids) != 1 {
		return fail()
	}
	body, err := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
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
	ids, err = listRefs(shell.Args[1], 6)
	if err != nil || len(ids) != 6 {
		return fail()
	}
	var faces [6]circularSourceFace
	kinds := [6]string{"PLANE", "PLANE", "PLANE", "CYLINDER", "PLANE", "CYLINDER"}
	for i, id := range ids {
		faces[i], err = readCircularSourceFaceBounded(f, id, 1, 4)
		if err != nil || faces[i].kind != kinds[i] || len(faces[i].loops) != 1 || len(faces[i].loops[0]) != 4 {
			return fail()
		}
	}
	low, high, outer, inner := faces[0], faces[1], faces[3], faces[5]
	frame := low.frame
	if low.sense || !high.sense || !outer.sense || inner.sense || outer.frame != frame || inner.frame != frame || high.frame.ZAxis() != frame.ZAxis() {
		return fail()
	}
	height := frame.ToLocal(high.frame.Origin())
	if height.X != 0 || height.Y != 0 || height.Z <= 0 {
		return fail()
	}
	m := QuarterAnnularExtrusionModel{LengthUnit: unit, Body: QuarterAnnularExtrusionBody{
		Name: name, Frame: frame, Inner: inner.radius, Radius: outer.radius, Height: height.Z,
	}}
	var out bytes.Buffer
	if err := writeQuarterAnnularExtrusion(&out, m); err != nil {
		return fail()
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return fail()
	}
	return m, nil
}
