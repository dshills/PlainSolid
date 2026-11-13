package iges

import (
	"context"
	"plainsolid/geom"
)

// NewCurvedOvalSurface creates exact paired full-loop type126 curves on an
// injective biquadratic support, type142 relations and one type144 face.
// Every composed model coefficient and common quarter join must be losslessly
// representable; unsupported rational normalization fails instead of rounding.
func NewCurvedOvalSurface(s *geom.BSplineSurface, outer QuadraticOval, holes []QuadraticOval, unit float64) (CurvedOvalSurface, error) {
	plane, e := unitOvalPlane()
	if e != nil {
		return CurvedOvalSurface{}, e
	}
	m, e := NewQuadraticOvalSurfaceWithHoles(plane, outer, holes, unit)
	if e != nil {
		return CurvedOvalSurface{}, e
	}
	if s == nil {
		return CurvedOvalSurface{}, geom.ErrInvalid
	}
	m.Entities[0] = Entity{Surface: &Surface{Geometry: s, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}}
	d := s.Data()
	polynomial := true
	for _, w := range d.Weights {
		polynomial = polynomial && w == d.Weights[0]
	}
	m.Entities[0].Surface.Polynomial = polynomial
	patch, e := m.curvedOvalSupport(0)
	if e != nil {
		return CurvedOvalSurface{}, e
	}
	for _, c := range m.CurvesOnSurfaces {
		uv := m.Entities[c.ParameterCurve].Spline
		exact, e := geom.NewExactSplineCurve(uv.Curve, curvedOvalOptions())
		if e != nil {
			return CurvedOvalSurface{}, e
		}
		var poles []geom.Vec3
		var weights, knots []float64
		degree := -1
		for q := 0; q < 4; q++ {
			arc, e := exact.Restrict(float64(q), float64(q+1), curvedOvalOptions())
			if e != nil {
				return CurvedOvalSurface{}, e
			}
			arc, e = arc.Reparameterize(0, 1, false, curvedOvalOptions())
			if e != nil {
				return CurvedOvalSurface{}, e
			}
			model, e := patch.ComposeParameterCurve(context.Background(), arc, curvedOvalOptions())
			if e != nil {
				return CurvedOvalSurface{}, e
			}
			curve, e := model.ToBSpline()
			if e != nil {
				return CurvedOvalSurface{}, e
			}
			p, w := curve.Poles(), curve.Weights()
			if degree < 0 {
				degree = curve.Degree()
				for k := 0; k <= degree; k++ {
					knots = append(knots, 0)
				}
			}
			if degree != curve.Degree() || len(p) != degree+1 {
				return CurvedOvalSurface{}, geom.ErrInvalid
			}
			if q > 0 {
				if p[0] != poles[len(poles)-1] || w[0] != weights[len(weights)-1] {
					return CurvedOvalSurface{}, geom.ErrUnrepresentable
				}
				p, w = p[1:], w[1:]
				for k := 0; k < degree; k++ {
					knots = append(knots, float64(q))
				}
			}
			poles = append(poles, p...)
			weights = append(weights, w...)
		}
		for k := 0; k <= degree; k++ {
			knots = append(knots, 4)
		}
		if poles[0] != poles[len(poles)-1] {
			return CurvedOvalSurface{}, geom.ErrInvalid
		}
		curve, e := geom.NewBSpline(degree, poles, knots, weights)
		if e != nil {
			return CurvedOvalSurface{}, e
		}
		m.Entities[c.ModelCurve] = Entity{Spline: &Spline{Curve: curve, Start: 0, End: 4, Closed: true}}
	}
	return CertifyCurvedOvalSurface(m)
}
