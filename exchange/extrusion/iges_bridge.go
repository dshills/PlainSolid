// Package extrusion bridges a bounded, exact standalone extrusion subset.
// It does not construct trimmed faces or solids.
package extrusion

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

var ErrUnsupportedConversion = errors.New("extrusion: unsupported lossless conversion")
var ErrInconclusiveProof = errors.New("extrusion: injectivity proof inconclusive")

type UnsupportedConversionError struct{ Field, Reason string }

func (e *UnsupportedConversionError) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrUnsupportedConversion, e.Field, e.Reason)
}
func (e *UnsupportedConversionError) Unwrap() error { return ErrUnsupportedConversion }

type InconclusiveProofError struct{ Reason string }

func (e *InconclusiveProofError) Error() string {
	return fmt.Sprintf("%v: %s", ErrInconclusiveProof, e.Reason)
}
func (e *InconclusiveProofError) Unwrap() error { return ErrInconclusiveProof }
func unsupported(field, reason string) error    { return &UnsupportedConversionError{field, reason} }
func rat(x float64) *big.Rat                    { return new(big.Rat).SetFloat64(x) }
func length(v [3]*big.Rat) (*big.Rat, error) {
	q := new(big.Rat)
	for _, x := range v {
		if x == nil {
			return nil, unsupported("direction", "missing ratio")
		}
		q.Add(q, new(big.Rat).Mul(x, x))
	}
	if q.Sign() == 0 {
		return nil, unsupported("direction", "zero displacement")
	}
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(new(big.Int).Set(n), n).Cmp(q.Num()) != 0 || new(big.Int).Mul(new(big.Int).Set(d), d).Cmp(q.Denom()) != 0 {
		return nil, unsupported("distance", "irrational norm")
	}
	return new(big.Rat).SetFrac(n, d), nil
}
func label(s, field string) error {
	if len(s) > 8 || strings.TrimSpace(s) != s {
		return unsupported(field, "requires at most eight ASCII bytes without boundary spaces")
	}
	for _, c := range []byte(s) {
		if c < 32 || c > 126 {
			return unsupported(field, "requires printable ASCII")
		}
	}
	return nil
}
func polynomial(c *geom.BSpline) bool {
	for _, w := range c.Weights() {
		if w != 1 {
			return false
		}
	}
	return true
}
func cloneCurve(c *geom.BSpline) (*geom.BSpline, error) {
	return geom.NewBSpline(c.Degree(), c.Poles(), c.Knots(), c.Weights())
}
func cross(a, b [3]*big.Rat) (r [3]*big.Rat) {
	for i := 0; i < 3; i++ {
		r[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[(i+1)%3], b[(i+2)%3]), new(big.Rat).Mul(a[(i+2)%3], b[(i+1)%3]))
	}
	return
}

// canonicalPlane retains only deterministically reconstructible IGES hints.
// Straight or spatial profiles use Planar=false, Normal=0. A planar profile's
// oriented normal comes from the first noncollinear control triple.
func canonicalPlane(c *geom.BSpline) (bool, geom.Vec3, error) {
	p := c.Poles()
	o := [3]float64{p[0].X, p[0].Y, p[0].Z}
	diff := func(v geom.Vec3) (r [3]*big.Rat) {
		a := [3]float64{v.X, v.Y, v.Z}
		for i := range r {
			r[i] = new(big.Rat).Sub(rat(a[i]), rat(o[i]))
		}
		return
	}
	var first, normal [3]*big.Rat
	have, plane := false, false
	for _, v := range p[1:] {
		q := diff(v)
		if !have {
			if q[0].Sign() != 0 || q[1].Sign() != 0 || q[2].Sign() != 0 {
				first = q
				have = true
			}
			continue
		}
		n := cross(first, q)
		if n[0].Sign() != 0 || n[1].Sign() != 0 || n[2].Sign() != 0 {
			normal = n
			plane = true
			break
		}
	}
	if !plane {
		return false, geom.Vec3{}, nil
	}
	for _, v := range p {
		q := diff(v)
		dot := new(big.Rat)
		for j := range q {
			dot.Add(dot, new(big.Rat).Mul(q[j], normal[j]))
		}
		if dot.Sign() != 0 {
			return false, geom.Vec3{}, nil
		}
	}
	l, e := length(normal)
	if e != nil {
		return false, geom.Vec3{}, unsupported("profile.normal", "unit normal is not rational")
	}
	var n [3]float64
	for i, v := range normal {
		var exact bool
		n[i], exact = new(big.Rat).Quo(v, l).Float64()
		if !exact {
			return false, geom.Vec3{}, unsupported("profile.normal", "unit normal would round")
		}
	}
	return true, geom.Vec3{X: n[0], Y: n[1], Z: n[2]}, nil
}
func prove(s *geom.BSplineSurface) error {
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{MaxPoles: 8192, MaxOperations: 600000, MaxCoefficientBits: 8192})
	if e != nil {
		return &InconclusiveProofError{e.Error()}
	}
	for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		r, e := geom.CertifySurfaceInjectivity(p, geom.SurfaceInjectivityOptions{ProjectionAxes: axes, MaxDepth: 8, MaxCells: 256, MaxOperations: 200000, MaxCoefficientBits: 8192})
		if e == nil && r.ProvenInjective {
			return nil
		}
	}
	return &InconclusiveProofError{"none of three bounded coordinate projections proved global injectivity"}
}

// FromIGES preserves exact terminal displacement through a STEP physical-distance
// parameter: S(u,t)=C(u)+t*(D/|D|), 0<=t<=|D|. Native IGES fraction v maps to
// t=v*|D|. Exact unit ratios and distance must fit the strict STEP representation.
// A global exact injectivity proof establishes nonclosed/non-self-intersecting
// directrix metadata F; inconclusive proofs return ErrInconclusiveProof.
func FromIGES(m iges.Model) (step.RationalLinearExtrusionModel, error) {
	fail := func(field, reason string) (step.RationalLinearExtrusionModel, error) {
		return step.RationalLinearExtrusionModel{}, unsupported(field, reason)
	}
	var check bytes.Buffer
	if e := iges.WriteTabulatedSurfaces(&check, m); e != nil {
		return fail("iges.geometry", e.Error())
	}
	out := step.RationalLinearExtrusionModel{LengthUnit: m.LengthUnit}
	for i, r := range m.TabulatedSurfaces {
		if r.Directrix != i {
			return fail("dependency.order", "expected directrices in root order")
		}
		entity := m.Entities[r.Directrix]
		if r.Subscript != 0 {
			return fail("surface.subscript", "not represented")
		}
		if entity.Subscript != 0 {
			return fail("profile.subscript", "not represented")
		}
		if entity.Form != 0 {
			return fail("profile.form", "requires canonical zero")
		}
		if e := label(r.Label, "surface.name"); e != nil {
			return step.RationalLinearExtrusionModel{}, e
		}
		if e := label(entity.Label, "profile.name"); e != nil {
			return step.RationalLinearExtrusionModel{}, e
		}
		c := entity.Spline.Curve
		planar, normal, e := canonicalPlane(c)
		if e != nil {
			return step.RationalLinearExtrusionModel{}, e
		}
		if entity.Spline.Planar != planar || entity.Spline.Normal != normal {
			return fail("profile.plane", "not canonical exact plane metadata")
		}
		if entity.Spline.Polynomial != polynomial(c) {
			return fail("profile.polynomial", "does not match weights")
		}
		p := c.Poles()[0]
		a, b := [3]float64{p.X, p.Y, p.Z}, [3]float64{r.End.X, r.End.Y, r.End.Z}
		var d [3]*big.Rat
		for j := range d {
			d[j] = new(big.Rat).Sub(rat(b[j]), rat(a[j]))
		}
		norm, e := length(d)
		if e != nil {
			return step.RationalLinearExtrusionModel{}, e
		}
		distance, exact := norm.Float64()
		if !exact {
			return fail("distance", "not float64-exact")
		}
		for j := range d {
			d[j].Quo(d[j], norm)
		}
		surface, e := m.TabulatedGeometry(i)
		if e != nil {
			return fail("geometry", e.Error())
		}
		if e = prove(surface); e != nil {
			return step.RationalLinearExtrusionModel{}, e
		}
		copy, e := cloneCurve(c)
		if e != nil {
			return fail("profile", e.Error())
		}
		s, e := step.NewRationalLinearExtrusionSurface(r.Label, step.SplineCurve{Name: entity.Label, Geometry: copy, Closed: "F", SelfIntersect: "F"}, d, 0, distance)
		if e != nil {
			return fail("step.representation", e.Error())
		}
		out.Surfaces = append(out.Surfaces, s)
	}
	check.Reset()
	if e := step.WriteRationalLinearExtrusions(&check, out); e != nil {
		return fail("step.representation", e.Error())
	}
	return out, nil
}

// ToIGES requires canonical exact unit source ratios, VStart=0 and explicit F
// logical flags, preserving the direction's raw scale rather than dropping it.
// Its type122 terminal point is C(start)+VEnd*direction. No native persistence
// format is added by this bridge; existing STEP codec behavior is unchanged.
func ToIGES(m step.RationalLinearExtrusionModel) (iges.Model, error) {
	if len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return iges.Model{}, unsupported("surface.count", "requires 1–64")
	}
	poles := 0
	for _, s := range m.Surfaces {
		if s.Profile.Geometry == nil {
			return iges.Model{}, unsupported("profile", "missing curve")
		}
		poles += len(s.Profile.Geometry.Poles())
		if poles > 4096 {
			return iges.Model{}, unsupported("profile.count", "cumulative pole limit 4096")
		}
		if e := label(s.Name, "surface.name"); e != nil {
			return iges.Model{}, e
		}
		if e := label(s.Profile.Name, "profile.name"); e != nil {
			return iges.Model{}, e
		}
	}
	var check bytes.Buffer
	if e := step.WriteRationalLinearExtrusions(&check, m); e != nil {
		return iges.Model{}, unsupported("step.geometry", e.Error())
	}
	out := iges.Model{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		if s.VStart != 0 || s.VEnd <= 0 {
			return iges.Model{}, unsupported("domain", "requires VStart=0 and positive VEnd")
		}
		if s.Profile.Closed != "F" || s.Profile.SelfIntersect != "F" {
			return iges.Model{}, unsupported("profile.flags", "requires explicit F")
		}
		if e := label(s.Name, "surface.name"); e != nil {
			return iges.Model{}, e
		}
		if e := label(s.Profile.Name, "profile.name"); e != nil {
			return iges.Model{}, e
		}
		d := s.DirectionRatios()
		norm, e := length(d)
		if e != nil {
			return iges.Model{}, e
		}
		if norm.Cmp(big.NewRat(1, 1)) != 0 {
			return iges.Model{}, unsupported("direction.scale", "requires canonical exact unit ratios")
		}
		surface, e := s.Surface()
		if e != nil {
			return iges.Model{}, unsupported("geometry", e.Error())
		}
		if e = prove(surface); e != nil {
			return iges.Model{}, e
		}
		c, e := cloneCurve(s.Profile.Geometry)
		if e != nil {
			return iges.Model{}, unsupported("profile", e.Error())
		}
		planar, normal, e := canonicalPlane(c)
		if e != nil {
			return iges.Model{}, e
		}
		p := c.Poles()[0]
		a := [3]float64{p.X, p.Y, p.Z}
		for j := range a {
			q := new(big.Rat).Add(rat(a[j]), new(big.Rat).Mul(rat(s.VEnd), d[j]))
			var exact bool
			a[j], exact = q.Float64()
			if !exact {
				return iges.Model{}, unsupported("terminal.point", "would round")
			}
		}
		lo, hi := c.Domain()
		i := len(out.Entities)
		out.Entities = append(out.Entities, iges.Entity{Label: s.Profile.Name, Spline: &iges.Spline{Curve: c, Start: lo, End: hi, Planar: planar, Polynomial: polynomial(c), Normal: normal}})
		out.TabulatedSurfaces = append(out.TabulatedSurfaces, iges.TabulatedSurface{Directrix: i, End: geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, Label: s.Name})
	}
	check.Reset()
	if e := iges.WriteTabulatedSurfaces(&check, out); e != nil {
		return iges.Model{}, unsupported("iges.representation", e.Error())
	}
	return out, nil
}
