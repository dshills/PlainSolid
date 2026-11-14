package step

import (
	"bytes"
	"context"
	"io"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// UnequalBoreBody describes two unequal through bores along the local X and Z
// axes of a rectangular stock. Dimensions and Crossing are stock coordinates.
// Offset shifts the Z bore along local Y relative to the X bore.
type UnequalBoreBody struct {
	Name                 string
	Frame                geom.Frame
	Dimensions, Crossing geom.Vec3
	XRadius, ZRadius     float64
	Offset               float64
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
	partial := math.Abs(p.Offset) > math.Abs(p.XRadius-p.ZRadius)
	// Resolve the same represented support origins that the canonical reader
	// uses. In particular a decimal offset need not survive translating a
	// signed stock frame unless this recipe is normalized before spline fitting.
	if p.Offset != 0 {
		if !partial && p.ZRadius > p.XRadius {
			frame, err := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{Y: p.Dimensions.Y}), p.Frame.ZAxis(), p.Frame.XAxis())
			if err != nil {
				return err
			}
			p.Frame = frame
			p.Crossing = geom.Vec3{X: p.Crossing.Z, Y: p.Dimensions.Y - (p.Crossing.Y + p.Offset), Z: p.Crossing.X}
			p.Dimensions = geom.Vec3{X: p.Dimensions.Z, Y: p.Dimensions.Y, Z: p.Dimensions.X}
			p.XRadius, p.ZRadius = p.ZRadius, p.XRadius
		}
		if partial && p.Offset < 0 {
			frame, err := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{Y: p.Dimensions.Y, Z: p.Dimensions.Z}), p.Frame.XAxis(), p.Frame.ZAxis().Scale(-1))
			if err != nil {
				return err
			}
			p.Frame = frame
			p.Crossing = geom.Vec3{X: p.Crossing.X, Y: p.Dimensions.Y - p.Crossing.Y, Z: p.Dimensions.Z - p.Crossing.Z}
			p.Offset = -p.Offset
		}
		small := p.Crossing.Add(geom.Vec3{Y: p.Offset})
		largeWorld, smallWorld := p.Frame.ToWorld(p.Crossing), p.Frame.ToWorld(small)
		largeLocal, smallLocal := p.Frame.ToLocal(largeWorld), p.Frame.ToLocal(smallWorld)
		offset := smallLocal.Y - largeLocal.Y
		if largeLocal.X != smallLocal.X || largeLocal.Z != smallLocal.Z || largeLocal.Y+offset != smallLocal.Y ||
			offset == 0 || p.Frame.ToWorld(largeLocal) != largeWorld || p.Frame.ToWorld(smallLocal) != smallWorld {
			return geom.ErrUnrepresentable
		}
		p.Crossing, p.Offset = largeLocal, offset
	}
	ctx := context.Background()
	var result trimmed.BoxCrossedBoresResult
	var err error
	if partial {
		result, err = trimmed.BoxPartialSkewBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset, 1e-9)
	} else {
		result, err = trimmed.BoxSkewCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset, 1e-9)
	}
	if err != nil {
		return err
	}
	tolerance := UnequalBoreSTEPAccuracyMillimeters * .001 / m.LengthUnit
	return writeNativeLongEdgeTopologyOptions(w, m.LengthUnit, p.Name, result.Solid, nativeLongEdgeLimits{64, 256, 256, 5, 64}, true, tolerance)
}

// ReadUnequalBore accepts the canonical emitted approximations and recovers
// the original native material recipe. It checks the full graph against a
// deterministic reconstruction before replacing the approximating trims.
// For two separate trim loops it canonicalizes the larger bore to local X.
// Partial overlap uses its single trim loop and a positive local Y offset.
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
	faces, err := listRefs(shell.Args[1], 10)
	if err != nil || len(faces) != 16 && len(faces) != 10 {
		return fail()
	}
	lower, err := readCircularSourceFace(f, faces[0])
	if err != nil || lower.kind != "PLANE" {
		return fail()
	}
	partial := len(faces) == 10
	xID, zID := faces[6], faces[8]
	if !partial {
		xID, zID = faces[14], faces[6]
	}
	var supports [2]circularSourceFace
	for i, id := range []int64{xID, zID} {
		supports[i], err = readCircularSourceFace(f, id)
		if err != nil || supports[i].kind != "CYLINDER" || supports[i].sense {
			return fail()
		}
	}
	x, z := supports[0], supports[1]
	if !partial && z.radius >= x.radius {
		return fail()
	}
	frame, err := geom.NewFrame(lower.frame.Origin(), x.frame.ZAxis(), z.frame.ZAxis())
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
	crossing := frame.ToLocal(x.frame.Origin())
	other := frame.ToLocal(z.frame.Origin())
	offset := other.Y - crossing.Y
	if other.X != crossing.X || other.Z != crossing.Z || crossing.Y+offset != other.Y {
		return fail()
	}
	return UnequalBoreBody{Name: name, Frame: frame, Dimensions: bounds.Max, Crossing: crossing, XRadius: x.radius, ZRadius: z.radius, Offset: offset}, nil
}
