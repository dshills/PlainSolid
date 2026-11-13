package iges

import (
	"errors"
	"plainsolid/geom"
)

const MaxOrthogonalVertices = 64

func validateOrthogonalPolygon(p []geom.Vec2) error {
	fail := func() error { return errors.New("iges: invalid orthogonal UV polygon") }
	if len(p) < 4 || len(p) > MaxOrthogonalVertices || len(p)%2 != 0 {
		return fail()
	}
	for _, a := range p {
		if !finite(a.X) || !finite(a.Y) {
			return fail()
		}
	}
	for i, a := range p {
		b, c := p[(i+1)%len(p)], p[(i+2)%len(p)]
		if (a.X == b.X) == (a.Y == b.Y) || (a.X == b.X) == (b.X == c.X) {
			return fail()
		}
		for j := i + 1; j < len(p); j++ {
			if j == i+1 || i == 0 && j == len(p)-1 {
				continue
			}
			if orthogonalSegmentsTouch(a, b, p[j], p[(j+1)%len(p)]) {
				return fail()
			}
		}
	}
	if trimSign(p) == 0 {
		return fail()
	}
	return nil
}
func orthogonalSegmentsTouch(a, b, c, d geom.Vec2) bool {
	return max(min(a.X, b.X), min(c.X, d.X)) <= min(max(a.X, b.X), max(c.X, d.X)) && max(min(a.Y, b.Y), min(c.Y, d.Y)) <= min(max(a.Y, b.Y), max(c.Y, d.Y))
}
func orthogonalLoopsTouch(a, b []geom.Vec2) bool {
	for i, p := range a {
		for j, q := range b {
			if orthogonalSegmentsTouch(p, a[(i+1)%len(a)], q, b[(j+1)%len(b)]) {
				return true
			}
		}
	}
	return false
}
func insideOrthogonal(p geom.Vec2, loop []geom.Vec2) bool {
	in := false
	for i, a := range loop {
		b := loop[(i+1)%len(loop)]
		if (a.Y > p.Y) != (b.Y > p.Y) && a.X > p.X {
			in = !in
		}
	}
	return in
}
func validateOrthogonalLoops(outer []geom.Vec2, holes [][]geom.Vec2) error {
	if e := validateOrthogonalPolygon(outer); e != nil {
		return e
	}
	if trimSign(outer) != 1 || len(holes) > MaxRectangularHoles {
		return errors.New("iges: outer sense or hole count")
	}
	for i, h := range holes {
		if e := validateOrthogonalPolygon(h); e != nil {
			return e
		}
		if trimSign(h) != -1 || orthogonalLoopsTouch(outer, h) || !insideOrthogonal(h[0], outer) {
			return errors.New("iges: hole sense or containment")
		}
		for _, q := range holes[:i] {
			if orthogonalLoopsTouch(h, q) || insideOrthogonal(h[0], q) || insideOrthogonal(q[0], h) {
				return errors.New("iges: holes overlap, nest or touch")
			}
		}
	}
	return nil
}
func (m Model) orthogonalTrimLoops(t TrimmedSurface) ([]geom.Vec2, [][]geom.Vec2, error) {
	fail := func() ([]geom.Vec2, [][]geom.Vec2, error) {
		return nil, nil, errors.New("iges: invalid orthogonal trimmed surface")
	}
	if t.Outer < 0 || t.Outer >= len(m.CurvesOnSurfaces) || len(t.Inner) > MaxRectangularHoles {
		return fail()
	}
	c := m.CurvesOnSurfaces[t.Outer]
	if c.Surface != t.Surface {
		return fail()
	}
	if e := m.ValidateCurveOnSurface(c); e != nil {
		return nil, nil, e
	}
	outer, e := m.orthogonalContourSense(c, 1)
	if e != nil {
		return nil, nil, e
	}
	seen := map[int]bool{t.Outer: true}
	holes := [][]geom.Vec2{}
	for _, id := range t.Inner {
		if id < 0 || id >= len(m.CurvesOnSurfaces) || seen[id] {
			return fail()
		}
		seen[id] = true
		c := m.CurvesOnSurfaces[id]
		if c.Surface != t.Surface {
			return fail()
		}
		if e := m.ValidateCurveOnSurface(c); e != nil {
			return nil, nil, e
		}
		h, e := m.orthogonalContourSense(c, -1)
		if e != nil {
			return nil, nil, e
		}
		holes = append(holes, h)
	}
	if e := validateOrthogonalLoops(outer, holes); e != nil {
		return nil, nil, e
	}
	return outer, holes, nil
}

// OrthogonalBoundaries returns certified source traversal: counterclockwise
// outer and clockwise holes. Returned slices are independent of model storage.
func (m Model) OrthogonalBoundaries(t TrimmedSurface) ([]geom.Vec2, [][]geom.Vec2, error) {
	if e := m.ValidateTrimmedSurface(t); e != nil {
		return nil, nil, e
	}
	return m.orthogonalTrimLoops(t)
}

// NewOrthogonalSurface retains exact support and UV coordinates. The outer
// polygon is counterclockwise and holes clockwise. Up to 16 disjoint holes and
// 4–64 alternating iso edges per polygon are supported, including concavity.
func NewOrthogonalSurface(s *geom.BSplineSurface, outer []geom.Vec2, holes [][]geom.Vec2, lengthUnit float64) (Model, error) {
	if e := validateOrthogonalLoops(outer, holes); e != nil {
		return Model{}, e
	}
	unitOK := false
	for _, u := range units {
		unitOK = unitOK || lengthUnit == u.meters
	}
	if !unitOK {
		return Model{}, errors.New("iges: unsupported length unit")
	}
	d := geom.ParameterRectangle{UMin: outer[0].X, UMax: outer[0].X, VMin: outer[0].Y, VMax: outer[0].Y}
	for _, v := range outer {
		d.UMin = min(d.UMin, v.X)
		d.UMax = max(d.UMax, v.X)
		d.VMin = min(d.VMin, v.Y)
		d.VMax = max(d.VMax, v.Y)
	}
	patch, e := rectanglePatch(s, d)
	if e != nil {
		return Model{}, e
	}
	m := Model{LengthUnit: lengthUnit, Entities: []Entity{{Surface: &Surface{Geometry: s, UStart: d.UMin, UEnd: d.UMax, VStart: d.VMin, VEnd: d.VMax}}}}
	trim := TrimmedSurface{Surface: 0, Outer: 0}
	for k, loop := range append([][]geom.Vec2{outer}, holes...) {
		uvIDs, modelIDs := []int{}, []int{}
		for i, a := range loop {
			b := loop[(i+1)%len(loop)]
			uvIDs = append(uvIDs, len(m.Entities))
			m.Entities = append(m.Entities, Entity{Line: &Line{Start: geom.Vec3{X: a.X, Y: a.Y}, End: geom.Vec3{X: b.X, Y: b.Y}}})
			axis, fixed, lo, hi, reverse := 0, a.X, min(a.Y, b.Y), max(a.Y, b.Y), b.Y < a.Y
			if a.Y == b.Y {
				axis, fixed, lo, hi, reverse = 1, a.Y, min(a.X, b.X), max(a.X, b.X), b.X < a.X
			}
			curve, e := patch.IsoCurve(axis, fixed, rectangleOptions)
			if e != nil {
				return Model{}, e
			}
			curve, e = curve.Restrict(lo, hi, rectangleOptions)
			if e != nil {
				return Model{}, e
			}
			if reverse {
				curve, e = curve.Reparameterize(lo, hi, true, rectangleOptions)
				if e != nil {
					return Model{}, e
				}
			}
			spline, e := curve.ToBSpline()
			if e != nil {
				return Model{}, e
			}
			modelIDs = append(modelIDs, len(m.Entities))
			m.Entities = append(m.Entities, Entity{Spline: &Spline{Curve: spline, Start: lo, End: hi}})
		}
		ui := len(m.Entities)
		m.Entities = append(m.Entities, Entity{Composite: &CompositeCurve{Curves: uvIDs}})
		mi := len(m.Entities)
		m.Entities = append(m.Entities, Entity{Composite: &CompositeCurve{Curves: modelIDs}})
		m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, CurveOnSurface{Surface: 0, ParameterCurve: ui, ModelCurve: mi, Preference: 3})
		if k > 0 {
			trim.Inner = append(trim.Inner, k)
		}
	}
	m.TrimmedSurfaces = []TrimmedSurface{trim}
	if e = m.ValidateTrimmedSurface(trim); e != nil {
		return Model{}, e
	}
	return m, nil
}
