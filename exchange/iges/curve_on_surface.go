package iges

import (
	"errors"
	"math/big"
	"plainsolid/geom"
)

// CurveOnSurface retains type142 references as indices into Model.Entities.
// Both UV and model-space splines are required. Preference and CreationMode are
// IGES metadata; this subset requires exact agreement regardless of preference.
type CurveOnSurface struct {
	Surface, ParameterCurve, ModelCurve int
	CreationMode, Preference            int
	Label                               string
	Subscript                           int
}

func rat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
func exactLinear(a, b, c, x, y, want float64) bool {
	v := new(big.Rat).Add(rat(a), new(big.Rat).Add(new(big.Rat).Mul(rat(b), rat(x)), new(big.Rat).Mul(rat(c), rat(y))))
	return v.Cmp(rat(want)) == 0
}

// ValidateCurveOnSurface proves UV/model correspondence for an affine bilinear
// type128 patch using exact rational arithmetic on the stored float64 values.
// No tolerance, sampling or approximation is used to accept correspondence.
func (m Model) ValidateCurveOnSurface(c CurveOnSurface) error {
	fail := func() error { return errors.New("iges: unsupported or inconsistent curve-on-surface mapping") }
	if len(c.Label) > 8 || c.Subscript < 0 || c.Subscript > 99999999 {
		return fail()
	}
	for _, b := range []byte(c.Label) {
		if b < 32 || b > 126 {
			return fail()
		}
	}
	if c.CreationMode < 0 || c.CreationMode > 3 || c.Preference < 0 || c.Preference > 3 || c.Surface < 0 || c.Surface >= len(m.Entities) || c.ParameterCurve < 0 || c.ParameterCurve >= len(m.Entities) || c.ModelCurve < 0 || c.ModelCurve >= len(m.Entities) || c.ParameterCurve == c.ModelCurve {
		return fail()
	}
	if !m.exactContourWeights(c) {
		return geom.ErrUnrepresentable
	}
	se, ue, me := m.Entities[c.Surface], m.Entities[c.ParameterCurve], m.Entities[c.ModelCurve]
	if ue.Composite != nil || me.Composite != nil {
		_, err := m.orthogonalContourSense(c, 0)
		return err
	}
	if se.Surface == nil || ue.Spline == nil || me.Spline == nil || se.Transform != nil || ue.Transform != nil || me.Transform != nil {
		return fail()
	}
	for _, e := range []Entity{se, ue, me} {
		if err := e.validate(); err != nil {
			return err
		}
	}
	d := se.Surface.Geometry.Data()
	if d.DegreeU != 1 || d.DegreeV != 1 || d.CountU != 2 || d.CountV != 2 || len(d.KnotsU) != 4 || len(d.KnotsV) != 4 || d.KnotsU[0] != d.KnotsU[1] || d.KnotsU[2] != d.KnotsU[3] || d.KnotsV[0] != d.KnotsV[1] || d.KnotsV[2] != d.KnotsV[3] {
		return fail()
	}
	for _, w := range d.Weights {
		if w != d.Weights[0] {
			return fail()
		}
	}
	// Require a unit knot square: broader reparameterization is explicitly absent.
	if d.KnotsU[0] != 0 || d.KnotsU[3] != 1 || d.KnotsV[0] != 0 || d.KnotsV[3] != 1 {
		return fail()
	}
	p00, p01, p10, p11 := d.Poles[0], d.Poles[1], d.Poles[2], d.Poles[3]
	a, b := p10.Sub(p00), p01.Sub(p00)
	if !a.IsFinite() || !b.IsFinite() || a.Cross(b).Length() == 0 {
		return fail()
	}
	// Verify differences and parallelogram exactly, including subtraction rounding.
	coords := func(p geom.Vec3) [3]float64 { return [3]float64{p.X, p.Y, p.Z} }
	o, x, y, end := coords(p00), coords(a), coords(b), coords(p11)
	px, py := coords(p10), coords(p01)
	for j := 0; j < 3; j++ {
		if !exactLinear(o[j], x[j], 0, 1, 0, px[j]) || !exactLinear(o[j], y[j], 0, 1, 0, py[j]) || !exactLinear(o[j], x[j], y[j], 1, 1, end[j]) {
			return fail()
		}
	}
	uv, model := ue.Spline, me.Spline
	if uv.Start != model.Start || uv.End != model.End || uv.Curve.Degree() != model.Curve.Degree() {
		return fail()
	}
	uk, mk := uv.Curve.Knots(), model.Curve.Knots()
	uw, mw := uv.Curve.Weights(), model.Curve.Weights()
	up, mp := uv.Curve.Poles(), model.Curve.Poles()
	if len(uk) != len(mk) || len(uw) != len(mw) || len(up) != len(mp) {
		return fail()
	}
	for i := range uk {
		if uk[i] != mk[i] {
			return fail()
		}
	}
	for i := range uw {
		if uw[i] != mw[i] {
			return fail()
		}
	}
	for i, p := range up {
		if p.Z != 0 || p.X < se.Surface.UStart || p.X > se.Surface.UEnd || p.Y < se.Surface.VStart || p.Y > se.Surface.VEnd {
			return fail()
		}
		q := coords(mp[i])
		for j := 0; j < 3; j++ {
			if !exactLinear(o[j], x[j], y[j], p.X, p.Y, q[j]) {
				return fail()
			}
		}
	}
	return nil
}
