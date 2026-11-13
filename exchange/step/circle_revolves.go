package step

import (
	"context"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CircleRevolveBody uses the native meridian-disk convention: its center is
// (MajorRadius, AxialCenter), with signed sweep about Frame.ZAxis().
type CircleRevolveBody struct {
	Name                                         string
	Frame                                        geom.Frame
	MajorRadius, AxialCenter, MinorRadius, Angle float64
}
type CircleRevolveModel struct {
	Bodies     []CircleRevolveBody
	LengthUnit float64
}

// WriteCircleRevolves writes one signed partial or full circular revolution.
// Negative partial sweeps rotate the underlying positive torus chart to the
// lower angular endpoint; axial center translates its origin. STEP material
// topology does not record the original signed feature history.
func WriteCircleRevolves(w io.Writer, m CircleRevolveModel) error {
	if w == nil || len(m.Bodies) != 1 {
		return geom.ErrInvalid
	}
	b := m.Bodies[0]
	if _, e := trimmed.RevolveCircle(context.Background(), b.Frame, b.MajorRadius, b.AxialCenter, b.MinorRadius, b.Angle); e != nil {
		return e
	}
	lower := math.Min(0, b.Angle)
	x := b.Frame.XAxis().Scale(math.Cos(lower)).Add(b.Frame.YAxis().Scale(math.Sin(lower)))
	frame, e := geom.NewFrame(b.Frame.ToWorld(geom.Vec3{Z: b.AxialCenter}), x, b.Frame.ZAxis())
	if e != nil {
		return e
	}
	if math.Abs(b.Angle) == 2*math.Pi {
		return WriteTori(w, TorusModel{LengthUnit: m.LengthUnit, Bodies: []TorusBody{{Name: b.Name, Frame: frame, MajorRadius: b.MajorRadius, MinorRadius: b.MinorRadius}}})
	}
	return WriteTorusElbows(w, TorusElbowModel{LengthUnit: m.LengthUnit, Bodies: []TorusElbowBody{{Name: b.Name, Frame: frame, MajorRadius: b.MajorRadius, MinorRadius: b.MinorRadius, Angle: math.Abs(b.Angle)}}})
}

// ReadCircleRevolves validates the existing complete torus/elbow source graph
// and returns one normalized positive sweep with AxialCenter zero. Original
// frame origin and sweep sign are not inferable from material geometry alone.
func ReadCircleRevolves(r io.Reader, l Limits) (CircleRevolveModel, error) {
	f, id, unit, e := parseAnalyticCircularExtrusion(r, l)
	if e != nil {
		return CircleRevolveModel{}, e
	}
	body, e := analyticBody(f, id)
	if e != nil {
		return CircleRevolveModel{}, e
	}
	faces := body.Solid.Faces()
	if len(faces) != 1 && len(faces) != 3 {
		return CircleRevolveModel{}, geom.ErrDomain
	}
	surface, ok := faces[0].Surface.(geom.TorusSurface)
	if !ok || faces[0].U.Min != 0 || faces[0].V.Min != 0 || faces[0].V.Max != 2*math.Pi {
		return CircleRevolveModel{}, geom.ErrDomain
	}
	frame, major, minor := surface.Parameters()
	return CircleRevolveModel{LengthUnit: unit, Bodies: []CircleRevolveBody{{Name: body.Name, Frame: frame, MajorRadius: major, MinorRadius: minor, Angle: faces[0].U.Max}}}, nil
}

// CircleRevolveResolution supplies a caller-known construction frame and sweep
// direction. It is a verified geometric resolution, not source metadata.
// Frame.ZAxis must agree with the source torus chart axis; for a partial sweep
// its XAxis must select the appropriate starting cap. Full turns have no
// distinguished starting cap, so any radial XAxis is valid.
type CircleRevolveResolution struct {
	Frame    geom.Frame
	Negative bool
}

// ReadCircleRevolvesWithResolution first verifies the entire STEP source, then
// solves the axial offset and checks the supplied signed construction frame.
// Inconsistent radial origins, axes or partial-sweep start directions fail.
// Resolution uses 1e-10 directional tolerance and (major+minor)*1e-10
// positional tolerance, matching the underlying analytic torus decoder.
func ReadCircleRevolvesWithResolution(r io.Reader, l Limits, h CircleRevolveResolution) (CircleRevolveModel, error) {
	m, e := ReadCircleRevolves(r, l)
	if e != nil {
		return m, e
	}
	b := &m.Bodies[0]
	tol := (b.MajorRadius + b.MinorRadius) * 1e-10
	if h.Frame.ZAxis().Sub(b.Frame.ZAxis()).Length() > 1e-10 {
		return CircleRevolveModel{}, geom.ErrDomain
	}
	offset := h.Frame.ToLocal(b.Frame.Origin())
	if !offset.IsFinite() || math.Hypot(offset.X, offset.Y) > tol || h.Frame.ToWorld(geom.Vec3{Z: offset.Z}).Distance(b.Frame.Origin()) > tol {
		return CircleRevolveModel{}, geom.ErrDomain
	}
	angle := b.Angle
	if h.Negative {
		angle = -angle
	}
	lower := math.Min(0, angle)
	x := h.Frame.XAxis().Scale(math.Cos(lower)).Add(h.Frame.YAxis().Scale(math.Sin(lower)))
	if b.Angle != 2*math.Pi && x.Sub(b.Frame.XAxis()).Length() > 1e-10 {
		return CircleRevolveModel{}, geom.ErrDomain
	}
	if _, e = trimmed.RevolveCircle(context.Background(), h.Frame, b.MajorRadius, offset.Z, b.MinorRadius, angle); e != nil {
		return CircleRevolveModel{}, e
	}
	b.Frame, b.AxialCenter, b.Angle = h.Frame, offset.Z, angle
	return m, nil
}
