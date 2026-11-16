package parametric

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"reflect"

	"plainsolid/geom"
)

const exactBezierCurveMaxDegree = 64

type exactBezierCurveData struct {
	Version, Degree int
	Domain          Interval
	Controls        [][4]string
}

func (d *exactBezierCurveData) UnmarshalJSON(raw []byte) error {
	if len(raw) > 1<<20 {
		return geom.ErrConvergence
	}
	var wire struct {
		Version, Degree int
		Domain          Interval
		Controls        json.RawMessage
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return err
	}
	if wire.Version != 1 || wire.Degree < 1 || wire.Degree > exactBezierCurveMaxDegree || !wire.Domain.valid() {
		return geom.ErrInvalid
	}
	controls, err := decodeExactSurfaceControls(wire.Controls, wire.Degree+1)
	if err != nil {
		return err
	}
	*d = exactBezierCurveData{Version: wire.Version, Degree: wire.Degree, Domain: wire.Domain, Controls: controls}
	return nil
}

func encodeExactBezierCurve(c *geom.ExactSplineCurve) (*exactBezierCurveData, error) {
	q, _, err := c.BezierControlNet(context.Background(), geom.SurfacePatchOptions{
		MaxPoles: 2 * (exactBezierCurveMaxDegree + 1), MaxCoefficientBits: exactSurfaceMaxBits})
	if err != nil {
		return nil, err
	}
	d := exactBezierCurveData{Version: 1, Degree: len(q.Control) - 1, Domain: Interval{q.Start, q.End},
		Controls: make([][4]string, len(q.Control))}
	if d.Degree < 1 || d.Degree > exactBezierCurveMaxDegree || !d.Domain.valid() {
		return nil, geom.ErrDomain
	}
	for i, h := range q.Control {
		for axis, n := range h {
			d.Controls[i][axis] = n.RatString()
		}
	}
	return &d, nil
}

func validExactBezierCurveEnvelope(d curveData) bool {
	d.Kind, d.ExactBezier = "", nil
	return reflect.ValueOf(d).IsZero()
}

func (d *exactBezierCurveData) decode(budget *int) (geom.Curve, error) {
	count := len(d.Controls)
	if count > *budget {
		return nil, geom.ErrConvergence
	}
	net := make([][4]*big.Rat, count)
	for i, h := range d.Controls {
		for axis, s := range h {
			q, ok := new(big.Rat).SetString(s)
			if !ok || q.RatString() != s || q.Num().BitLen() > exactSurfaceMaxBits || q.Denom().BitLen() > exactSurfaceMaxBits ||
				axis == 3 && q.Sign() <= 0 {
				return nil, geom.ErrInvalid
			}
			net[i][axis] = q
		}
	}
	knots := make([]float64, 2*count)
	for i := range count {
		knots[i], knots[count+i] = d.Domain.Min, d.Domain.Max
	}
	q, err := geom.NewExactHomogeneousSpline(context.Background(), d.Degree, net, knots,
		geom.SurfacePatchOptions{MaxPoles: 2 * count, MaxCoefficientBits: exactSurfaceMaxBits})
	if err != nil {
		return nil, err
	}
	*budget -= count
	return q, nil
}
