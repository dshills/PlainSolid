package step

import (
	"bytes"
	"context"
	"io"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CrossedObliqueBoreBody retains a cardinal Z bore and a concurrent
// equal-radius oblique bore crossing opposite X stock caps.
type CrossedObliqueBoreBody struct {
	Name                 string
	Frame                geom.Frame
	Dimensions, Crossing geom.Vec3
	Radius               float64
	// AxisRatio zero preserves the legacy 4:3 file representation.
	// The additional supported ratios are (3,4), (12,5) and (5,12).
	AxisRatio geom.Vec2
	// Positive blind depth stops the cardinal bore beyond the intersection.
	BlindDepth  float64
	OppositeCap bool
	// ObliqueFloor is the signed station from the crossing along the
	// positive oblique axis. Its sign identifies the entry cap.
	ObliqueFloor float64
}

type CrossedObliqueBoreModel struct {
	Body       CrossedObliqueBoreBody
	LengthUnit float64
}

func WriteCrossedObliqueBore(w io.Writer, m CrossedObliqueBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var buf bytes.Buffer
	if err := writeCrossedObliqueBore(&buf, m); err != nil {
		return err
	}
	if _, err := ReadCrossedObliqueBore(bytes.NewReader(buf.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &buf)
	return err
}

func writeCrossedObliqueBore(w io.Writer, m CrossedObliqueBoreModel) error {
	p := m.Body
	ctx := context.Background()
	ratio := p.AxisRatio
	if ratio == (geom.Vec2{X: 4, Y: 3}) || p.BlindDepth == 0 && p.OppositeCap {
		return geom.ErrDomain
	}
	if ratio == (geom.Vec2{}) {
		ratio = geom.Vec2{X: 4, Y: 3}
	}
	var r trimmed.BoxCrossedBoresResult
	var err error
	if p.ObliqueFloor != 0 {
		if p.BlindDepth != 0 {
			return geom.ErrDomain
		}
		var e error
		p.ObliqueFloor, e = trimmed.RepresentedObliqueFloorStation(p.Frame, p.Crossing, ratio, p.ObliqueFloor, 1e-9/8)
		if e != nil {
			return e
		}
		r, _, err = trimmed.BoxObliqueFloorCrossedBores(ctx, p.Frame, p.Dimensions, p.Crossing, ratio, p.Radius, p.ObliqueFloor, p.ObliqueFloor < 0, 1e-9)
	} else if p.BlindDepth == 0 {
		r, err = trimmed.BoxCrossedObliqueBores(ctx, p.Frame, p.Dimensions, p.Crossing, ratio, p.Radius, 1e-9)
	} else {
		r, _, err = trimmed.BoxBlindCrossedObliqueBores(ctx, p.Frame, p.Dimensions, p.Crossing, ratio, p.Radius, p.BlindDepth, p.OppositeCap, 1e-9)
	}
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopologyConics(w, m.LengthUnit, p.Name, r.Solid, nativeLongEdgeLimits{23, 96, 64, 2, 4}, false, 0, nil, false, true)
}

func ReadCrossedObliqueBore(r io.Reader, l Limits) (CrossedObliqueBoreModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil || len(ids) != 1 {
		return CrossedObliqueBoreModel{}, circularSourceFailure()
	}
	body, err := recoverCrossedObliqueBore(f, ids[0])
	if err != nil {
		return CrossedObliqueBoreModel{}, err
	}
	m := CrossedObliqueBoreModel{Body: body, LengthUnit: unit}
	var buf bytes.Buffer
	if err := writeCrossedObliqueBore(&buf, m); err != nil {
		return CrossedObliqueBoreModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &buf); err != nil {
		return CrossedObliqueBoreModel{}, err
	}
	return m, nil
}

func recoverCrossedObliqueBore(f *File, id int64) (CrossedObliqueBoreBody, error) {
	fail := func() (CrossedObliqueBoreBody, error) { return CrossedObliqueBoreBody{}, circularSourceFailure() }
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
	ids, err := listRefs(shell.Args[1], 22)
	if err != nil || len(ids) != 22 && len(ids) != 23 {
		return fail()
	}
	z, err := readCircularSourceFaceBounded(f, ids[6], 1, 4)
	if err != nil || z.kind != "CYLINDER" || z.sense {
		return fail()
	}
	b, err := readCircularSourceFaceBounded(f, ids[14], 1, 4)
	if err != nil || b.kind != "CYLINDER" || b.sense || b.radius != z.radius || b.frame.Origin() != z.frame.Origin() {
		return fail()
	}
	y := z.frame.YAxis()
	if math.Abs(y.Dot(b.frame.ZAxis())) > 8*0x1p-52 {
		return fail()
	}
	x, axis := z.frame.XAxis(), z.frame.ZAxis()
	ratio, ok := trimmed.CrossedObliqueAxisRatio(x.Dot(b.frame.ZAxis()), axis.Dot(b.frame.ZAxis()))
	if !ok {
		return fail()
	}
	if ratio == (geom.Vec2{X: 4, Y: 3}) {
		ratio = geom.Vec2{}
	}
	frame, err := geom.NewFrame(geom.Vec3{}, x, axis)
	if err != nil {
		return fail()
	}
	var vertices []geom.Vec3
	for _, faceID := range ids[:6] {
		face, err := readCircularSourceFaceBounded(f, faceID, 2, 4)
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
	if err != nil {
		return fail()
	}
	placed, err := geom.NewFrame(frame.ToWorld(bounds.Min), x, axis)
	if err != nil {
		return fail()
	}
	out := CrossedObliqueBoreBody{Name: name, Frame: placed, Dimensions: bounds.Max.Sub(bounds.Min), Crossing: placed.ToLocal(z.frame.Origin()), Radius: z.radius, AxisRatio: ratio}
	if len(ids) == 23 {
		floor, err := readCircularSourceFaceBounded(f, ids[22], 1, 4)
		if err != nil || floor.kind != "PLANE" || len(floor.loops[0]) != 4 {
			return fail()
		}
		level := placed.ToLocal(floor.frame.Origin()).Z
		switch floor.frame.ZAxis() {
		case axis:
			out.BlindDepth = level
		case axis.Scale(-1):
			out.BlindDepth, out.OppositeCap = out.Dimensions.Z-level, true
		case b.frame.ZAxis():
			loc := b.frame.ToLocal(floor.frame.Origin())
			if math.Hypot(loc.X, loc.Y) > 1e-9/8 || loc.Z == 0 {
				return fail()
			}
			out.ObliqueFloor = loc.Z
		default:
			return fail()
		}
		if out.ObliqueFloor == 0 && out.BlindDepth <= 0 {
			return fail()
		}
	}
	return out, nil
}
