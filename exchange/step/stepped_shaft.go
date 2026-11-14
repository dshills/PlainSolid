package step

import (
	"bytes"
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// SteppedShaftModel retains coaxial stock and its optional blind polygonal socket.
type SteppedShaftModel struct {
	Name       string
	Body       trimmed.SteppedShaft
	LengthUnit float64
}

func writeSteppedShaft(w io.Writer, m SteppedShaftModel) error {
	s, err := m.Body.Solid(context.Background())
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Name, s, nativeLongEdgeLimits{76, 256, 160, 2, 64})
}

// WriteSteppedShaft exchanges the original circular stock and polygonal socket.
func WriteSteppedShaft(w io.Writer, m SteppedShaftModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeSteppedShaft(&out, m); err != nil {
		return err
	}
	if _, err := ReadSteppedShaft(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

// ReadSteppedShaft recovers the recipe and verifies the complete represented graph.
func ReadSteppedShaft(r io.Reader, l Limits) (SteppedShaftModel, error) {
	fail := func() (SteppedShaftModel, error) {
		return SteppedShaftModel{}, circularSourceFailure()
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
	ids, err = listRefs(shell.Args[1], 11)
	if err != nil || len(ids) > 76 {
		return fail()
	}
	var caps [3]circularSourceFace
	for i := range caps {
		caps[i], err = readCircularSourceFaceBounded(f, ids[i], 2, 64)
		if err != nil || caps[i].kind != "PLANE" || len(caps[i].loops[0]) != 4 {
			return fail()
		}
	}
	low, shoulder, head := caps[0], caps[1], caps[2]
	if low.sense || shoulder.sense || !head.sense || len(low.loops) != 1 || len(shoulder.loops) != 2 ||
		len(shoulder.loops[1]) != 4 || low.frame.ZAxis() != shoulder.frame.ZAxis() || head.frame.ZAxis() != shoulder.frame.ZAxis() {
		return fail()
	}
	frame := shoulder.frame
	a, b := frame.ToLocal(low.frame.Origin()), frame.ToLocal(head.frame.Origin())
	if a.X != 0 || a.Y != 0 || b.X != 0 || b.Y != 0 || a.Z >= 0 || b.Z <= 0 {
		return fail()
	}
	shaftWall, err := readCircularSourceFaceBounded(f, ids[len(ids)-8], 1, 4)
	if err != nil || shaftWall.kind != "CYLINDER" || !shaftWall.sense {
		return fail()
	}
	headWall, err := readCircularSourceFaceBounded(f, ids[len(ids)-7], 1, 4)
	if err != nil || headWall.kind != "CYLINDER" || !headWall.sense {
		return fail()
	}
	p := trimmed.SteppedShaft{Frame: frame, ShaftRadius: shaftWall.radius, HeadRadius: headWall.radius, ShaftLength: -a.Z, HeadHeight: b.Z}
	if len(head.loops) == 2 {
		wire := head.loops[1]
		if len(wire) < 3 || len(ids) != 12+len(wire) {
			return fail()
		}
		floor, err := readCircularSourceFaceBounded(f, ids[len(ids)-9], 1, 64)
		if err != nil || floor.kind != "PLANE" || !floor.sense || floor.frame.ZAxis() != frame.ZAxis() ||
			len(floor.loops) != 1 || len(floor.loops[0]) != len(wire) {
			return fail()
		}
		p.SocketFloor = frame.ToLocal(floor.frame.Origin()).Z
		for _, use := range wire {
			edge, err := readCircularEdge(f, use.id, true)
			if err != nil || edge.kind != "LINE" {
				return fail()
			}
			id := edge.a
			if !use.sense {
				id = edge.b
			}
			v, err := analyticVertex(f, id)
			if err != nil {
				return fail()
			}
			v = frame.ToLocal(v)
			if v.Z != p.HeadHeight {
				return fail()
			}
			p.Socket = append(p.Socket, geom.Vec2{X: v.X, Y: v.Y})
		}
	} else if len(ids) != 11 {
		return fail()
	}
	m := SteppedShaftModel{Name: name, Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeSteppedShaft(&out, m); err != nil {
		return fail()
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return fail()
	}
	return m, nil
}
