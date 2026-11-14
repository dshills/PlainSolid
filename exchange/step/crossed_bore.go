package step

import (
	"bytes"
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CrossedBoreBody describes a rectangular stock and equal-radius through
// cutters along its local X and Z axes. Crossing is in stock coordinates.
// A positive BlindDepth stops the Z cutter beyond the complete intersection.
// OppositeCap starts that cutter at the upper cap.
type CrossedBoreBody struct {
	Name                 string
	Frame                geom.Frame
	Dimensions, Crossing geom.Vec3
	Radius               float64
	BlindDepth           float64
	OppositeCap          bool
}

type CrossedBoreModel struct {
	Body       CrossedBoreBody
	LengthUnit float64
}

// WriteCrossedBore exports the analytic boundary and checks its canonical
// readback before writing to the caller.
func WriteCrossedBore(w io.Writer, m CrossedBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeCrossedBore(&out, m); err != nil {
		return err
	}
	if _, err := ReadCrossedBore(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

func writeCrossedBore(w io.Writer, m CrossedBoreModel) error {
	p := m.Body
	ctx := context.Background()
	var result trimmed.BoxCrossedBoresResult
	var err error
	if p.BlindDepth != 0 || p.OppositeCap {
		result, err = trimmed.BoxBlindCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.Radius, p.BlindDepth, p.OppositeCap, 1e-9)
	} else {
		result, err = trimmed.BoxCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.Radius, 1e-9)
	}
	if err != nil {
		return err
	}
	return writeNativeCrossedBoreTopology(w, m.LengthUnit, p.Name, result.Solid)
}

// ReadCrossedBore recovers the stock and two cylinder supports from the
// canonical graph, then regenerates the complete boundary to check the trims.
func ReadCrossedBore(r io.Reader, limits Limits) (CrossedBoreModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return CrossedBoreModel{}, circularSourceFailure()
	}
	p, err := recoverCrossedBore(f, ids[0])
	if err != nil {
		return CrossedBoreModel{}, err
	}
	m := CrossedBoreModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeCrossedBore(&out, m); err != nil {
		return CrossedBoreModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return CrossedBoreModel{}, err
	}
	return m, nil
}

func recoverCrossedBore(f *File, id int64) (CrossedBoreBody, error) {
	fail := func() (CrossedBoreBody, error) { return CrossedBoreBody{}, circularSourceFailure() }
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
	faces, err := listRefs(shell.Args[1], 22)
	if err != nil || len(faces) != 22 && len(faces) != 23 {
		return fail()
	}
	lower, err := readCircularSourceFace(f, faces[0])
	if err != nil || lower.kind != "PLANE" {
		return fail()
	}
	cylinderStart := 6
	blind := len(faces) == 23
	if blind {
		cylinderStart++
	}
	var supports [2]circularSourceFace
	for i, id := range []int64{faces[cylinderStart], faces[cylinderStart+8]} {
		supports[i], err = readCircularSourceFace(f, id)
		if err != nil || supports[i].kind != "CYLINDER" || supports[i].sense {
			return fail()
		}
	}
	a, b := supports[0], supports[1]
	if a.radius != b.radius || a.frame.Origin() != b.frame.Origin() {
		return fail()
	}
	frame, err := geom.NewFrame(lower.frame.Origin(), b.frame.ZAxis(), a.frame.ZAxis())
	if err != nil {
		return fail()
	}
	var vertices []geom.Vec3
	for _, id := range faces[:6] {
		face, err := readCircularSourceFace(f, id)
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
	p := CrossedBoreBody{Name: name, Frame: frame, Dimensions: bounds.Max, Crossing: frame.ToLocal(a.frame.Origin()), Radius: a.radius}
	if blind {
		floor, err := readCircularSourceFace(f, faces[trimmed.BlindCrossedBoreFloorFace])
		if err != nil || floor.kind != "PLANE" || len(floor.loops) != 1 || len(floor.loops[0]) != 4 {
			return fail()
		}
		high, err := readCircularSourceFace(f, faces[1])
		if err != nil || len(lower.loops)+len(high.loops) != 3 {
			return fail()
		}
		p.OppositeCap = len(high.loops) == 2
		level := frame.ToLocal(floor.frame.Origin()).Z
		p.BlindDepth = level
		if p.OppositeCap {
			p.BlindDepth = bounds.Max.Z - level
		}
	}
	return p, nil
}
