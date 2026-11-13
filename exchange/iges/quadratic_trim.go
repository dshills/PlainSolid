package iges

import (
	"fmt"
	"math/big"
	"plainsolid/geom"
)

// QuadraticOval is a simple CCW four-quarter rational profile in UV space.
// QuarterWeight is positive and at most one; the profile is not generally an
// exact ellipse. No circularity is inferred from a rounded sqrt(1/2) weight.
type QuadraticOval struct {
	Center, Radii geom.Vec2
	QuarterWeight float64
}

func ovalPoles(q QuadraticOval) ([]geom.Vec3, error) {
	if !(geom.Vec3{X: q.Center.X, Y: q.Center.Y}).IsFinite() || !(geom.Vec3{X: q.Radii.X, Y: q.Radii.Y, Z: q.QuarterWeight}).IsFinite() || q.Radii.X <= 0 || q.Radii.Y <= 0 || q.QuarterWeight <= 0 || q.QuarterWeight > 1 {
		return nil, geom.ErrInvalid
	}
	out := make([]geom.Vec3, 9)
	for i, p := range [][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}} {
		x, e := geom.RepresentedExactScalar(new(big.Rat).Add(rat(q.Center.X), new(big.Rat).Mul(rat(q.Radii.X), big.NewRat(p[0], 1))))
		if e != nil {
			return nil, e
		}
		y, e := geom.RepresentedExactScalar(new(big.Rat).Add(rat(q.Center.Y), new(big.Rat).Mul(rat(q.Radii.Y), big.NewRat(p[1], 1))))
		if e != nil {
			return nil, e
		}
		out[i] = geom.Vec3{X: x, Y: y}
	}
	return out, nil
}

// QuadraticOvalBoundary certifies one closed positive rational quadratic loop on
// an affine support. Four disjoint open quadrants prove simplicity and winding;
// each arc has positive weights and monotone coordinates. Holes are unsupported.
func (m Model) QuadraticOvalBoundary(t TrimmedSurface) (QuadraticOval, error) {
	if len(t.Inner) != 0 {
		return QuadraticOval{}, geom.ErrInvalid
	}
	return m.quadraticOvalContour(t.Surface, t.Outer, false)
}
func (m Model) quadraticOvalContour(surfaceID, contourID int, reverse bool) (QuadraticOval, error) {
	fail := func() (QuadraticOval, error) {
		return QuadraticOval{}, fmt.Errorf("iges: unsupported quadratic oval trim")
	}
	if contourID < 0 || contourID >= len(m.CurvesOnSurfaces) {
		return fail()
	}
	c := m.CurvesOnSurfaces[contourID]
	if c.Surface != surfaceID {
		return fail()
	}
	if e := m.ValidateCurveOnSurface(c); e != nil {
		return QuadraticOval{}, e
	}
	uv := m.Entities[c.ParameterCurve].Spline
	if uv == nil {
		return fail()
	}
	p, k, w := uv.Curve.Poles(), uv.Curve.Knots(), uv.Curve.Weights()
	if uv.Curve.Degree() != 2 || len(p) != 9 || len(k) != 12 || len(w) != 9 || uv.Start != 0 || uv.End != 4 {
		return fail()
	}
	for i, v := range []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4} {
		if k[i] != v {
			return fail()
		}
	}
	if reverse {
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
			w[i], w[j] = w[j], w[i]
		}
	}
	x, e := geom.RepresentedExactScalar(new(big.Rat).Quo(new(big.Rat).Add(rat(p[0].X), rat(p[4].X)), big.NewRat(2, 1)))
	if e != nil {
		return QuadraticOval{}, e
	}
	y, e := geom.RepresentedExactScalar(new(big.Rat).Quo(new(big.Rat).Add(rat(p[2].Y), rat(p[6].Y)), big.NewRat(2, 1)))
	if e != nil {
		return QuadraticOval{}, e
	}
	rx, e := geom.RepresentedExactScalar(new(big.Rat).Sub(rat(p[0].X), rat(x)))
	if e != nil {
		return QuadraticOval{}, e
	}
	ry, e := geom.RepresentedExactScalar(new(big.Rat).Sub(rat(p[2].Y), rat(y)))
	if e != nil {
		return QuadraticOval{}, e
	}
	q := QuadraticOval{Center: geom.Vec2{X: x, Y: y}, Radii: geom.Vec2{X: rx, Y: ry}, QuarterWeight: w[1]}
	want, e := ovalPoles(q)
	if e != nil {
		return QuadraticOval{}, e
	}
	surface := m.Entities[surfaceID].Surface
	for i, v := range p {
		if v != want[i] || i%2 == 0 && w[i] != 1 || i%2 == 1 && w[i] != q.QuarterWeight {
			return fail()
		}
		if v.X <= surface.UStart || v.X >= surface.UEnd || v.Y <= surface.VStart || v.Y >= surface.VEnd {
			return fail()
		}
	}
	return q, nil
}

// NewQuadraticOvalSurface constructs paired UV/model type126 curves, a type142
// relation and type144 trimmed face. Lifting must be exactly representable.
func NewQuadraticOvalSurface(s *geom.BSplineSurface, q QuadraticOval, unit float64) (Model, error) {
	unitOK := false
	for _, u := range units {
		unitOK = unitOK || unit == u.meters
	}
	if !unitOK {
		return Model{}, geom.ErrInvalid
	}
	p, e := ovalPoles(q)
	if e != nil {
		return Model{}, e
	}
	if s == nil {
		return Model{}, geom.ErrInvalid
	}
	d := s.Data()
	if len(d.Poles) != 4 {
		return Model{}, geom.ErrInvalid
	}
	model := make([]geom.Vec3, len(p))
	origin := d.Poles[0]
	a, b := d.Poles[2].Sub(origin), d.Poles[1].Sub(origin)
	for i, v := range p {
		coords := []float64{origin.X, origin.Y, origin.Z}
		ax, by := []float64{a.X, a.Y, a.Z}, []float64{b.X, b.Y, b.Z}
		for j := range coords {
			coords[j], e = geom.RepresentedExactScalar(new(big.Rat).Add(rat(coords[j]), new(big.Rat).Add(new(big.Rat).Mul(rat(ax[j]), rat(v.X)), new(big.Rat).Mul(rat(by[j]), rat(v.Y)))))
			if e != nil {
				return Model{}, e
			}
		}
		model[i] = geom.Vec3{X: coords[0], Y: coords[1], Z: coords[2]}
	}
	knots := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	weights := []float64{1, q.QuarterWeight, 1, q.QuarterWeight, 1, q.QuarterWeight, 1, q.QuarterWeight, 1}
	uv, e := geom.NewBSpline(2, p, knots, weights)
	if e != nil {
		return Model{}, e
	}
	curve, e := geom.NewBSpline(2, model, knots, weights)
	if e != nil {
		return Model{}, e
	}
	normal, e := a.Cross(b).Unit()
	if e != nil {
		return Model{}, e
	}
	m := Model{LengthUnit: unit, Entities: []Entity{{Surface: &Surface{Geometry: s, Polynomial: true, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}}, {Spline: &Spline{Curve: uv, Start: 0, End: 4, Closed: true, Planar: true, Normal: geom.Vec3{Z: 1}}}, {Spline: &Spline{Curve: curve, Start: 0, End: 4, Closed: true, Planar: true, Normal: normal}}}, CurvesOnSurfaces: []CurveOnSurface{{Surface: 0, ParameterCurve: 1, ModelCurve: 2, Preference: 3}}, TrimmedSurfaces: []TrimmedSurface{{Surface: 0, Outer: 0}}}
	if _, e = m.QuadraticOvalBoundary(m.TrimmedSurfaces[0]); e != nil {
		return Model{}, e
	}
	return m, nil
}
