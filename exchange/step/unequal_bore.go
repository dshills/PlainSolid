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
	XBlindDepth          float64
	ZBlindDepth          float64
	XOppositeCap         bool
	ZOppositeCap         bool
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
	return writeUnequalBoreOptions(w, m, true)
}

func writeUnequalBoreOptions(w io.Writer, m UnequalBoreModel, compactFinite bool) error {
	if !validLengthUnit(m.LengthUnit) {
		return geom.ErrInvalid
	}
	p := m.Body
	partial := math.Abs(p.Offset) > math.Abs(p.XRadius-p.ZRadius)
	hasFloor := p.XBlindDepth != 0 || p.ZBlindDepth != 0
	if p.XBlindDepth == 0 && p.XOppositeCap || p.ZBlindDepth == 0 && p.ZOppositeCap ||
		!finiteNumber(p.XBlindDepth) || !finiteNumber(p.ZBlindDepth) || p.XBlindDepth < 0 || p.ZBlindDepth < 0 {
		return geom.ErrDomain
	}
	// Resolve the same represented support origins that the canonical reader
	// uses. In particular a decimal offset need not survive translating a
	// signed stock frame unless this recipe is normalized before spline fitting.
	if p.Offset != 0 || hasFloor {
		if !partial && p.ZRadius > p.XRadius {
			frame, err := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{Y: p.Dimensions.Y}), p.Frame.ZAxis(), p.Frame.XAxis())
			if err != nil {
				return err
			}
			p.Frame = frame
			p.Crossing = geom.Vec3{X: p.Crossing.Z, Y: p.Dimensions.Y - (p.Crossing.Y + p.Offset), Z: p.Crossing.X}
			p.Dimensions = geom.Vec3{X: p.Dimensions.Z, Y: p.Dimensions.Y, Z: p.Dimensions.X}
			p.XRadius, p.ZRadius = p.ZRadius, p.XRadius
			p.XBlindDepth, p.ZBlindDepth = p.ZBlindDepth, p.XBlindDepth
			p.XOppositeCap, p.ZOppositeCap = p.ZOppositeCap, p.XOppositeCap
		}
		if partial && p.Offset < 0 {
			frame, err := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{Y: p.Dimensions.Y, Z: p.Dimensions.Z}), p.Frame.XAxis(), p.Frame.ZAxis().Scale(-1))
			if err != nil {
				return err
			}
			p.Frame = frame
			p.Crossing = geom.Vec3{X: p.Crossing.X, Y: p.Dimensions.Y - p.Crossing.Y, Z: p.Dimensions.Z - p.Crossing.Z}
			p.Offset = -p.Offset
			if p.ZBlindDepth != 0 {
				p.ZOppositeCap = !p.ZOppositeCap
			}
		}
		if p.Offset != 0 {
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
	}
	if hasFloor {
		var e error
		p.XBlindDepth, e = canonicalUnequalBoreDepth(p.Frame, geom.Vec3{X: 1}, p.Dimensions.X, p.XBlindDepth, p.XOppositeCap)
		if e != nil {
			return e
		}
		p.ZBlindDepth, e = canonicalUnequalBoreDepth(p.Frame, geom.Vec3{Z: 1}, p.Dimensions.Z, p.ZBlindDepth, p.ZOppositeCap)
		if e != nil {
			return e
		}
	}

	ctx := context.Background()
	var result trimmed.BoxCrossedBoresResult
	var err error
	if hasFloor && partial {
		var finite trimmed.FiniteSkewCrossedBoresResult
		finite, err = trimmed.BoxFinitePartialSkewBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset,
			p.XBlindDepth, p.ZBlindDepth, p.XOppositeCap, p.ZOppositeCap, 1e-9)
		result = finite.BoxCrossedBoresResult
	} else if hasFloor {
		var finite trimmed.FiniteSkewCrossedBoresResult
		finite, err = trimmed.BoxFiniteSkewCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset,
			p.XBlindDepth, p.ZBlindDepth, p.XOppositeCap, p.ZOppositeCap, 1e-9)
		result = finite.BoxCrossedBoresResult
	} else if partial {
		result, err = trimmed.BoxPartialSkewBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset, 1e-9)
	} else {
		result, err = trimmed.BoxSkewCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset, 1e-9)
	}
	if err != nil {
		return err
	}
	tolerance := UnequalBoreSTEPAccuracyMillimeters * .001 / m.LengthUnit
	if hasFloor && compactFinite {
		return writeNativeFiniteUnequalBoreTopology(w, m.LengthUnit, p.Name, result.Solid, tolerance)
	}
	return writeNativeLongEdgeTopologyOptions(w, m.LengthUnit, p.Name, result.Solid, nativeLongEdgeLimits{64, 256, 256, 5, 64}, true, tolerance)
}

// Reproduce the translated exit-plane origin used by the native bore stop.
func canonicalUnequalBoreDepth(frame geom.Frame, axis geom.Vec3, size, depth float64, opposite bool) (float64, error) {
	if depth == 0 {
		return 0, nil
	}
	direction := frame.XAxis()
	if axis.Z != 0 {
		direction = frame.ZAxis()
	}
	exit := frame.ToWorld(axis.Scale(size))
	sign := 1.
	if opposite {
		exit, sign = frame.Origin(), -1
	}
	original := depth
	for range 4 {
		floor := exit.Add(direction.Scale(-sign * (size - depth)))
		next := frame.ToLocal(floor).Dot(axis)
		if opposite {
			next = size - next
		}
		if !finiteNumber(next) || math.Abs(next-original) > 1e-9/8 {
			return 0, geom.ErrUnrepresentable
		}
		if next == depth {
			return depth, nil
		}
		depth = next
	}
	return 0, geom.ErrUnrepresentable
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
		// Accept only the earlier, independently regenerated finite representation.
		// Through families retain their single existing canonical graph.
		if p.XBlindDepth == 0 && p.ZBlindDepth == 0 {
			return UnequalBoreModel{}, err
		}
		var legacy bytes.Buffer
		if legacyErr := writeUnequalBoreOptions(&legacy, m, false); legacyErr != nil {
			return UnequalBoreModel{}, legacyErr
		}
		if legacyErr := verifyCanonicalCircularExtrusionGraph(f, rep, &legacy); legacyErr != nil {
			return UnequalBoreModel{}, err
		}
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
	if err != nil || (len(faces) < 10 || len(faces) > 12) && (len(faces) < 16 || len(faces) > 18) {
		return fail()
	}
	lower, err := readCircularSourceFace(f, faces[0])
	if err != nil || lower.kind != "PLANE" {
		return fail()
	}
	partial := len(faces) <= 12
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
	p := UnequalBoreBody{Name: name, Frame: frame, Dimensions: bounds.Max, Crossing: crossing, XRadius: x.radius, ZRadius: z.radius, Offset: offset}
	floorStart, rimUses := 16, 4
	if partial {
		floorStart, rimUses = 10, 6
	}
	for _, id := range faces[floorStart:] {
		floor, err := readCircularSourceFace(f, id)
		if err != nil || floor.kind != "PLANE" || floor.sense || len(floor.loops) != 1 || len(floor.loops[0]) != rimUses {
			return fail()
		}
		l := frame.ToLocal(floor.frame.Origin())
		normal := floor.frame.ZAxis()
		switch {
		case normal == frame.XAxis() && p.XBlindDepth == 0:
			p.XBlindDepth = l.X
		case normal == frame.XAxis().Scale(-1) && p.XBlindDepth == 0:
			p.XBlindDepth, p.XOppositeCap = bounds.Max.X-l.X, true
		case normal == frame.ZAxis() && p.ZBlindDepth == 0:
			p.ZBlindDepth = l.Z
		case normal == frame.ZAxis().Scale(-1) && p.ZBlindDepth == 0:
			p.ZBlindDepth, p.ZOppositeCap = bounds.Max.Z-l.Z, true
		default:
			return fail()
		}
	}
	return p, nil
}
