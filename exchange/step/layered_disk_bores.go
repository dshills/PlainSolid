package step

import (
	"bytes"
	"context"
	"io"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteLayeredDiskBores writes a circular solid with intersecting cylindrical
// cutters. The exterior and every exposed floor and wall are native STEP
// surfaces. The actual emitted graph must pass recipe recovery before writing.
func WriteLayeredDiskBores(w io.Writer, model DiskHoleExtrusionModel) error {
	if w == nil || len(model.Bodies) != 1 {
		return geom.ErrInvalid
	}
	b := model.Bodies[0]
	if b.Height < 0 {
		frame, err := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{Z: b.Height}), b.Frame.XAxis(), b.Frame.ZAxis())
		if err != nil {
			return err
		}
		b.Frame, b.Height = frame, -b.Height
		b.Blind = append([]trimmed.ExtrusionBlindHole(nil), b.Blind...)
		for i := range b.Blind {
			b.Blind[i].OppositeCap = !b.Blind[i].OppositeCap
		}
	}
	var out bytes.Buffer
	if err := writeLayeredDiskBody(&out, model.LengthUnit, b); err != nil {
		return err
	}
	if _, err := ReadLayeredDiskBores(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

func writeLayeredDiskBody(w io.Writer, unit float64, b DiskHoleExtrusionBody) error {
	body, _, err := trimmed.BoreDiskDepthUnion(context.Background(), b.Frame, b.Outer, b.Holes, b.Blind, b.Height)
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopology(w, unit, b.Name, body)
}

// ReadLayeredDiskBores accepts canonical circular layered boundaries whose
// cylindrical supports and exposed axial intervals recover an identical graph.
func ReadLayeredDiskBores(r io.Reader, limits Limits) (DiskHoleExtrusionModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return DiskHoleExtrusionModel{}, circularSourceFailure()
	}
	b, err := recoverLayeredDiskBores(f, ids[0])
	if err != nil {
		return DiskHoleExtrusionModel{}, err
	}
	var out bytes.Buffer
	if err := writeLayeredDiskBody(&out, unit, b); err != nil {
		return DiskHoleExtrusionModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return DiskHoleExtrusionModel{}, err
	}
	return DiskHoleExtrusionModel{LengthUnit: unit, Bodies: []DiskHoleExtrusionBody{b}}, nil
}

func recoverLayeredDiskBores(f *File, id int64) (DiskHoleExtrusionBody, error) {
	fail := func() (DiskHoleExtrusionBody, error) { return DiskHoleExtrusionBody{}, circularSourceFailure() }
	g, err := readCircularSourceGraph(f, id)
	if err != nil || len(g.faces) < 7 || g.faces[0].kind != "PLANE" || g.faces[1].kind != "PLANE" {
		return fail()
	}
	frame := g.faces[0].frame
	shift := frame.ToLocal(g.faces[1].frame.Origin())
	if shift.X != 0 || shift.Y != 0 || shift.Z <= 0 || frame.XAxis() != g.faces[1].frame.XAxis() || frame.ZAxis() != g.faces[1].frame.ZAxis() {
		return fail()
	}
	body, err := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return fail()
	}
	name, err := textValue(body.Args[0])
	if err != nil {
		return fail()
	}
	out := DiskHoleExtrusionBody{Name: name, Frame: frame, Height: shift.Z}
	type axialSpan struct{ low, high float64 }
	tools := map[trimmed.ExtrusionCircle]axialSpan{}
	for i, face := range g.faces[2:] {
		if i >= 4 && face.kind == "PLANE" {
			continue
		}
		center := frame.ToLocal(face.frame.Origin())
		if face.kind != "CYLINDER" || face.frame.XAxis() != frame.XAxis() || face.frame.ZAxis() != frame.ZAxis() || center.Z != 0 || face.sense != (i < 4) {
			return fail()
		}
		circle := trimmed.ExtrusionCircle{Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: face.radius}
		if i < 4 {
			if i == 0 {
				out.Outer = circle
			}
			if out.Outer != circle {
				return fail()
			}
			continue
		}
		span, seen := tools[circle]
		if !seen {
			span = axialSpan{math.Inf(1), math.Inf(-1)}
		}
		for _, use := range face.loops[0] {
			edge := g.edges[use.id]
			if edge.kind == "CIRCLE" {
				z := frame.ToLocal(edge.frame.Origin()).Z
				span.low, span.high = min(span.low, z), max(span.high, z)
			}
		}
		if span.low < 0 || span.high > out.Height || span.high <= span.low {
			return fail()
		}
		tools[circle] = span
	}
	if len(tools) == 0 || len(tools) > 32 {
		return fail()
	}
	// The native arrangement sorts its tools by footprint and axial span.
	// No stored process metadata is used to reproduce the material boundary.
	for circle, span := range tools {
		switch {
		case span.low == 0 && span.high == out.Height:
			out.Holes = append(out.Holes, circle)
		case span.low == 0:
			out.Blind = append(out.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: circle, Depth: span.high})
		case span.high == out.Height:
			out.Blind = append(out.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: circle, Depth: out.Height - span.low, OppositeCap: true})
		default:
			return fail()
		}
	}
	return out, nil
}
