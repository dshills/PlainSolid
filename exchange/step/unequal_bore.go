package step

import (
	"bytes"
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// UnequalBoreBody describes two unequal through bores along the local X and Z
// axes of a rectangular stock. Dimensions and Crossing are stock coordinates.
type UnequalBoreBody struct {
	Name                 string
	Frame                geom.Frame
	Dimensions, Crossing geom.Vec3
	XRadius, ZRadius     float64
}

type UnequalBoreModel struct {
	Body       UnequalBoreBody
	LengthUnit float64
}

// UnequalBoreSTEPAccuracyMillimeters is the declared STEP distance uncertainty.
// The cylinders and planes retain analytic supports. The nonrational model
// intersection and its paired trims use approximating cubic splines.
const UnequalBoreSTEPAccuracyMillimeters = 1e-7

// WriteUnequalBore writes the bounded spline approximation and reads its entire
// graph back before returning it to the caller. Interpolation uses a fourth
// derivative bound and a separate floating-point reserve. It rejects models
// outside the supported span count or coordinate scale.
func WriteUnequalBore(w io.Writer, m UnequalBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeUnequalBore(&out, m); err != nil {
		return err
	}
	if _, err := ReadUnequalBore(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

func writeUnequalBore(w io.Writer, m UnequalBoreModel) error {
	if !validLengthUnit(m.LengthUnit) {
		return geom.ErrInvalid
	}
	p := m.Body
	result, err := trimmed.BoxUnequalCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, 1e-9)
	if err != nil {
		return err
	}
	tolerance := UnequalBoreSTEPAccuracyMillimeters * .001 / m.LengthUnit
	return writeNativeLongEdgeTopologyOptions(w, m.LengthUnit, p.Name, result.Solid, nativeLongEdgeLimits{64, 256, 256, 5, 64}, true, tolerance)
}

// ReadUnequalBore accepts the canonical emitted approximations and recovers
// the original native material recipe. It checks the full graph against a
// deterministic reconstruction before replacing the approximating trims.
// It canonicalizes the larger bore to local X, preserving the world geometry.
func ReadUnequalBore(r io.Reader, limits Limits) (UnequalBoreModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return UnequalBoreModel{}, circularSourceFailure()
	}
	p, err := recoverUnequalBore(f, ids[0])
	if err != nil {
		return UnequalBoreModel{}, err
	}
	m := UnequalBoreModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeUnequalBore(&out, m); err != nil {
		return UnequalBoreModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return UnequalBoreModel{}, err
	}
	return m, nil
}

func recoverUnequalBore(f *File, id int64) (UnequalBoreBody, error) {
	fail := func() (UnequalBoreBody, error) { return UnequalBoreBody{}, circularSourceFailure() }
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
	faces, err := listRefs(shell.Args[1], 16)
	if err != nil || len(faces) != 16 {
		return fail()
	}
	lower, err := readCircularSourceFace(f, faces[0])
	if err != nil || lower.kind != "PLANE" {
		return fail()
	}
	var supports [2]circularSourceFace
	for i, id := range []int64{faces[6], faces[14]} {
		supports[i], err = readCircularSourceFace(f, id)
		if err != nil || supports[i].kind != "CYLINDER" || supports[i].sense {
			return fail()
		}
	}
	small, large := supports[0], supports[1]
	if small.radius >= large.radius || small.frame.Origin() != large.frame.Origin() {
		return fail()
	}
	frame, err := geom.NewFrame(lower.frame.Origin(), large.frame.ZAxis(), small.frame.ZAxis())
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
	return UnequalBoreBody{Name: name, Frame: frame, Dimensions: bounds.Max, Crossing: frame.ToLocal(small.frame.Origin()), XRadius: large.radius, ZRadius: small.radius}, nil
}
