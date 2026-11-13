package revolution

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"strings"
)

// ErrUnsupportedConversion identifies a conversion outside the strict lossless
// metadata/geometry bridge subset, distinct from accepting and dropping fields.
var ErrUnsupportedConversion = errors.New("revolution: unsupported lossless conversion")

// UnsupportedConversionError identifies the source field or representability
// gate that prevents conversion. errors.Is recognizes ErrUnsupportedConversion.
type UnsupportedConversionError struct{ Field, Reason string }

func (e *UnsupportedConversionError) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrUnsupportedConversion, e.Field, e.Reason)
}
func (e *UnsupportedConversionError) Unwrap() error { return ErrUnsupportedConversion }
func unsupported(field, reason string) error        { return &UnsupportedConversionError{field, reason} }
func bridgeRat(x float64) *big.Rat                  { return new(big.Rat).SetFloat64(x) }
func bridgePolynomial(c *geom.BSpline) bool {
	for _, w := range c.Weights() {
		if w != 1 {
			return false
		}
	}
	return true
}

// The canonical normal is radial cross positive-coordinate-axis. It deliberately
// does not depend on the sense of revolution or an approximately normalized frame.
func bridgeNormal(s step.RevolutionSurface) (geom.Vec3, error) {
	g, e := s.Surface()
	if e != nil {
		return geom.Vec3{}, unsupported("geometry", e.Error())
	}
	origin, axis := g.Axis()
	p := s.Profile.Geometry.Poles()[0]
	o := [3]float64{origin.X, origin.Y, origin.Z}
	a := [3]float64{axis.X, axis.Y, axis.Z}
	q := [3]float64{p.X, p.Y, p.Z}
	var r [3]*big.Rat
	ax := -1
	for i := 0; i < 3; i++ {
		r[i] = new(big.Rat).Sub(bridgeRat(q[i]), bridgeRat(o[i]))
		if a[i] != 0 {
			ax = i
		}
	}
	r[ax].SetInt64(0)
	n := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	n[(ax+1)%3].Set(r[(ax+2)%3])
	n[(ax+2)%3].Neg(r[(ax+1)%3])
	norm := new(big.Rat)
	for _, v := range n {
		norm.Add(norm, new(big.Rat).Mul(v, v))
	}
	num := new(big.Int).Sqrt(norm.Num())
	den := new(big.Int).Sqrt(norm.Denom())
	if new(big.Int).Mul(new(big.Int).Set(num), num).Cmp(norm.Num()) != 0 || new(big.Int).Mul(new(big.Int).Set(den), den).Cmp(norm.Denom()) != 0 {
		return geom.Vec3{}, unsupported("profile.normal", "unit normal is not rational")
	}
	length := new(big.Rat).SetFrac(num, den)
	var out [3]float64
	for i, v := range n {
		x := new(big.Rat).Quo(v, length)
		var exact bool
		out[i], exact = x.Float64()
		if !exact {
			return geom.Vec3{}, unsupported("profile.normal", "unit normal is not float64-exact")
		}
	}
	return geom.Vec3{X: out[0], Y: out[1], Z: out[2]}, nil
}
func bridgeAxisEnd(origin, axis geom.Vec3) (geom.Vec3, error) {
	o := [3]float64{origin.X, origin.Y, origin.Z}
	a := [3]float64{axis.X, axis.Y, axis.Z}
	var end [3]float64
	for i := range o {
		q := new(big.Rat).Add(bridgeRat(o[i]), bridgeRat(a[i]))
		var exact bool
		end[i], exact = q.Float64()
		if !exact {
			return geom.Vec3{}, unsupported("axis.end", "unit endpoint is not float64-exact")
		}
	}
	return geom.Vec3{X: end[0], Y: end[1], Z: end[2]}, nil
}
func bridgeLabel(name, field string) error {
	if strings.TrimSpace(name) != name {
		return unsupported(field, "IGES directory labels cannot preserve boundary spaces")
	}
	if len(name) > 8 {
		return unsupported(field, "IGES label exceeds eight bytes")
	}
	for _, c := range []byte(name) {
		if c < 32 || c > 126 {
			return unsupported(field, "IGES label requires printable ASCII")
		}
	}
	return nil
}

// FromIGES converts the strictly owned type120 family without discarding metadata.
// It requires axis/profile pairs in root order, empty axis labels, zero forms and
// subscripts, and deterministic planar/polynomial/normal metadata. Labels become
// native/STEP names. Strict continuous axial monotonicity proves the generatrix
// is neither closed nor self-intersecting, so its STEP logical flags become F.
func FromIGES(m iges.Model) (step.RevolutionSurfaceModel, error) {
	fail := func(field, reason string) (step.RevolutionSurfaceModel, error) {
		return step.RevolutionSurfaceModel{}, unsupported(field, reason)
	}
	var check bytes.Buffer
	if e := iges.WriteRevolutionSurfaces(&check, m); e != nil {
		return fail("iges.geometry", e.Error())
	}
	out := step.RevolutionSurfaceModel{LengthUnit: m.LengthUnit}
	for i, r := range m.Revolutions {
		if r.Axis != 2*i || r.Generatrix != 2*i+1 {
			return fail("dependency.order", "expected axis/profile pairs in root order")
		}
		a, p := m.Entities[r.Axis], m.Entities[r.Generatrix]
		if e := bridgeLabel(r.Label, "surface.name"); e != nil {
			return step.RevolutionSurfaceModel{}, e
		}
		if e := bridgeLabel(p.Label, "profile.name"); e != nil {
			return step.RevolutionSurfaceModel{}, e
		}
		if a.Label != "" {
			return fail("axis.label", "STEP support has no axis-label field")
		}
		if a.Subscript != 0 {
			return fail("axis.subscript", "not represented")
		}
		if p.Subscript != 0 {
			return fail("profile.subscript", "not represented")
		}
		if r.Subscript != 0 {
			return fail("surface.subscript", "not represented")
		}
		if a.Form != 0 || p.Form != 0 {
			return fail("directory.form", "only canonical form zero is represented")
		}
		g, e := m.RevolutionGeometry(i)
		if e != nil {
			return fail("geometry", e.Error())
		}
		origin, axis := g.Axis()
		s := step.RevolutionSurface{Name: r.Label, Profile: step.SplineCurve{Name: p.Label, Geometry: g.Profile(), Closed: "F", SelfIntersect: "F"}, Origin: origin, Axis: axis, AngleStart: r.AngleStart, AngleEnd: r.AngleEnd}
		normal, e := bridgeNormal(s)
		if e != nil {
			return step.RevolutionSurfaceModel{}, e
		}
		if !p.Spline.Planar {
			return fail("profile.planar", "canonical bridge requires true")
		}
		if p.Spline.Normal != normal {
			return fail("profile.normal", "source differs from exact canonical meridian normal")
		}
		if p.Spline.Polynomial != bridgePolynomial(s.Profile.Geometry) {
			return fail("profile.polynomial", "source differs from canonical weight classification")
		}
		out.Surfaces = append(out.Surfaces, s)
	}
	return out, nil
}

// ToIGES converts native/STEP revolution supports to the reversible type120
// bridge subset. Unknown/empty logical flags, long/non-ASCII names, unsupported
// units or rounded IGES scalar output fail explicitly. Native geometry is copied.
func ToIGES(m step.RevolutionSurfaceModel) (iges.Model, error) {
	l, _ := (RevolutionSurfaceJSONLimits{}).defaults()
	if _, e := revolutionSurfacePayloadVersion(m, l, 2); e != nil {
		return iges.Model{}, unsupported("geometry", e.Error())
	}
	out := iges.Model{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		if s.Profile.Closed != "F" {
			return iges.Model{}, unsupported("profile.closed", "requires explicit F")
		}
		if s.Profile.SelfIntersect != "F" {
			return iges.Model{}, unsupported("profile.self_intersect", "requires explicit F")
		}
		if e := bridgeLabel(s.Name, "surface.name"); e != nil {
			return iges.Model{}, e
		}
		if e := bridgeLabel(s.Profile.Name, "profile.name"); e != nil {
			return iges.Model{}, e
		}
		normal, e := bridgeNormal(s)
		if e != nil {
			return iges.Model{}, e
		}
		end, e := bridgeAxisEnd(s.Origin, s.Axis)
		if e != nil {
			return iges.Model{}, e
		}
		g, _ := s.Surface()
		c := g.Profile()
		lo, hi := c.Domain()
		i := len(out.Entities)
		out.Entities = append(out.Entities, iges.Entity{Line: &iges.Line{Start: s.Origin, End: end}}, iges.Entity{Label: s.Profile.Name, Spline: &iges.Spline{Curve: c, Start: lo, End: hi, Planar: true, Polynomial: bridgePolynomial(c), Normal: normal}})
		out.Revolutions = append(out.Revolutions, iges.RevolutionSurface{Axis: i, Generatrix: i + 1, AngleStart: s.AngleStart, AngleEnd: s.AngleEnd, Label: s.Name})
	}
	var check bytes.Buffer
	if e := iges.WriteRevolutionSurfaces(&check, out); e != nil {
		return iges.Model{}, unsupported("iges.representation", e.Error())
	}
	return out, nil
}
