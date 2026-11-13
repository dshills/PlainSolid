package iges

import (
	"errors"
	"math/big"
	"plainsolid/geom"
)

// TrimmedSurface retains type144 explicit outer and optional inner contour
// references. Surface indexes Entities; contour indices index CurvesOnSurfaces.
type TrimmedSurface struct {
	Surface, Outer int
	Inner          []int
	Label          string
	Subscript      int
}

// MaxTrimVertices bounds total polygon vertices per trimmed surface.
const MaxTrimVertices = 256

func uvOn(a, b, p geom.Vec2) bool {
	o, _ := geom.Orient2D(a, b, p)
	return o == 0 && p.X >= min(a.X, b.X) && p.X <= max(a.X, b.X) && p.Y >= min(a.Y, b.Y) && p.Y <= max(a.Y, b.Y)
}
func uvMeet(a, b, c, d geom.Vec2) bool {
	x, _ := geom.Orient2D(a, b, c)
	y, _ := geom.Orient2D(a, b, d)
	z, _ := geom.Orient2D(c, d, a)
	w, _ := geom.Orient2D(c, d, b)
	return x*y < 0 && z*w < 0 || uvOn(a, b, c) || uvOn(a, b, d) || uvOn(c, d, a) || uvOn(c, d, b)
}
func uvInside(loop []geom.Vec2, p geom.Vec2) bool {
	w := 0
	for i, a := range loop {
		b := loop[(i+1)%len(loop)]
		if uvOn(a, b, p) {
			return false
		}
		o, _ := geom.Orient2D(a, b, p)
		if a.Y <= p.Y && b.Y > p.Y && o > 0 {
			w++
		}
		if a.Y > p.Y && b.Y <= p.Y && o < 0 {
			w--
		}
	}
	return w != 0
}
func trimSign(loop []geom.Vec2) int {
	sum := new(big.Rat)
	for i, p := range loop {
		q := loop[(i+1)%len(loop)]
		sum.Add(sum, new(big.Rat).Sub(new(big.Rat).Mul(rat(p.X), rat(q.Y)), new(big.Rat).Mul(rat(p.Y), rat(q.X))))
	}
	return sum.Sign()
}

// ValidateTrimmedSurface proves polygon loop simplicity, orientation, strict
// containment and disjoint holes using exact orientation predicates. Contours
// are either full-domain degree1 planar splines or paired orthogonal composites
// on an injective spline support; curved trims are not approximated.
func (m Model) ValidateTrimmedSurface(t TrimmedSurface) error {
	fail := func() error { return errors.New("iges: unsupported or invalid polygon trimmed surface") }
	if t.Surface < 0 || t.Surface >= len(m.Entities) || len(t.Label) > 8 || t.Subscript < 0 || t.Subscript > 99999999 {
		return fail()
	}
	for _, b := range []byte(t.Label) {
		if b < 32 || b > 126 {
			return fail()
		}
	}
	if len(t.Inner) > MaxTrimVertices/3-1 {
		return fail()
	}
	if t.Outer >= 0 && t.Outer < len(m.CurvesOnSurfaces) {
		c := m.CurvesOnSurfaces[t.Outer]
		if c.ParameterCurve >= 0 && c.ParameterCurve < len(m.Entities) && m.Entities[c.ParameterCurve].Composite != nil {
			_, _, err := m.orthogonalTrimLoops(t)
			return err
		}
	}
	if t.Outer >= 0 && t.Outer < len(m.CurvesOnSurfaces) {
		c := m.CurvesOnSurfaces[t.Outer]
		if c.ParameterCurve >= 0 && c.ParameterCurve < len(m.Entities) {
			spline := m.Entities[c.ParameterCurve].Spline
			if spline != nil && spline.Curve != nil && spline.Curve.Degree() == 2 {
				_, _, e := m.QuadraticOvalBoundaries(t)
				return e
			}
		}
	}
	ids := append([]int{t.Outer}, t.Inner...)
	loops := make([][]geom.Vec2, len(ids))
	seen := map[int]bool{}
	count := 0
	for k, id := range ids {
		if id < 0 || id >= len(m.CurvesOnSurfaces) || seen[id] {
			return fail()
		}
		seen[id] = true
		c := m.CurvesOnSurfaces[id]
		if c.Surface != t.Surface {
			return fail()
		}
		if e := m.ValidateCurveOnSurface(c); e != nil {
			return e
		}
		s := m.Entities[c.ParameterCurve].Spline
		a, b := s.Curve.Domain()
		p := s.Curve.Poles()
		knots := s.Curve.Knots()
		if s.Curve.Degree() != 1 || s.Start != a || s.End != b || len(p) < 4 || len(p) > MaxTrimVertices+1 || p[0] != p[len(p)-1] || len(knots) != len(p)+2 || knots[0] != knots[1] || knots[len(knots)-1] != knots[len(knots)-2] {
			return fail()
		}
		for i := 2; i < len(knots)-1; i++ {
			if knots[i] <= knots[i-1] {
				return fail()
			}
		}
		count += len(p) - 1
		if count > MaxTrimVertices {
			return fail()
		}
		loop := make([]geom.Vec2, len(p)-1)
		for i := range loop {
			loop[i] = geom.Vec2{X: p[i].X, Y: p[i].Y}
		}
		for i, a := range loop {
			b, c := loop[(i+1)%len(loop)], loop[(i+2)%len(loop)]
			o, e := geom.Orient2D(a, b, c)
			if e != nil || o == 0 {
				return fail()
			}
			for j := i + 1; j < len(loop); j++ {
				if j == i+1 || i == 0 && j == len(loop)-1 {
					continue
				}
				if uvMeet(a, b, loop[j], loop[(j+1)%len(loop)]) {
					return fail()
				}
			}
		}
		sign := trimSign(loop)
		if k == 0 && sign != 1 || k > 0 && sign != -1 {
			return fail()
		}
		loops[k] = loop
	}
	for i := 1; i < len(loops); i++ {
		if !uvInside(loops[0], loops[i][0]) {
			return fail()
		}
		for j := 0; j < i; j++ {
			for a, p := range loops[i] {
				q := loops[i][(a+1)%len(loops[i])]
				for b, r := range loops[j] {
					if uvMeet(p, q, r, loops[j][(b+1)%len(loops[j])]) {
						return fail()
					}
				}
			}
			if j > 0 && (uvInside(loops[j], loops[i][0]) || uvInside(loops[i], loops[j][0])) {
				return fail()
			}
		}
	}
	return nil
}
