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
// OppositeCap starts that cutter at the upper cap. A positive XBlindDepth
// also stops the X cutter beyond the intersection, with XOppositeCap
// selecting its upper cap.
type CrossedBoreBody struct {
	Name                 string
	Frame                geom.Frame
	Dimensions, Crossing geom.Vec3
	Radius               float64
	BlindDepth           float64
	OppositeCap          bool
	XBlindDepth          float64
	XOppositeCap         bool
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
	switch {
	case p.XBlindDepth != 0 || p.XOppositeCap:
		result, err = trimmed.BoxDoubleBlindCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.Radius, p.XBlindDepth, p.BlindDepth, p.XOppositeCap, p.OppositeCap, 1e-9)
	case p.BlindDepth != 0 || p.OppositeCap:
		result, err = trimmed.BoxBlindCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.Radius, p.BlindDepth, p.OppositeCap, 1e-9)
	default:
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
	if err != nil || len(faces) > 24 {
		return fail()
	}
	lower, err := readCircularSourceFace(f, faces[0])
	if err != nil || lower.kind != "PLANE" {
		return fail()
	}
	zBlind, xBlind := len(faces) >= 23, len(faces) == 24
	zStart, xStart := 6, 14
	if zBlind {
		zStart++
		xStart++
	}
	if xBlind {
		xStart++
	}
	var supports [2]circularSourceFace
	for i, id := range []int64{faces[zStart], faces[xStart]} {
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
	if zBlind {
		var ok bool
		p.BlindDepth, p.OppositeCap, ok = crossedFloorDepth(f, frame.Origin(), faces[trimmed.BlindCrossedBoreFloorFace],
			faces[0], faces[1], bounds.Max.Z, frame.ZAxis())
		if !ok {
			return fail()
		}
	}
	if xBlind {
		var ok bool
		p.XBlindDepth, p.XOppositeCap, ok = crossedFloorDepth(f, frame.Origin(), faces[trimmed.DoubleBlindCrossedBoreXFloorFace],
			faces[5], faces[3], bounds.Max.X, frame.XAxis())
		if !ok {
			return fail()
		}
	}
	return p, nil
}

func crossedFloorDepth(f *File, origin geom.Vec3, floorID, lowerID, upperID int64, span float64, axis geom.Vec3) (float64, bool, bool) {
	floor, err := readCircularSourceFace(f, floorID)
	if err != nil || floor.kind != "PLANE" || len(floor.loops) != 1 || len(floor.loops[0]) != 4 {
		return 0, false, false
	}
	lower, err := readCircularSourceFace(f, lowerID)
	if err != nil {
		return 0, false, false
	}
	upper, err := readCircularSourceFace(f, upperID)
	if err != nil || len(lower.loops)+len(upper.loops) != 3 {
		return 0, false, false
	}
	level := floor.frame.Origin().Sub(origin).Dot(axis)
	if len(upper.loops) == 2 {
		return span - level, true, true
	}
	return level, false, true
}
