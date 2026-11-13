package iges

import (
	"math/big"
	"plainsolid/geom"
)

// QuadraticOvalBoundaries accepts at most16 clockwise holes. Containment is
// certified by concentric homothety or strict inclusion of each hole's control
// rectangle in the outer oval's inscribed diamond. Hole rectangles must be
// strictly separated; more general geometrically disjoint arrangements fail.
func (m Model) QuadraticOvalBoundaries(t TrimmedSurface) (QuadraticOval, []QuadraticOval, error) {
	fail := func() (QuadraticOval, []QuadraticOval, error) { return QuadraticOval{}, nil, geom.ErrConvergence }
	if len(t.Inner) > 16 {
		return fail()
	}
	outer, e := m.quadraticOvalContour(t.Surface, t.Outer, false)
	if e != nil {
		return QuadraticOval{}, nil, e
	}
	holes := []QuadraticOval{}
	seen := map[int]bool{t.Outer: true}
	for _, id := range t.Inner {
		if seen[id] {
			return fail()
		}
		seen[id] = true
		h, e := m.quadraticOvalContour(t.Surface, id, true)
		if e != nil {
			return QuadraticOval{}, nil, e
		}
		contained := false
		if h.Center == outer.Center && h.QuarterWeight == outer.QuarterWeight {
			x := new(big.Rat).Quo(rat(h.Radii.X), rat(outer.Radii.X))
			y := new(big.Rat).Quo(rat(h.Radii.Y), rat(outer.Radii.Y))
			contained = x.Cmp(y) == 0 && x.Cmp(big.NewRat(1, 1)) < 0
		}
		if !contained {
			x := new(big.Rat).Add(new(big.Rat).Abs(new(big.Rat).Sub(rat(h.Center.X), rat(outer.Center.X))), rat(h.Radii.X))
			x.Quo(x, rat(outer.Radii.X))
			y := new(big.Rat).Add(new(big.Rat).Abs(new(big.Rat).Sub(rat(h.Center.Y), rat(outer.Center.Y))), rat(h.Radii.Y))
			y.Quo(y, rat(outer.Radii.Y))
			contained = new(big.Rat).Add(x, y).Cmp(big.NewRat(1, 1)) < 0
		}
		if !contained {
			return fail()
		}
		for _, g := range holes {
			dx := new(big.Rat).Abs(new(big.Rat).Sub(rat(h.Center.X), rat(g.Center.X)))
			dy := new(big.Rat).Abs(new(big.Rat).Sub(rat(h.Center.Y), rat(g.Center.Y)))
			if dx.Cmp(new(big.Rat).Add(rat(h.Radii.X), rat(g.Radii.X))) <= 0 && dy.Cmp(new(big.Rat).Add(rat(h.Radii.Y), rat(g.Radii.Y))) <= 0 {
				return fail()
			}
		}
		holes = append(holes, h)
	}
	return outer, holes, nil
}

// NewQuadraticOvalSurfaceWithHoles preserves exact paired model and UV curves;
// holes are emitted clockwise. Unsupported containment fails before output.
func NewQuadraticOvalSurfaceWithHoles(s *geom.BSplineSurface, outer QuadraticOval, holes []QuadraticOval, unit float64) (Model, error) {
	if len(holes) > 16 {
		return Model{}, geom.ErrInvalid
	}
	m, e := NewQuadraticOvalSurface(s, outer, unit)
	if e != nil {
		return Model{}, e
	}
	for _, h := range holes {
		child, e := NewQuadraticOvalSurface(s, h, unit)
		if e != nil {
			return Model{}, e
		}
		start := len(m.Entities)
		for _, entity := range child.Entities[1:] {
			sp := *entity.Spline
			p := sp.Curve.Poles()
			w := sp.Curve.Weights()
			for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
				w[i], w[j] = w[j], w[i]
			}
			sp.Curve, e = geom.NewBSpline(2, p, sp.Curve.Knots(), w)
			if e != nil {
				return Model{}, e
			}
			entity.Spline = &sp
			m.Entities = append(m.Entities, entity)
		}
		m.TrimmedSurfaces[0].Inner = append(m.TrimmedSurfaces[0].Inner, len(m.CurvesOnSurfaces))
		m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, CurveOnSurface{Surface: 0, ParameterCurve: start, ModelCurve: start + 1, Preference: 3})
	}
	if _, _, e = m.QuadraticOvalBoundaries(m.TrimmedSurfaces[0]); e != nil {
		return Model{}, e
	}
	return m, nil
}
