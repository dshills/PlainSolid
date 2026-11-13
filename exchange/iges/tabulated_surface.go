package iges

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"plainsolid/geom"
)

// TabulatedSurface retains type122: End is the authored terminal point of the
// generatrix whose start is the directrix's first point. It is not a direction.
type TabulatedSurface struct {
	Directrix int
	End       geom.Vec3
	Label     string
	Subscript int
}

// TabulatedGeometry constructs the exact tensor identity S(u,v)=C(u)+vD,
// D=End−C(start), on the full original curve domain and normalized v∈[0,1].
// Repeated weights and linear v basis prove this throughout the rectangle.
// No direction normalization, regularity, embedding or face proof is implied.
// OCCT uses physical distance v*|D|; its distance derivative must be scaled by |D|.
func (m Model) TabulatedGeometry(index int) (*geom.BSplineSurface, error) {
	if index < 0 || index >= len(m.TabulatedSurfaces) {
		return nil, geom.ErrDomain
	}
	r := m.TabulatedSurfaces[index]
	if r.Directrix < 0 || r.Directrix >= len(m.Entities) || !r.End.IsFinite() {
		return nil, geom.ErrDomain
	}
	e := m.Entities[r.Directrix]
	if e.Spline == nil || e.Transform != nil || e.sourceWeightsRounded {
		return nil, geom.ErrDomain
	}
	if err := e.validate(); err != nil {
		return nil, err
	}
	s := e.Spline
	c := s.Curve
	lo, hi := c.Domain()
	if s.Start != lo || s.End != hi || s.Periodic || s.Closed {
		return nil, geom.ErrDomain
	}
	poles, weights, knots := c.Poles(), c.Weights(), c.Knots()
	if len(poles) > 4096 {
		return nil, geom.ErrDomain
	}
	for i := 0; i <= c.Degree(); i++ {
		if knots[i] != lo || knots[len(knots)-1-i] != hi {
			return nil, geom.ErrDomain
		}
	}
	run := 0
	for i, k := range knots {
		if i == 0 || k != knots[i-1] {
			run = 1
		} else {
			run++
		}
		if k != lo && k != hi && run > c.Degree() {
			return nil, geom.ErrDomain
		}
	}
	a := [3]float64{poles[0].X, poles[0].Y, poles[0].Z}
	b := [3]float64{r.End.X, r.End.Y, r.End.Z}
	var delta [3]*big.Rat
	nonzero := false
	for i := range delta {
		delta[i] = new(big.Rat).Sub(compositeRat(b[i]), compositeRat(a[i]))
		nonzero = nonzero || delta[i].Sign() != 0
	}
	if !nonzero {
		return nil, geom.ErrDomain
	}
	net := make([]geom.Vec3, 0, 2*len(poles))
	ws := make([]float64, 0, 2*len(poles))
	for i, p := range poles {
		q := [3]float64{p.X, p.Y, p.Z}
		for j := range q {
			v := new(big.Rat).Add(compositeRat(q[j]), delta[j])
			var exact bool
			q[j], exact = v.Float64()
			if !exact || !finite(q[j]) {
				return nil, geom.ErrUnrepresentable
			}
		}
		net = append(net, p, geom.Vec3{X: q[0], Y: q[1], Z: q[2]})
		ws = append(ws, weights[i], weights[i])
	}
	return geom.NewBSplineSurface(c.Degree(), 1, len(poles), 2, net, knots, []float64{0, 0, 1, 1}, ws)
}
func validateTabulated(m Model) error {
	if len(m.TabulatedSurfaces) < 1 || len(m.TabulatedSurfaces) > 64 || len(m.Entities) > 64 || len(m.Revolutions) > 0 || len(m.CurvesOnSurfaces) > 0 || len(m.TrimmedSurfaces) > 0 {
		return geom.ErrDomain
	}
	owned := map[int]bool{}
	poles := 0
	for i, r := range m.TabulatedSurfaces {
		if len(r.Label) > 8 || r.Subscript < 0 || r.Subscript > 99999999 {
			return geom.ErrDomain
		}
		for _, c := range []byte(r.Label) {
			if c < 32 || c > 126 {
				return geom.ErrDomain
			}
		}
		if _, e := m.TabulatedGeometry(i); e != nil {
			return e
		}
		if owned[r.Directrix] {
			return errors.New("iges: shared tabulated directrix")
		}
		owned[r.Directrix] = true
		poles += len(m.Entities[r.Directrix].Spline.Curve.Poles())
		if poles > 4096 {
			return geom.ErrDomain
		}
	}
	if len(owned) != len(m.Entities) {
		return errors.New("iges: unowned tabulated entity")
	}
	return nil
}

// ReadTabulatedSurfaces reads strictly owned standalone type122/direct-type126
// surfaces with source-exact scalar conversion and lossless weight normalization.
func ReadTabulatedSurfaces(r io.Reader, l Limits) (Model, error) {
	l, e := l.defaults()
	if e != nil {
		return Model{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 128)
	l.MaxControlPoints = min(l.MaxControlPoints, 4096)
	m, e := readSweptMode(r, l, false, false, false, true)
	if e != nil {
		return Model{}, e
	}
	if e = validateTabulated(m); e != nil {
		return Model{}, e
	}
	return m, nil
}

// WriteTabulatedSurfaces validates and stages exact type122 geometry before I/O.
// Terminal point, directed displacement, original U domain, labels and units
// are retained; no source-to-model rescaling or approximate pole fit is used.
func WriteTabulatedSurfaces(w io.Writer, m Model) error {
	if w == nil {
		return geom.ErrDomain
	}
	if e := validateTabulated(m); e != nil {
		return e
	}
	var b bytes.Buffer
	if e := writeSweptMode(&b, m, false, false, false, true); e != nil {
		return e
	}
	if _, e := ReadTabulatedSurfaces(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		return io.ErrShortWrite
	}
	return e
}
