package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// SemicircleExtrusionBody retains the upper half disk in local XY, bounded
// by its diameter, with positive height along a signed cardinal frame normal.
type SemicircleExtrusionBody struct {
	Name           string
	Frame          geom.Frame
	Center         geom.Vec2
	Radius, Height float64
}
type SemicircleExtrusionModel struct {
	Body       SemicircleExtrusionBody
	LengthUnit float64
}

func BuildSemicircleExtrusion(p SemicircleExtrusionBody) (trimmed.Solid, error) {
	if !finiteOblique(p.Height) || p.Height <= 0 {
		return trimmed.Solid{}, geom.ErrDomain
	}
	for _, v := range []geom.Vec3{p.Frame.XAxis(), p.Frame.YAxis(), p.Frame.ZAxis()} {
		n := 0
		for _, c := range []float64{v.X, v.Y, v.Z} {
			if c != 0 {
				if c != 1 && c != -1 {
					return trimmed.Solid{}, geom.ErrDomain
				}
				n++
			}
		}
		if n != 1 {
			return trimmed.Solid{}, geom.ErrDomain
		}
	}
	return trimmed.ExtrudeSemicircle(context.Background(), p.Frame, p.Center, p.Radius, p.Height)
}
func writeSemicircleExtrusion(w io.Writer, m SemicircleExtrusionModel) error {
	s, e := BuildSemicircleExtrusion(m.Body)
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Body.Name, s, nativeLongEdgeLimits{5, 9, 6, 1, 4})
}
func WriteSemicircleExtrusion(w io.Writer, m SemicircleExtrusionModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := writeSemicircleExtrusion(&b, m); e != nil {
		return e
	}
	if _, e := ReadSemicircleExtrusion(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &b)
	return e
}

// ReadSemicircleExtrusion proves the entire source graph against regenerated
// native circular geometry; surface identities alone never establish a match.
func ReadSemicircleExtrusion(r io.Reader, l Limits) (SemicircleExtrusionModel, error) {
	fail := func() (SemicircleExtrusionModel, error) { return SemicircleExtrusionModel{}, circularSourceFailure() }
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
		if i == 2 || i == 3 {
			want = "CYLINDER"
		}
		if support[i].kind != want {
			return fail()
		}
	}
	low, high, cyl := support[0], support[1], support[2]
	if low.sense || !high.sense || !cyl.sense || !support[3].sense || len(low.loops[0]) != 3 || len(high.loops[0]) != 3 {
		return fail()
	}
	frame := low.frame
	delta := frame.ToLocal(high.frame.Origin())
	center := frame.ToLocal(cyl.frame.Origin())
	if delta.X != 0 || delta.Y != 0 || delta.Z <= 0 || center.Z != 0 || high.frame.ZAxis() != frame.ZAxis() || cyl.frame.ZAxis() != frame.ZAxis() {
		return fail()
	}
	m := SemicircleExtrusionModel{LengthUnit: unit, Body: SemicircleExtrusionBody{Name: name, Frame: frame, Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: cyl.radius, Height: delta.Z}}
	var b bytes.Buffer
	if e = writeSemicircleExtrusion(&b, m); e != nil {
		return fail()
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &b); e != nil {
		return fail()
	}
	return m, nil
}
