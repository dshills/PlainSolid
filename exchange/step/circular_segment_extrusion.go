package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CircularSegmentExtrusionBody retains a circular arc and closing chord.
// Height and Sweep are positive; Frame has signed cardinal axes.
// Frame origin is the circle center on the lower cap. StartAngle must be
// normalized, EndAngle must not exceed pi, and endpoint subtraction must be
// exactly reversible. Branch-crossing intervals are not remapped or snapped.
type CircularSegmentExtrusionBody struct {
	Name              string
	Frame             geom.Frame
	Radius, Height    float64
	StartAngle, Sweep float64
}
type CircularSegmentExtrusionModel struct {
	Body       CircularSegmentExtrusionBody
	LengthUnit float64
}

func BuildCircularSegmentExtrusion(p CircularSegmentExtrusionBody) (trimmed.Solid, error) {
	if !finiteOblique(p.Height) || p.Height <= 0 || !finiteOblique(p.StartAngle) || math.Remainder(p.StartAngle, 2*math.Pi) != p.StartAngle || !finiteOblique(p.Sweep) || p.Sweep <= 0 || p.Sweep >= 2*math.Pi || (p.StartAngle+p.Sweep)-p.StartAngle != p.Sweep || p.StartAngle+p.Sweep > math.Pi {
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
	return trimmed.ExtrudeCircularSegment(context.Background(), p.Frame, geom.Vec2{}, p.Radius, p.StartAngle, p.Sweep, p.Height)
}
func writeCircularSegmentExtrusion(w io.Writer, m CircularSegmentExtrusionModel) error {
	s, e := BuildCircularSegmentExtrusion(m.Body)
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Body.Name, s, nativeLongEdgeLimits{8, 18, 12, 1, 6})
}
func WriteCircularSegmentExtrusion(w io.Writer, m CircularSegmentExtrusionModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := writeCircularSegmentExtrusion(&b, m); e != nil {
		return e
	}
	if _, e := ReadCircularSegmentExtrusion(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &b)
	return e
}

// ReadCircularSegmentExtrusion proves the entire source graph against regenerated
// native circular geometry. Angular endpoints are recovered from axial seam
// pcurve LINE coordinates, never inferred through atan2 of rounded vertices.
func ReadCircularSegmentExtrusion(r io.Reader, l Limits) (CircularSegmentExtrusionModel, error) {
	fail := func() (CircularSegmentExtrusionModel, error) {
		return CircularSegmentExtrusionModel{}, circularSourceFailure()
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
	if e != nil || len(faces) < 5 || len(faces) > 8 {
		return fail()
	}
	n := len(faces) - 3
	support := make([]circularSourceFace, len(faces))
	for i, id := range faces {
		support[i], e = readCircularSourceFaceBounded(f, id, 1, 6)
		if e != nil || len(support[i].loops) != 1 {
			return fail()
		}
		want := "PLANE"
		if i >= 2 && i < 2+n {
			want = "CYLINDER"
		}
		if support[i].kind != want {
			return fail()
		}
	}
	low, high, cyl := support[0], support[1], support[2]
	if low.sense || !high.sense || len(low.loops[0]) != n+1 || len(high.loops[0]) != n+1 {
		return fail()
	}
	frame := low.frame
	delta := frame.ToLocal(high.frame.Origin())
	if delta.X != 0 || delta.Y != 0 || delta.Z <= 0 || cyl.frame.Origin() != frame.Origin() || high.frame.ZAxis() != frame.ZAxis() || cyl.frame.ZAxis() != frame.ZAxis() {
		return fail()
	}
	angles := make([]float64, n+1)
	for i, wall := range support[2 : 2+n] {
		if !wall.sense || wall.radius != cyl.radius || wall.frame != cyl.frame || len(wall.loops[0]) != 4 {
			return fail()
		}
		var values []float64
		for _, u := range wall.loops[0] {
			edge, err := readCircularEdge(f, u.id, true)
			if err != nil {
				return fail()
			}
			if edge.kind != "LINE" {
				if edge.kind != "CIRCLE" {
					return fail()
				}
				continue
			}
			found := false
			for _, pc := range edge.pcs {
				owner, geometry, err := circularPCurveGeometry(f, pc)
				if err != nil {
					return fail()
				}
				if owner != wall.surface {
					continue
				}
				if found {
					return fail()
				}
				found = true
				point, direction, err := circularUnitLineGeometry(f, geometry, 2)
				if err != nil || direction.X != 0 || math.Abs(direction.Y) != 1 || point.Y != 0 && point.Y != delta.Z {
					return fail()
				}
				values = append(values, point.X)
			}
			if !found {
				return fail()
			}
		}
		if len(values) != 2 {
			return fail()
		}
		lo, hi := min(values[0], values[1]), max(values[0], values[1])
		if lo >= hi || i > 0 && lo != angles[i] {
			return fail()
		}
		angles[i], angles[i+1] = lo, hi
	}
	start, sweep := angles[0], angles[n]-angles[0]
	if n != max(2, int(math.Ceil(sweep/(math.Pi/2)))+1) {
		return fail()
	}
	for i, u := range angles {
		if u != start+sweep*(float64(i)/float64(n)) {
			return fail()
		}
	}
	m := CircularSegmentExtrusionModel{LengthUnit: unit, Body: CircularSegmentExtrusionBody{Name: name, Frame: frame, Radius: cyl.radius, Height: delta.Z, StartAngle: start, Sweep: sweep}}
	var b bytes.Buffer
	if e = writeCircularSegmentExtrusion(&b, m); e != nil {
		return fail()
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &b); e != nil {
		return fail()
	}
	return m, nil
}
