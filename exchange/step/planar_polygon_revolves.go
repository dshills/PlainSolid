package step

import (
	"context"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PlanarPolygonRevolveBody retains a model-space meridian and directed axis.
// The exact native resolver requires a coordinate-aligned axis in the polygon's
// coordinate plane. This exchange subset supports one partial revolution.
type PlanarPolygonRevolveBody struct {
	Name                     string
	Points                   []geom.Vec3
	AxisPoint, AxisDirection geom.Vec3
	Angle                    float64
}
type PlanarPolygonRevolveModel struct {
	Bodies     []PlanarPolygonRevolveBody
	LengthUnit float64
}

// WritePlanarPolygonRevolves resolves model-space vertices without projection
// rounding. Axis-contact rectangles use the independently verified cylinder
// sector graph. Other admitted polygons use the existing polygon-revolve codec.
func WritePlanarPolygonRevolves(w io.Writer, m PlanarPolygonRevolveModel) error {
	if w == nil || len(m.Bodies) != 1 {
		return geom.ErrInvalid
	}
	b := m.Bodies[0]
	if !finiteNumber(b.Angle) || b.Angle == 0 || math.Abs(b.Angle) >= 2*math.Pi || len(b.Points) > 32 {
		return geom.ErrDomain
	}
	p, e := trimmed.PreparePlanarPolygonRevolution(context.Background(), b.Points, b.AxisPoint, b.AxisDirection)
	if e != nil {
		return e
	}
	if _, e = trimmed.RevolvePolygon(context.Background(), p.Frame, p.Meridian, b.Angle); e != nil {
		return e
	}
	if r, lo, hi, ok := axisRectangle(p.Meridian); ok {
		height := hi - lo
		exact := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo))
		if !finiteNumber(height) || exact.Cmp(new(big.Rat).SetFloat64(height)) != 0 {
			return geom.ErrUnrepresentable
		}
		origin := p.Frame.ToWorld(geom.Vec3{Z: lo})
		lower := math.Min(0, b.Angle)
		x := p.Frame.XAxis().Scale(math.Cos(lower)).Add(p.Frame.YAxis().Scale(math.Sin(lower)))
		frame, e := geom.NewFrame(origin, x, p.Frame.ZAxis())
		if e != nil {
			return e
		}
		return WriteCylinderSectors(w, CylinderSectorModel{LengthUnit: m.LengthUnit, Bodies: []CylinderSectorBody{{Name: b.Name, Frame: frame, Radius: r, Height: height, Angle: math.Abs(b.Angle)}}})
	}
	contact := false
	for _, q := range p.Meridian {
		contact = contact || q.X == 0
	}
	return WritePolygonRevolves(w, PolygonRevolveModel{LengthUnit: m.LengthUnit, Bodies: []PolygonRevolveBody{{Name: b.Name, Frame: p.Frame, Profile: p.Meridian, Angle: b.Angle, AllowAxisContact: contact}}})
}
func axisRectangle(p []geom.Vec2) (radius, lo, hi float64, ok bool) {
	if len(p) != 4 {
		return
	}
	lo, hi = p[0].Y, p[0].Y
	for _, v := range p {
		radius = math.Max(radius, v.X)
		lo = math.Min(lo, v.Y)
		hi = math.Max(hi, v.Y)
	}
	if radius <= 0 || lo >= hi {
		return
	}
	seen := map[geom.Vec2]bool{}
	for i, v := range p {
		if seen[v] || (v.X != 0 && v.X != radius) || (v.Y != lo && v.Y != hi) {
			return radius, lo, hi, false
		}
		seen[v] = true
		next := p[(i+1)%4]
		if (v.X == next.X) == (v.Y == next.Y) {
			return radius, lo, hi, false
		}
	}
	return radius, lo, hi, true
}

// ReadRectangleRevolves verifies one cylindrical-sector B-rep and returns a
// normalized positive local rectangle revolution. It does not infer original
// model-space axis placement, vertex order or signed construction history.
func ReadRectangleRevolves(r io.Reader, l Limits) (PolygonRevolveModel, error) {
	f, id, unit, e := parseAnalyticCircularExtrusion(r, l)
	if e != nil {
		return PolygonRevolveModel{}, e
	}
	b, e := analyticBody(f, id)
	if e != nil {
		return PolygonRevolveModel{}, e
	}
	faces := b.Solid.Faces()
	if len(faces) != 5 {
		return PolygonRevolveModel{}, geom.ErrDomain
	}
	c, ok := faces[0].Surface.(geom.CylinderSurface)
	if !ok || faces[0].U.Min != 0 || faces[0].V.Min != 0 {
		return PolygonRevolveModel{}, geom.ErrDomain
	}
	frame, radius := c.Parameters()
	height, angle := faces[0].V.Max, faces[0].U.Max
	if height <= 0 || angle <= 0 || angle >= 2*math.Pi {
		return PolygonRevolveModel{}, geom.ErrDomain
	}
	return PolygonRevolveModel{LengthUnit: unit, Bodies: []PolygonRevolveBody{{Name: b.Name, Frame: frame, Profile: []geom.Vec2{{}, {X: radius}, {X: radius, Y: height}, {Y: height}}, Angle: angle, AllowAxisContact: true}}}, nil
}

// RectangleRevolveResolution states a caller-known meridian frame and sweep
// sign. Source chart axes must agree. The frame must yield model-space points
// admitted by the exact coordinate-axis native resolver.
type RectangleRevolveResolution struct {
	Frame    geom.Frame
	Negative bool
}

// ReadPlanarRectangleRevolvesWithResolution restores a verified construction
// placement, with explicit sign. Geometry uses the sector decoder's tolerance;
// the resulting model-space coordinates must also resolve natively exactly.
func ReadPlanarRectangleRevolvesWithResolution(r io.Reader, l Limits, h RectangleRevolveResolution) (PlanarPolygonRevolveModel, error) {
	m, e := ReadRectangleRevolves(r, l)
	if e != nil {
		return PlanarPolygonRevolveModel{}, e
	}
	b := m.Bodies[0]
	radius, _, height, ok := axisRectangle(b.Profile)
	if !ok {
		return PlanarPolygonRevolveModel{}, geom.ErrDomain
	}
	tol := math.Max(radius, height) * 1e-9
	if h.Frame.ZAxis().Distance(b.Frame.ZAxis()) > 1e-10 {
		return PlanarPolygonRevolveModel{}, geom.ErrDomain
	}
	q := h.Frame.ToLocal(b.Frame.Origin())
	if !q.IsFinite() || math.Hypot(q.X, q.Y) > tol || h.Frame.ToWorld(geom.Vec3{Z: q.Z}).Distance(b.Frame.Origin()) > tol {
		return PlanarPolygonRevolveModel{}, geom.ErrDomain
	}
	angle := b.Angle
	if h.Negative {
		angle = -angle
	}
	low := math.Min(0, angle)
	x := h.Frame.XAxis().Scale(math.Cos(low)).Add(h.Frame.YAxis().Scale(math.Sin(low)))
	if x.Distance(b.Frame.XAxis()) > 1e-10 {
		return PlanarPolygonRevolveModel{}, geom.ErrDomain
	}
	high := q.Z + height
	exact := new(big.Rat).Add(new(big.Rat).SetFloat64(q.Z), new(big.Rat).SetFloat64(height))
	if !finiteNumber(high) || exact.Cmp(new(big.Rat).SetFloat64(high)) != 0 {
		return PlanarPolygonRevolveModel{}, geom.ErrUnrepresentable
	}
	points := []geom.Vec3{h.Frame.ToWorld(geom.Vec3{Z: q.Z}), h.Frame.ToWorld(geom.Vec3{X: radius, Z: q.Z}), h.Frame.ToWorld(geom.Vec3{X: radius, Z: high}), h.Frame.ToWorld(geom.Vec3{Z: high})}
	p, e := trimmed.PreparePlanarPolygonRevolution(context.Background(), points, h.Frame.Origin(), h.Frame.ZAxis())
	if e != nil {
		return PlanarPolygonRevolveModel{}, e
	}
	rr, z0, z1, ok := axisRectangle(p.Meridian)
	if !ok || rr != radius || z0 != q.Z || z1 != high {
		return PlanarPolygonRevolveModel{}, geom.ErrUnrepresentable
	}
	return PlanarPolygonRevolveModel{LengthUnit: m.LengthUnit, Bodies: []PlanarPolygonRevolveBody{{Name: b.Name, Points: points, AxisPoint: h.Frame.Origin(), AxisDirection: h.Frame.ZAxis(), Angle: angle}}}, nil
}
