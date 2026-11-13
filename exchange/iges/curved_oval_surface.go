package iges

import (
	"context"
	"io"
	"plainsolid/geom"
)

// CurvedOvalSurface is one injective nonplanar spline face with a certified
// rational quadratic outer oval and optional clockwise disjoint oval holes.
// It preserves its IGES support and paired UV/model multispan curves. It does
// not assert closed-shell topology or material volume.
type CurvedOvalSurface struct {
	Model Model
	Outer QuadraticOval
	Holes []QuadraticOval
}

func curvedOvalOptions() geom.SurfacePatchOptions {
	return geom.SurfacePatchOptions{MaxPoles: 1024, MaxOperations: 1000000, MaxCoefficientBits: 8192}
}
func unitOvalPlane() (*geom.BSplineSurface, error) {
	return geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
}
func (m Model) curvedOvalProjection(t TrimmedSurface) (Model, error) {
	if t.Surface < 0 || t.Surface >= len(m.Entities) || len(t.Inner) > 16 {
		return Model{}, geom.ErrInvalid
	}
	plane, e := unitOvalPlane()
	if e != nil {
		return Model{}, e
	}
	flat := m
	flat.Entities = append([]Entity(nil), m.Entities...)
	flat.Entities[t.Surface] = Entity{Surface: &Surface{Geometry: plane, Polynomial: true, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}}
	for _, id := range append([]int{t.Outer}, t.Inner...) {
		if id < 0 || id >= len(m.CurvesOnSurfaces) {
			return Model{}, geom.ErrInvalid
		}
		c := m.CurvesOnSurfaces[id]
		if c.Surface != t.Surface || c.ParameterCurve < 0 || c.ParameterCurve >= len(m.Entities) || c.ModelCurve < 0 || c.ModelCurve >= len(m.Entities) || c.ParameterCurve == c.ModelCurve {
			return Model{}, geom.ErrInvalid
		}
		uv := m.Entities[c.ParameterCurve]
		if uv.Spline == nil {
			return Model{}, geom.ErrInvalid
		}
		sp := *uv.Spline
		sp.Planar = true
		sp.Normal = geom.Vec3{Z: 1}
		flat.Entities[c.ModelCurve] = Entity{Spline: &sp}
	}
	return flat, nil
}
func (m Model) curvedOvalSupport(id int) (*geom.SurfacePatch, error) {
	if id < 0 || id >= len(m.Entities) {
		return nil, geom.ErrInvalid
	}
	en := m.Entities[id]
	if en.Surface == nil || en.Transform != nil || en.sourceWeightsRounded {
		return nil, geom.ErrUnrepresentable
	}
	if e := en.validate(); e != nil {
		return nil, e
	}
	s := en.Surface
	d := s.Geometry.Data()
	if d.DegreeU != 2 || d.DegreeV != 2 || d.CountU != 3 || d.CountV != 3 || s.UStart < 0 || s.UEnd > 1 || s.VStart < 0 || s.VEnd > 1 {
		return nil, geom.ErrInvalid
	}
	p, e := geom.NewSurfacePatch(s.Geometry, curvedOvalOptions())
	if e != nil {
		return nil, e
	}
	proof, e := geom.CertifySurfaceInjectivity(p, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: 8, MaxCells: 128, MaxOperations: 1000000, MaxCoefficientBits: 8192})
	if e != nil || !proof.ProvenInjective {
		return nil, geom.ErrConvergence
	}
	return p, nil
}
func (m Model) validateCurvedOvalMapping(c CurveOnSurface) error {
	if len(c.Label) > 8 || c.Subscript < 0 || c.Subscript > 99999999 || c.CreationMode < 0 || c.CreationMode > 3 || c.Preference < 0 || c.Preference > 3 || c.ParameterCurve < 0 || c.ParameterCurve >= len(m.Entities) || c.ModelCurve < 0 || c.ModelCurve >= len(m.Entities) || c.ParameterCurve == c.ModelCurve {
		return geom.ErrInvalid
	}
	for _, b := range []byte(c.Label) {
		if b < 32 || b > 126 {
			return geom.ErrInvalid
		}
	}
	if !m.exactContourWeights(c) {
		return geom.ErrUnrepresentable
	}
	patch, e := m.curvedOvalSupport(c.Surface)
	if e != nil {
		return e
	}
	uv, model := m.Entities[c.ParameterCurve], m.Entities[c.ModelCurve]
	if uv.Spline == nil || model.Spline == nil || uv.Transform != nil || model.Transform != nil {
		return geom.ErrInvalid
	}
	for _, en := range []Entity{uv, model} {
		if e = en.validate(); e != nil {
			return e
		}
	}
	u, v := uv.Spline, model.Spline
	support := m.Entities[c.Surface].Surface
	for _, p := range u.Curve.Poles() {
		if p.X < support.UStart || p.X > support.UEnd || p.Y < support.VStart || p.Y > support.VEnd {
			return geom.ErrInvalid
		}
	}

	if u.Start != 0 || u.End != 4 || v.Start != 0 || v.End != 4 || !u.Closed || !v.Closed || u.Curve.Degree() != 2 || len(u.Curve.Poles()) != 9 || v.Curve.Degree() > 16 || len(v.Curve.Poles()) > 128 {
		return geom.ErrInvalid
	}
	exactUV, e := geom.NewExactSplineCurve(u.Curve, curvedOvalOptions())
	if e != nil {
		return e
	}
	exactModel, e := geom.NewExactSplineCurve(v.Curve, curvedOvalOptions())
	if e != nil {
		return e
	}
	for quarter := 0; quarter < 4; quarter++ {
		a, b := float64(quarter), float64(quarter+1)
		arc, e := exactUV.Restrict(a, b, curvedOvalOptions())
		if e != nil {
			return e
		}
		arc, e = arc.Reparameterize(0, 1, false, curvedOvalOptions())
		if e != nil {
			return e
		}
		composed, e := patch.ComposeParameterCurve(context.Background(), arc, curvedOvalOptions())
		if e != nil {
			return e
		}
		modelArc, e := exactModel.Restrict(a, b, curvedOvalOptions())
		if e != nil {
			return e
		}
		same, e := geom.CompareExactCurvesContext(context.Background(), composed, modelArc, geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxSpans: 128, MaxOperations: 1000000, MaxCoefficientBits: 8192})
		if e != nil {
			return e
		}
		if !same.Complete || !same.Equal {
			return geom.ErrInvalid
		}
	}
	return nil
}
func (m Model) validateCurvedOvalTrim(t TrimmedSurface) error {
	flat, e := m.curvedOvalProjection(t)
	if e != nil {
		return e
	}
	if e = flat.ValidateTrimmedSurface(t); e != nil {
		return e
	}
	if _, _, e = flat.QuadraticOvalBoundaries(t); e != nil {
		return e
	}
	for _, id := range append([]int{t.Outer}, t.Inner...) {
		if e = m.validateCurvedOvalMapping(m.CurvesOnSurfaces[id]); e != nil {
			return e
		}
	}
	return nil
}

// CertifyCurvedOvalSurface proves complete ownership, projected injectivity,
// winding/containment and exact whole-curve surface correspondence.
func CertifyCurvedOvalSurface(m Model) (CurvedOvalSurface, error) {
	if len(m.TrimmedSurfaces) != 1 {
		return CurvedOvalSurface{}, geom.ErrInvalid
	}
	t := m.TrimmedSurfaces[0]
	n := 1 + len(t.Inner)
	if n < 1 || n > 17 || len(m.Entities) != 1+2*n || len(m.CurvesOnSurfaces) != n {
		return CurvedOvalSurface{}, geom.ErrInvalid
	}
	used := map[int]bool{t.Surface: true}
	contours := map[int]bool{}
	for _, id := range append([]int{t.Outer}, t.Inner...) {
		if id < 0 || id >= n || contours[id] {
			return CurvedOvalSurface{}, geom.ErrInvalid
		}
		contours[id] = true
		c := m.CurvesOnSurfaces[id]
		for _, entity := range []int{c.ParameterCurve, c.ModelCurve} {
			if used[entity] {
				return CurvedOvalSurface{}, geom.ErrInvalid
			}
			used[entity] = true
		}
	}
	if len(used) != len(m.Entities) {
		return CurvedOvalSurface{}, geom.ErrInvalid
	}
	if e := m.validateCurvedOvalTrim(t); e != nil {
		return CurvedOvalSurface{}, e
	}
	flat, e := m.curvedOvalProjection(t)
	if e != nil {
		return CurvedOvalSurface{}, e
	}
	outer, holes, e := flat.QuadraticOvalBoundaries(t)
	if e != nil {
		return CurvedOvalSurface{}, e
	}
	return CurvedOvalSurface{m, outer, holes}, nil
}
func ReadCurvedOvalSurface(r io.Reader, l Limits) (CurvedOvalSurface, error) {
	m, e := readCurvedMode(r, l, true)
	if e != nil {
		return CurvedOvalSurface{}, e
	}
	return CertifyCurvedOvalSurface(m)
}
func WriteCurvedOvalSurface(w io.Writer, s CurvedOvalSurface) error {
	proved, e := CertifyCurvedOvalSurface(s.Model)
	if e != nil {
		return e
	}
	return writeCurvedMode(w, proved.Model, true)
}
