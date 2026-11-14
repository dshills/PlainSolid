package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// QuarterSectorExtrusionBody is a circular quadrant bounded by two radial
// lines, extruded along the signed cardinal frame normal. Height is positive.
// Quarter selects the counterclockwise arc starting at Quarter*pi/2.
type QuarterSectorExtrusionBody struct {
	Name           string
	Frame          geom.Frame
	Center         geom.Vec2
	Radius, Height float64
	Quarter        int
}
type QuarterSectorExtrusionModel struct {
	Body       QuarterSectorExtrusionBody
	LengthUnit float64
}

// Segments returns a fresh owned canonical radial/arc/radial profile.
func (p QuarterSectorExtrusionBody) Segments() ([]trimmed.ExtrusionSegment, error) {
	if !finiteOblique(p.Center.X) || !finiteOblique(p.Center.Y) || p.Quarter < 0 || p.Quarter > 3 || !finiteOblique(p.Height) || p.Height <= 0 || !finiteOblique(p.Radius) || p.Radius <= 0 {
		return nil, geom.ErrDomain
	}
	for _, v := range []geom.Vec3{p.Frame.XAxis(), p.Frame.YAxis(), p.Frame.ZAxis()} {
		n := 0
		for _, c := range []float64{v.X, v.Y, v.Z} {
			if c != 0 {
				if c != 1 && c != -1 {
					return nil, geom.ErrDomain
				}
				n++
			}
		}
		if n != 1 {
			return nil, geom.ErrDomain
		}
	}
	points := [4]geom.Vec2{{X: p.Center.X + p.Radius, Y: p.Center.Y}, {X: p.Center.X, Y: p.Center.Y + p.Radius}, {X: p.Center.X - p.Radius, Y: p.Center.Y}, {X: p.Center.X, Y: p.Center.Y - p.Radius}}
	a, b := points[p.Quarter], points[(p.Quarter+1)%4]
	segments := []trimmed.ExtrusionSegment{{Start: p.Center, End: a}, {Start: a, End: b, Center: p.Center, Radius: p.Radius, StartAngle: float64(p.Quarter) * math.Pi / 2, Sweep: math.Pi / 2}, {Start: b, End: p.Center}}
	return segments, nil
}

// BuildQuarterSectorExtrusion retains the exact profile and analytic measures.
func BuildQuarterSectorExtrusion(p QuarterSectorExtrusionBody) (trimmed.ArcLoopExtrusion, error) {
	segments, e := p.Segments()
	if e != nil {
		return trimmed.ArcLoopExtrusion{}, e
	}
	return trimmed.ExtrudeArcLoop(context.Background(), p.Frame, segments, p.Height, max(1, p.Height, p.Radius)*1e-10)
}
func writeQuarterSectorExtrusion(w io.Writer, m QuarterSectorExtrusionModel) error {
	s, e := BuildQuarterSectorExtrusion(m.Body)
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Body.Name, s.Solid, nativeLongEdgeLimits{5, 9, 6, 1, 4})
}
func WriteQuarterSectorExtrusion(w io.Writer, m QuarterSectorExtrusionModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := writeQuarterSectorExtrusion(&b, m); e != nil {
		return e
	}
	if _, e := ReadQuarterSectorExtrusion(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &b)
	return e
}

// ReadQuarterSectorExtrusion accepts only the complete regenerated native
// graph. Four bounded candidate quadrants avoid reconstructing arc angles
// through atan2 and changing represented endpoint parameters.
func ReadQuarterSectorExtrusion(r io.Reader, l Limits) (QuarterSectorExtrusionModel, error) {
	fail := func() (QuarterSectorExtrusionModel, error) {
		return QuarterSectorExtrusionModel{}, circularSourceFailure()
	}
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil || len(ids) != 1 {
		return fail()
	}
	body, e := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
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
	faces, e := listRefs(shell.Args[1], 5)
	if e != nil || len(faces) != 5 {
		return fail()
	}
	var support [5]circularSourceFace
	for i, id := range faces {
		support[i], e = readCircularSourceFaceBounded(f, id, 1, 4)
		if e != nil || len(support[i].loops) != 1 {
			return fail()
		}
		want := "PLANE"
		if i == 3 {
			want = "CYLINDER"
		}
		if support[i].kind != want {
			return fail()
		}
	}
	low, high, cyl := support[0], support[1], support[3]
	if low.sense || !high.sense || !cyl.sense || len(low.loops[0]) != 3 || len(high.loops[0]) != 3 {
		return fail()
	}
	frame := low.frame
	delta := frame.ToLocal(high.frame.Origin())
	center := frame.ToLocal(cyl.frame.Origin())
	if delta.X != 0 || delta.Y != 0 || delta.Z <= 0 || center.Z != 0 || high.frame.ZAxis() != frame.ZAxis() || cyl.frame.ZAxis() != frame.ZAxis() {
		return fail()
	}
	m := QuarterSectorExtrusionModel{LengthUnit: unit, Body: QuarterSectorExtrusionBody{Name: name, Frame: frame, Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: cyl.radius, Height: delta.Z}}
	for q := 0; q < 4; q++ {
		m.Body.Quarter = q
		var b bytes.Buffer
		if e = writeQuarterSectorExtrusion(&b, m); e != nil {
			continue
		}
		if e = verifyCanonicalCircularExtrusionGraph(f, rep, &b); e == nil {
			return m, nil
		}
	}
	return fail()
}
