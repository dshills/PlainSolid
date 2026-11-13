package iges

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"plainsolid/geom"
)

// RevolutionSurface retains a type120 record, with direct axis-line and spline
// generatrix indices into Model.Entities. Angles are source radians, increasing
// about the directed axis line. The qualified OCCT IGES import uses the opposite
// normal to native source-parameter Du×Dv; no oriented face is constructed.
type RevolutionSurface struct {
	Axis, Generatrix     int
	AngleStart, AngleEnd float64
	Label                string
	Subscript            int
}

// RevolutionGeometry constructs source-parameter analytic geometry. U is the
// authored rotation angle; V is the full original spline parameter. In OCCT's
// imported face parameterization Uocct=AngleEnd−U and its axis is reversed.
func (m Model) RevolutionGeometry(index int) (geom.RevolutionSurface, error) {
	fail := func() (geom.RevolutionSurface, error) { return geom.RevolutionSurface{}, geom.ErrDomain }
	if index < 0 || index >= len(m.Revolutions) {
		return fail()
	}
	r := m.Revolutions[index]
	if r.Axis < 0 || r.Axis >= len(m.Entities) || r.Generatrix < 0 || r.Generatrix >= len(m.Entities) || r.Axis == r.Generatrix {
		return fail()
	}
	a, p := m.Entities[r.Axis], m.Entities[r.Generatrix]
	if a.Line == nil || p.Spline == nil || a.Transform != nil || p.Transform != nil || a.validate() != nil || p.validate() != nil || p.sourceWeightsRounded {
		return fail()
	}
	s := p.Spline
	if s.Periodic || s.Closed {
		return fail()
	}
	lo, hi := s.Curve.Domain()
	if s.Start != lo || s.End != hi {
		return fail()
	}
	start, end := a.Line.Start, a.Line.End
	x := [3]float64{start.X, start.Y, start.Z}
	y := [3]float64{end.X, end.Y, end.Z}
	var dir [3]float64
	nonzero := 0
	for i := 0; i < 3; i++ {
		q := new(big.Rat).Sub(compositeRat(y[i]), compositeRat(x[i]))
		if q.Sign() != 0 {
			if q.Cmp(big.NewRat(1, 1)) != 0 && q.Cmp(big.NewRat(-1, 1)) != 0 {
				return fail()
			}
			dir[i] = float64(q.Sign())
			nonzero++
		}
	}
	if nonzero != 1 {
		return fail()
	}
	return geom.NewRevolutionSurface(s.Curve, start, geom.Vec3{X: dir[0], Y: dir[1], Z: dir[2]}, r.AngleStart, r.AngleEnd)
}
func validateRevolutions(m Model) error {
	if len(m.Revolutions) < 1 || len(m.Revolutions) > 64 || len(m.Entities) > 128 || len(m.CurvesOnSurfaces) > 0 || len(m.TrimmedSurfaces) > 0 || len(m.TabulatedSurfaces) > 0 {
		return geom.ErrDomain
	}
	owned := map[int]bool{}
	poles := 0
	for i, r := range m.Revolutions {
		if len(r.Label) > 8 || r.Subscript < 0 || r.Subscript > 99999999 {
			return geom.ErrDomain
		}
		for _, c := range []byte(r.Label) {
			if c < 32 || c > 126 {
				return geom.ErrDomain
			}
		}
		if _, e := m.RevolutionGeometry(i); e != nil {
			return e
		}
		for _, id := range []int{r.Axis, r.Generatrix} {
			if owned[id] {
				return errors.New("iges: shared revolution dependency")
			}
			owned[id] = true
		}
		poles += len(m.Entities[r.Generatrix].Spline.Curve.Poles())
		if poles > 4096 {
			return geom.ErrDomain
		}
	}
	if len(owned) != len(m.Entities) {
		return errors.New("iges: unowned revolution entity")
	}
	return nil
}

// ReadRevolutionSurfaces reads standalone type120 supports with uniquely owned
// type110 unit axis lines and full-interval type126 profiles. Original scalar
// decimals must be float64-exact; normalized weights must also preserve ratios.
// Transforms, topology, nesting and partial meridian trims are unsupported.
func ReadRevolutionSurfaces(r io.Reader, l Limits) (Model, error) {
	l, e := l.defaults()
	if e != nil {
		return Model{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 192)
	l.MaxControlPoints = min(l.MaxControlPoints, 4096)
	m, e := readRevolutionMode(r, l, false, false, true)
	if e != nil {
		return Model{}, e
	}
	if e = validateRevolutions(m); e != nil {
		return Model{}, e
	}
	return m, nil
}

// WriteRevolutionSurfaces stages and revalidates type120 geometry before I/O.
// Authored axis direction, angles, profile parameters, labels and units survive;
// the writer does not silently choose an opposite surface-normal convention.
func WriteRevolutionSurfaces(w io.Writer, m Model) error {
	if w == nil {
		return geom.ErrDomain
	}
	if e := validateRevolutions(m); e != nil {
		return e
	}
	var b bytes.Buffer
	if e := writeRevolutionMode(&b, m, false, false, true); e != nil {
		return e
	}
	if _, e := ReadRevolutionSurfaces(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		return io.ErrShortWrite
	}
	return e
}
