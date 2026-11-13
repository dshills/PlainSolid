package iges

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// RationalLensCurve retains raw represented positive weights without division
// by their maximum. Curves are continuous clamped splines on [0,1].
type RationalLensCurve struct {
	Degree         int
	Poles          []geom.Vec3
	Knots, Weights []float64
}

// RationalSplineLens retains independently refined authoritative model and
// upper/lower pcurve bases. Curves[edge][0,1,2] is model,upper UV,lower UV.
type RationalSplineLens struct {
	Body   SplineLensSolid
	Curves [4][3]RationalLensCurve
}

func rationalIGESCurve(c RationalLensCurve) (*geom.ExactSplineCurve, error) {
	if c.Degree < 1 || c.Degree > 16 || len(c.Poles) < 2 || len(c.Poles) > 64 || len(c.Weights) != len(c.Poles) || len(c.Knots) != len(c.Poles)+c.Degree+1 {
		return nil, geom.ErrInvalid
	}
	grammar, e := geom.NewBSpline(c.Degree, c.Poles, c.Knots, nil)
	if e != nil {
		return nil, e
	}
	a, b := grammar.Domain()
	if a != 0 || b != 1 {
		return nil, geom.ErrDomain
	}
	net := make([][4]*big.Rat, len(c.Poles))
	for i, p := range c.Poles {
		w := new(big.Rat).SetFloat64(c.Weights[i])
		if w == nil || w.Sign() <= 0 {
			return nil, geom.ErrInvalid
		}
		net[i] = [4]*big.Rat{new(big.Rat).Mul(new(big.Rat).SetFloat64(p.X), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Y), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Z), w), w}
	}
	return geom.NewExactHomogeneousSpline(context.Background(), c.Degree, net, c.Knots, curvedOvalOptions())
}
func decodeRationalIGESCurve(r topologyRecord, max int) (RationalLensCurve, *geom.ExactSplineCurve, Entity, error) {
	fail := func() (RationalLensCurve, *geom.ExactSplineCurve, Entity, error) {
		return RationalLensCurve{}, nil, Entity{}, geom.ErrInvalid
	}
	p := r.values
	if len(p) < 7 {
		return fail()
	}
	upper, e := intValue(p[1])
	if e != nil || upper < 1 || upper >= max || upper >= 64 {
		return fail()
	}
	degree, e := intValue(p[2])
	if e != nil || degree < 1 || degree > 16 {
		return fail()
	}
	n := upper + 1
	k := n + degree + 1
	if len(p) < 7+k+4*n+5 {
		return fail()
	}
	copyValues := append([]parameter(nil), p...)
	weights := make([]float64, n)
	for i := range weights {
		weights[i], e = realValue(p[7+k+i])
		if e != nil || weights[i] <= 0 {
			return fail()
		}
		copyValues[7+k+i] = parameter{text: "1"}
	}
	en, e := decode(126, copyValues, max)
	if e != nil {
		return RationalLensCurve{}, nil, Entity{}, e
	}
	s := en.Spline
	if s == nil || s.Start != 0 || s.End != 1 || s.Closed || s.Periodic {
		return fail()
	}
	if s.Polynomial {
		for _, w := range weights {
			if w != weights[0] {
				return fail()
			}
		}
	}
	if e = en.validate(); e != nil {
		return fail()
	}
	c := RationalLensCurve{degree, s.Curve.Poles(), s.Curve.Knots(), weights}
	x, e := rationalIGESCurve(c)
	return c, x, en, e
}
func ReadRationalSplineLensSolid(r io.Reader, l Limits) (RationalSplineLens, error) {
	d, e := readTopologySource(r, l, true)
	if e != nil {
		return RationalSplineLens{}, e
	}
	m, e := readIGESLens(d, l, true)
	if e != nil {
		return RationalSplineLens{}, e
	}
	return RationalSplineLens{Body: m, Curves: m.rawCurves}, nil
}
func encodeRationalIGESCurve(c RationalLensCurve, uv bool) (string, error) {
	if _, e := rationalIGESCurve(c); e != nil {
		return "", e
	}
	p := []string{"126", strconv.Itoa(len(c.Poles) - 1), strconv.Itoa(c.Degree), flag(uv), "0", "0", "0"}
	for _, values := range [][]float64{c.Knots, c.Weights} {
		for _, v := range values {
			p = append(p, number(v))
		}
	}
	for _, v := range c.Poles {
		p = append(p, number(v.X), number(v.Y), number(v.Z))
	}
	p = append(p, "0.", "1.", "0.", "0.", "1.")
	return strings.Join(p, ",") + ";", nil
}

// WriteRationalSplineLensSolid re-proves complete source geometry/material and
// retains raw source coefficient scales and independent knot partitions.
func WriteRationalSplineLensSolid(w io.Writer, m RationalSplineLens) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var stage bytes.Buffer
	body := m.Body
	body.rawCurves = [4][3]RationalLensCurve{}
	if e := WriteSplineLensSolid(&stage, body); e != nil {
		return e
	}
	d, e := readTopologySource(bytes.NewReader(stage.Bytes()), Limits{}, true)
	if e != nil {
		return e
	}
	base, e := readIGESLens(d, Limits{})
	if e != nil {
		return e
	}
	replace := map[int]string{}
	for i, row := range m.Curves {
		for owner, c := range row {
			id := base.SourceModelCurveIDs[i]
			if owner > 0 {
				id = base.SourceParameterCurveIDs[owner-1][i]
			}
			value, e := encodeRationalIGESCurve(c, owner > 0)
			if e != nil {
				return e
			}
			replace[id] = value
		}
	}
	a := topologyAuthor{geometry: Model{Entities: []Entity{{Surface: &Surface{Geometry: m.Body.Upper, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}}, {Surface: &Surface{Geometry: m.Body.Lower, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}}}}}
	for _, r := range d.entities {
		data, ok := replace[r.id]
		if !ok {
			v := make([]string, len(r.values))
			for i, p := range r.values {
				if p.stringValue {
					return geom.ErrInvalid
				}
				v[i] = p.text
			}
			data = strings.Join(v, ",") + ";"
		}
		form := 0
		if r.kind >= 500 {
			form = 1
		}
		a.add(r.kind, form, data)
	}
	data, e := a.file(m.Body.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = ReadRationalSplineLensSolid(bytes.NewReader(data), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(data)
	if e == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return e
}
