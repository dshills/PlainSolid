package iges

import (
	"errors"
	"math/big"
	"plainsolid/geom"
)

var rectangleOptions = geom.SurfacePatchOptions{MaxOperations: 200000, MaxCoefficientBits: 4096}

func (m Model) validateComposite(index int) error {
	e := m.Entities[index]
	if err := e.validate(); err != nil {
		return err
	}
	seen := map[int]bool{}
	for _, k := range e.Composite.Curves {
		if k < 0 || k >= len(m.Entities) || seen[k] {
			return errors.New("iges: invalid composite reference")
		}
		seen[k] = true
		leaf := m.Entities[k]
		if leaf.Line == nil && leaf.Spline == nil {
			return errors.New("iges: composite requires direct lines or splines")
		}
		if err := leaf.validate(); err != nil {
			return err
		}
	}
	return nil
}

func rectanglePatch(s *geom.BSplineSurface, d geom.ParameterRectangle) (*geom.SurfacePatch, error) {
	p, err := geom.NewSurfacePatch(s, rectangleOptions)
	if err != nil {
		return nil, err
	}
	p, err = p.Trim(d)
	if err != nil {
		return nil, err
	}
	for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		r, err := geom.CertifySurfaceInjectivity(p, geom.SurfaceInjectivityOptions{ProjectionAxes: axes, MaxDepth: 4, MaxCells: 64, MaxOperations: 200000, MaxCoefficientBits: 4096})
		if err == nil && r.ProvenInjective {
			return p, nil
		}
	}
	return nil, errors.New("iges: rectangular surface injectivity not certified")
}

// rectangularContour proves a positively oriented axis-aligned UV rectangle
// and exact model correspondence; no tolerant or sampled equality is accepted.
func (m Model) rectangularContour(c CurveOnSurface) (geom.ParameterRectangle, error) {
	return m.rectangularContourSense(c, 0)
}

func (m Model) rectangularContourSense(c CurveOnSurface, sense int) (geom.ParameterRectangle, error) {
	loop, e := m.orthogonalContourSense(c, sense)
	if e != nil {
		return geom.ParameterRectangle{}, e
	}
	if len(loop) != 4 {
		return geom.ParameterRectangle{}, errors.New("iges: rectangular API requires four edges")
	}
	d := geom.ParameterRectangle{UMin: loop[0].X, UMax: loop[0].X, VMin: loop[0].Y, VMax: loop[0].Y}
	for _, p := range loop {
		d.UMin = min(d.UMin, p.X)
		d.UMax = max(d.UMax, p.X)
		d.VMin = min(d.VMin, p.Y)
		d.VMax = max(d.VMax, p.Y)
	}
	return d, nil
}

func (m Model) orthogonalContourSense(c CurveOnSurface, sense int) ([]geom.Vec2, error) {
	fail := func() ([]geom.Vec2, error) {
		return nil, errors.New("iges: unsupported orthogonal spline contour")
	}
	if c.Surface < 0 || c.Surface >= len(m.Entities) || c.ParameterCurve < 0 || c.ParameterCurve >= len(m.Entities) || c.ModelCurve < 0 || c.ModelCurve >= len(m.Entities) {
		return fail()
	}
	if !m.exactContourWeights(c) {
		return nil, geom.ErrUnrepresentable
	}
	se, ue, me := m.Entities[c.Surface], m.Entities[c.ParameterCurve], m.Entities[c.ModelCurve]
	if se.Surface == nil || ue.Composite == nil || me.Composite == nil || len(ue.Composite.Curves) != len(me.Composite.Curves) {
		return fail()
	}
	if err := exactRigidPlacement(se.Transform); err != nil {
		return fail()
	}
	if err := se.validate(); err != nil {
		return fail()
	}
	for _, idx := range []int{c.ParameterCurve, c.ModelCurve} {
		if err := m.validateComposite(idx); err != nil {
			return fail()
		}
	}
	uv := make([]Line, len(ue.Composite.Curves))
	d := geom.ParameterRectangle{}
	for i, k := range ue.Composite.Curves {
		e := m.Entities[k]
		if e.Line == nil || e.Transform != nil {
			return fail()
		}
		uv[i] = *e.Line
		a, b := uv[i].Start, uv[i].End
		if a.Z != 0 || b.Z != 0 || (a.X == b.X) == (a.Y == b.Y) {
			return fail()
		}
		if i == 0 {
			d = geom.ParameterRectangle{UMin: a.X, UMax: a.X, VMin: a.Y, VMax: a.Y}
		}
		d.UMin = min(d.UMin, a.X, b.X)
		d.UMax = max(d.UMax, a.X, b.X)
		d.VMin = min(d.VMin, a.Y, b.Y)
		d.VMax = max(d.VMax, a.Y, b.Y)
	}
	loop := make([]geom.Vec2, len(uv))
	for i, e := range uv {
		if e.End != uv[(i+1)%len(uv)].Start {
			return fail()
		}
		loop[i] = geom.Vec2{X: e.Start.X, Y: e.Start.Y}
	}
	if err := validateOrthogonalPolygon(loop); err != nil {
		return fail()
	}
	if d.UMin >= d.UMax || d.VMin >= d.VMax || trimSign(loop) == 0 || sense != 0 && trimSign(loop) != sense {
		return fail()
	}
	s := se.Surface
	if d.UMin < s.UStart || d.UMax > s.UEnd || d.VMin < s.VStart || d.VMax > s.VEnd {
		return fail()
	}
	patch, err := rectanglePatch(s.Geometry, d)
	if err != nil {
		return nil, err
	}
	for i, line := range uv {
		axis, fixed, reverse := 0, line.Start.X, line.End.Y < line.Start.Y
		if line.Start.Y == line.End.Y {
			axis, fixed, reverse = 1, line.Start.Y, line.End.X < line.Start.X
		}
		expected, err := patch.IsoCurve(axis, fixed, rectangleOptions)
		if err != nil {
			return nil, err
		}
		lo, hi := min(line.Start.Y, line.End.Y), max(line.Start.Y, line.End.Y)
		if axis == 1 {
			lo, hi = min(line.Start.X, line.End.X), max(line.Start.X, line.End.X)
		}
		expected, err = expected.Restrict(lo, hi, rectangleOptions)
		if err != nil {
			return nil, err
		}
		if se.Transform != nil {
			expected, err = expected.Transform(*se.Transform, rectangleOptions)
			if err != nil {
				return nil, err
			}
		}
		leaf := m.Entities[me.Composite.Curves[i]]
		if err := exactRigidPlacement(leaf.Transform); err != nil {
			return fail()
		}
		var actual *geom.ExactSplineCurve
		if leaf.Spline != nil {
			actual, err = geom.NewExactSplineCurve(leaf.Spline.Curve, rectangleOptions)
			if err == nil {
				actual, err = actual.Restrict(leaf.Spline.Start, leaf.Spline.End, rectangleOptions)
			}
		} else {
			return fail()
		}
		if err != nil {
			return nil, err
		}
		if leaf.Transform != nil {
			actual, err = actual.Transform(*leaf.Transform, rectangleOptions)
			if err != nil {
				return nil, err
			}
		}
		result, err := geom.CompareExactCurves(expected, actual, geom.ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: reverse, MaxSpans: 1024, MaxOperations: 200000, MaxCoefficientBits: 4096})
		if err != nil || !result.Complete || !result.Equal {
			return fail()
		}
	}
	return loop, nil
}

// RectangularDomain returns the certified UV rectangle of a type144 spline
// face with a four-line type102 UV contour and matching model spline contour.
// General curved trims, nested composites and nonrigid placements are unsupported.
func (m Model) RectangularDomain(t TrimmedSurface) (geom.ParameterRectangle, error) {
	if err := m.ValidateTrimmedSurface(t); err != nil {
		return geom.ParameterRectangle{}, err
	}
	return m.rectangularContour(m.CurvesOnSurfaces[t.Outer])
}

// NewRectangularSurface builds a single explicitly trimmed rational spline face.
// It retains the original support and exports each exact iso boundary only when
// its spline coefficients are losslessly representable. No geometry is rounded.
func NewRectangularSurface(s *geom.BSplineSurface, d geom.ParameterRectangle, lengthUnit float64) (Model, error) {
	return newRectangularSurface(s, d, lengthUnit, false)
}

func newRectangularSurface(s *geom.BSplineSurface, d geom.ParameterRectangle, lengthUnit float64, clockwise bool) (Model, error) {
	unitOK := false
	for _, u := range units {
		unitOK = unitOK || lengthUnit == u.meters
	}
	if !unitOK {
		return Model{}, errors.New("iges: unsupported length unit")
	}
	p, err := rectanglePatch(s, d)
	if err != nil {
		return Model{}, err
	}
	m := Model{LengthUnit: lengthUnit, Entities: []Entity{{Surface: &Surface{Geometry: s, UStart: d.UMin, UEnd: d.UMax, VStart: d.VMin, VEnd: d.VMax}}}}
	corners := []geom.Vec3{{X: d.UMin, Y: d.VMax}, {X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}}
	if clockwise {
		corners = []geom.Vec3{{X: d.UMin, Y: d.VMin}, {X: d.UMin, Y: d.VMax}, {X: d.UMax, Y: d.VMax}, {X: d.UMax, Y: d.VMin}}
	}
	uvIDs, modelIDs := []int{}, []int{}
	for i, a := range corners {
		b := corners[(i+1)%4]
		uvIDs = append(uvIDs, len(m.Entities))
		m.Entities = append(m.Entities, Entity{Line: &Line{a, b}})
		axis, fixed, reverse := 0, a.X, b.Y < a.Y
		if a.Y == b.Y {
			axis, fixed, reverse = 1, a.Y, b.X < a.X
		}
		curve, e := p.IsoCurve(axis, fixed, rectangleOptions)
		if e != nil {
			return Model{}, e
		}
		lo, hi := curve.Domain()
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
	m.CurvesOnSurfaces = []CurveOnSurface{{Surface: 0, ParameterCurve: ui, ModelCurve: mi, Preference: 3}}
	m.TrimmedSurfaces = []TrimmedSurface{{Surface: 0, Outer: 0}}
	if !clockwise {
		if err := m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); err != nil {
			return Model{}, err
		}
	}
	return m, nil
}

// exactRigidPlacement requires exact orthogonality on the stored binary values.
// This deliberately rejects rounded rotations as well as nonuniform transforms:
// OCCT may discard unsupported IGES placements without failing the shape import.
func exactRigidPlacement(a *geom.Affine) error {
	if a == nil {
		return nil
	}
	if _, err := a.Inverse(); err != nil {
		return err
	}
	if a.Determinant() <= 0 {
		return errors.New("iges: reflected placement unsupported")
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			dot := new(big.Rat)
			for k := 0; k < 3; k++ {
				dot.Add(dot, new(big.Rat).Mul(rat(a[k][i]), rat(a[k][j])))
			}
			target := new(big.Rat)
			if i == j {
				target.SetInt64(1)
			}
			if dot.Cmp(target) != 0 {
				return errors.New("iges: rectangular face placement must be exactly rigid")
			}
		}
	}
	return nil
}

// NewPlacedRectangularSurface retains a proper rigid type124 placement on the
// support and model boundaries; UV curves remain in the original parameter space.
// Exact binary orthogonality is required. Reflection, scale, shear and rounded
// approximate rotation matrices fail explicitly. Control points are not baked.
func NewPlacedRectangularSurface(s *geom.BSplineSurface, d geom.ParameterRectangle, lengthUnit float64, placement geom.Affine) (Model, error) {
	if err := exactRigidPlacement(&placement); err != nil {
		return Model{}, err
	}
	m, err := NewRectangularSurface(s, d, lengthUnit)
	if err != nil {
		return Model{}, err
	}
	m.Entities[0].Transform = &placement
	c := m.CurvesOnSurfaces[0]
	for _, idx := range m.Entities[c.ModelCurve].Composite.Curves {
		a := placement
		m.Entities[idx].Transform = &a
	}
	if err := m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); err != nil {
		return Model{}, err
	}
	return m, nil
}

// NewRectangularSurfaceWithHole constructs a single exact rational face with a
// counterclockwise outer rectangle and one strictly interior clockwise hole.
// The original support is retained, and every boundary must export losslessly.
func NewRectangularSurfaceWithHole(s *geom.BSplineSurface, outer, hole geom.ParameterRectangle, lengthUnit float64) (Model, error) {
	return NewRectangularSurfaceWithHoles(s, outer, []geom.ParameterRectangle{hole}, lengthUnit)
}

// MaxRectangularHoles bounds exact spline contour proofs per face.
const MaxRectangularHoles = 16

// NewRectangularSurfaceWithHoles retains the original rational support and
// constructs up to MaxRectangularHoles disjoint clockwise rectangular holes.
// An empty hole list is allowed. Touching, nesting and overlap fail exactly.
func NewRectangularSurfaceWithHoles(s *geom.BSplineSurface, outer geom.ParameterRectangle, holes []geom.ParameterRectangle, lengthUnit float64) (Model, error) {
	if len(holes) > MaxRectangularHoles {
		return Model{}, errors.New("iges: rectangular hole limit")
	}
	for i, hole := range holes {
		if !rectangleContains(outer, hole) {
			return Model{}, errors.New("iges: hole must be strictly inside outer rectangle")
		}
		for _, previous := range holes[:i] {
			if !rectanglesDisjoint(previous, hole) {
				return Model{}, errors.New("iges: rectangular holes must be disjoint without touching")
			}
		}
	}
	m, err := newRectangularSurface(s, outer, lengthUnit, false)
	if err != nil {
		return Model{}, err
	}
	for _, hole := range holes {
		inner, err := newRectangularSurface(s, hole, lengthUnit, true)
		if err != nil {
			return Model{}, err
		}
		shift := len(m.Entities) - 1
		for _, entity := range inner.Entities[1:] {
			if entity.Composite != nil {
				for i := range entity.Composite.Curves {
					entity.Composite.Curves[i] += shift
				}
			}
			m.Entities = append(m.Entities, entity)
		}
		c := inner.CurvesOnSurfaces[0]
		c.ParameterCurve += shift
		c.ModelCurve += shift
		m.TrimmedSurfaces[0].Inner = append(m.TrimmedSurfaces[0].Inner, len(m.CurvesOnSurfaces))
		m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, c)
	}
	if err = m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); err != nil {
		return Model{}, err
	}
	return m, nil
}
func rectanglesDisjoint(a, b geom.ParameterRectangle) bool {
	return a.UMax < b.UMin || b.UMax < a.UMin || a.VMax < b.VMin || b.VMax < a.VMin
}

func rectangleContains(outer, hole geom.ParameterRectangle) bool {
	return hole.UMin > outer.UMin && hole.UMax < outer.UMax && hole.VMin > outer.VMin && hole.VMax < outer.VMax && hole.UMin < hole.UMax && hole.VMin < hole.VMax
}

// RectangularHole returns the one certified inner rectangle. Faces with no hole
// or with a different boundary representation return an error.
func (m Model) RectangularHole(t TrimmedSurface) (geom.ParameterRectangle, error) {
	if err := m.ValidateTrimmedSurface(t); err != nil {
		return geom.ParameterRectangle{}, err
	}
	if len(t.Inner) != 1 {
		return geom.ParameterRectangle{}, errors.New("iges: exactly one rectangular hole required")
	}
	return m.rectangularContourSense(m.CurvesOnSurfaces[t.Inner[0]], -1)
}

// RectangularHoles returns all certified inner rectangles in source contour
// order. It accepts zero through MaxRectangularHoles and requires a rectangular
// outer contour. RectangularHole retains its exactly-one-hole contract.
func (m Model) RectangularHoles(t TrimmedSurface) ([]geom.ParameterRectangle, error) {
	if _, err := m.RectangularDomain(t); err != nil {
		return nil, err
	}
	holes := make([]geom.ParameterRectangle, len(t.Inner))
	for i, id := range t.Inner {
		d, err := m.rectangularContourSense(m.CurvesOnSurfaces[id], -1)
		if err != nil {
			return nil, err
		}
		holes[i] = d
	}
	return holes, nil
}
